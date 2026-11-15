# Especificación de frontend — MediList

Documento de trabajo para el modelo de IA o equipo que construirá el frontend.

**Fuente de verdad:** [`docs/openapi.yaml`](./openapi.yaml). Si este documento y OpenAPI difieren, manda OpenAPI y el comportamiento real de la API.

**Alcance:** frontend web para pacientes, médicos, recepción/enfermería, administración y auditoría. No se debe consultar PostgreSQL directamente ni inventar endpoints.

---

## 1. Arquitectura recomendada

- SPA o framework con rutas protegidas: React/Vite, Next.js, Vue u otro.
- Cliente HTTP centralizado con:
  - `Authorization: Bearer <access_token>`.
  - renovación automática con `POST /api/v1/auth/refresh`;
  - manejo uniforme de errores y `request_id`;
  - bloqueo de múltiples refresh simultáneos.
- Estado de sesión global: usuario, roles, permisos, `paciente_id` y `medico_id`.
- Guardas de ruta por **permiso**, no solo por rol.
- Formularios con validación en cliente, pero la API siempre es la autoridad final.
- Nunca enviar tokens, contraseñas, notas clínicas ni datos de pacientes a logs, analytics o consola.

### Entorno local

```env
API_BASE_URL=http://localhost:8080/api/v1
CORS_ALLOWED_ORIGINS=http://localhost:5173,http://localhost:3000
```

La API local debe estar ejecutándose con PostgreSQL migrado y las feature flags activas:

```env
PORTAL_PACIENTE_ENABLED=true
DERIVACIONES_ENABLED=true
TRANSFERENCIAS_PACIENTE_ENABLED=true
```

---

## 2. Contrato general de la API

### Autenticación

- Login: `POST /api/v1/auth/login`
- Refresh: `POST /api/v1/auth/refresh`
- Logout: `POST /api/v1/auth/logout`
- Sesión actual: `GET /api/v1/auth/me`
- Cambio de contraseña: `POST /api/v1/auth/password`
- Registro de paciente: `POST /api/v1/auth/register`
- Verificación: `POST /api/v1/auth/verify-email`
- Reenvío: `POST /api/v1/auth/resend-verification`
- Recuperación: `POST /api/v1/auth/forgot-password`
- Reset: `POST /api/v1/auth/reset-password`

El access token dura aproximadamente 15 minutos. El refresh token rota y debe guardarse de forma segura. Si el refresh falla, cerrar sesión y volver al login.

### Respuestas exitosas

```json
{
  "data": {}
}
```

Listas:

```json
{
  "data": [],
  "meta": {
    "total": 0,
    "limit": 20,
    "offset": 0
  }
}
```

### Errores

```json
{
  "error": {
    "code": "validation_error",
    "message": "descripción legible"
  },
  "request_id": "identificador-para-soporte"
}
```

Códigos esperados:

| HTTP | Significado | Acción UI |
|---:|---|---|
| 400 | Validación, token de cuenta inválido o payload incorrecto | Mostrar errores de campo |
| 401 | Token ausente, inválido, expirado o sesión revocada | Intentar refresh una vez; si falla, login |
| 403 | Sin permiso, cuenta sin vínculo o email no verificado | Mostrar acceso denegado o estado de cuenta |
| 404 | Recurso inexistente | Mostrar vacío/no encontrado |
| 409 | Conflicto de slot, versión, derivación o estado | Recargar datos y explicar el conflicto |
| 423 | Cuenta bloqueada temporalmente | Mostrar tiempo de bloqueo |
| 429 | Rate limit | Respetar `Retry-After` |
| 503 | Dependencia caída o `feature_disabled` | Mostrar módulo no disponible; no reintentar en bucle |
| 500 | Error interno | Mostrar mensaje genérico y `request_id` |

### Reglas globales

- Fechas de API en RFC3339/UTC; mostrar en `America/Guayaquil`.
- Fechas de disponibilidad (`desde`, `hasta`) en formato `YYYY-MM-DD`.
- Paginación con `limit` y `offset`; búsqueda con `search` cuando exista.
- Los endpoints de escritura que devuelven `version` requieren control optimista. Si llega `409`, recargar antes de reintentar.
- En portal de paciente y portal médico, el paciente/médico se obtiene del token. **Nunca** enviar `paciente_id` o `medico_id` desde el body para decidir ownership.
- No mostrar IDs internos como identificadores principales; usar códigos de cita, nombres y fechas.
- Mostrar `request_id` en el detalle técnico de errores 500.

---

## 3. Roles y permisos

La sesión devuelve roles y permisos. La UI debe construir el menú y las acciones con esos permisos.

| Funcionalidad | admin | recepción | enfermería | médico | auditor | paciente |
|---|---:|---:|---:|---:|---:|---:|
| Login / sesión propia | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Portal del paciente | —* | —* | —* | —* | —* | ✓ |
| Pacientes globales: leer | ✓ | ✓ | ✓ | — | — | — |
| Pacientes globales: escribir | ✓ | ✓ | — | — | — | — |
| Paciente con relación de atención | ✓ | — | — | ✓ | — | — |
| Médicos: leer | ✓ | ✓ | ✓ | ✓ | — | catálogo portal |
| Médicos: escribir | ✓ | — | — | — | — | — |
| Especialidades: leer | ✓ | ✓ | ✓ | ✓ | — | catálogo portal |
| Especialidades: escribir | ✓ | — | — | — | — | — |
| Agenda/horarios/bloqueos | ✓ | ✓ | — | propia | — | — |
| Citas globales: leer | ✓ | ✓ | ✓ | — | — | — |
| Citas globales: escribir | ✓ | ✓ | — | — | — | — |
| Citas propias del médico | ✓ | — | — | ✓ | — | — |
| Nota clínica | ✓ | — | — | ✓ | — | — |
| Derivaciones | ✓ | — | — | ✓ | — | propias |
| Excepción de derivación | ✓ | — | — | — | — | — |
| Transferencia interna | ✓ | ✓ | — | — | — | — |
| Vinculación de cuentas | ✓ | ✓ | — | — | — | — |
| Usuarios/roles | ✓ | — | — | — | solo lectura | — |
| Auditoría/errores | ✓ | — | — | — | ✓ | — |

\* Un admin puede tener permisos de portal, pero solo debe usar el portal si su cuenta tiene un `paciente_id` vinculado.

### Navegación sugerida

- **Paciente:** inicio, perfil, preferencias, especialidades, médicos, disponibilidad, mis citas, mis derivaciones.
- **Médico:** agenda, detalle de cita, nota, derivaciones, paciente atendido.
- **Recepción:** pacientes, médicos, agenda, citas, vinculaciones, transferencias.
- **Enfermería:** pacientes, médicos, especialidades y citas en modo lectura.
- **Admin:** todo lo de recepción más usuarios, roles, especialidades, auditoría, errores y excepciones.
- **Auditor:** auditoría, errores, usuarios y roles en modo lectura.

---

## 4. Autenticación y cuenta

### Login
- Campo `username` y `password` para personal de salud.
- Mostrar errores genéricos; no revelar si el usuario existe.
- Si `403 email_no_verificado`, ofrecer reenviar verificación.
- Si `423`, mostrar cuenta bloqueada.
- Si `429`, mostrar `Retry-After`.
- Al autenticar, llamar `GET /auth/me` y redirigir según rol y vínculos.

### Login y registro con Clerk (pacientes)
- Rutas `/sign-in`, `/sign-up` y callback `/clerk/callback`.
- Clerk gestiona registro, verificación de correo y recuperación de contraseña.
- Tras el flujo de Clerk, obtener el token con `getToken()` y llamar `POST /auth/clerk`.
- Guardar los tokens locales devueltos y llamar `GET /auth/me` para resolver el portal.
- Si el paciente aún no tiene `paciente_id`, mostrar cuenta pendiente de vinculación.
- Si `GET /auth/me` devuelve `requiere_completar_registro: true`, redirigir a `/completar-registro` para crear la historia clínica o una solicitud.
- El login local sigue disponible para staff; no vincular automáticamente cuentas internas por correo.

### Registro de paciente
- Solo para pacientes.
- Campos: `username`, `email`, `password`, `nombres`, `apellidos`, `tipo_documento`, `numero_documento`, `fecha_nacimiento`, `sexo`, `telefono`, `direccion`, `acepta_terminos`, `version_terminos`.
- `201`: historia nueva; ir a “revisa tu correo”.
- `202`: ya existía una historia sin cuenta; se creó solicitud de vinculación; explicar que recepción debe aprobarla.
- No iniciar sesión automáticamente después del registro.

### Verificación de email
- Ruta frontend que lea `?token=`.
- Llamar `POST /auth/verify-email` con `{ "token": "..." }`.
- Mostrar éxito/error y enlace a login.
- El token es de un solo uso y vence.

### Recuperación de contraseña
- `POST /auth/forgot-password` con email; respuesta genérica.
- `POST /auth/reset-password` con token y nueva contraseña.
- El reset revoca sesiones existentes; obligar a iniciar sesión de nuevo.

### Cambio de contraseña
- `POST /auth/password` con `password_actual` y `password_nueva`.
- Tras `204`, cerrar sesión y pedir login.

### Logout
- `POST /auth/logout` con refresh token.
- Limpiar tokens locales y redirigir a login.

### Cuenta pendiente de vinculación
- `GET /auth/me` puede devolver `paciente_id: null`.
- Si `requiere_completar_registro` es `true`, redirigir a `/completar-registro`.
- Mostrar estado “cuenta pendiente de vinculación” y no permitir reservas hasta que recepción apruebe.

---

## 5. Portal del paciente

Rutas frontend sugeridas: `/paciente`, `/paciente/perfil`, `/paciente/preferencias`, `/paciente/citas`, `/paciente/derivaciones`.

### Inicio
- Próxima cita.
- Acceso rápido a “Agendar cita”.
- Derivaciones pendientes.
- Estado de la cuenta.

### Perfil
- `GET /portal/perfil`.
- `PUT /portal/perfil` con `telefono`, `direccion`, `contacto_emergencia_nombre`, `contacto_emergencia_telefono`, `version` opcional.
- No permitir cambiar documento, nombres, email ni fecha de nacimiento desde aquí.
- Si `409`, recargar perfil.

### Preferencias de notificación
- `GET /portal/preferencias-notificacion`.
- `PUT /portal/preferencias-notificacion`.
- Campos: `email_habilitado`, `recordatorios_habilitados`.
- Mostrar que desactivar recordatorios no cancela citas.

### Agendar cita directa
1. `GET /portal/especialidades` y mostrar solo `requiere_derivacion=false`.
2. `GET /portal/medicos?especialidad_id=...`.
3. `GET /portal/disponibilidad?medico_id=...&desde=YYYY-MM-DD&hasta=YYYY-MM-DD`.
4. Elegir un slot exacto.
5. `POST /portal/citas`:

```json
{
  "medico_id": 1,
  "fecha_hora_inicio": "2026-11-03T14:00:00Z",
  "tipo": "primera_vez",
  "motivo": "Dolor de cabeza"
}
```

- `201`: cita creada.
- `409`: slot ocupado; refrescar disponibilidad.
- No enviar `paciente_id`.
- Especialidades directas: Medicina General, Fisioterapia, Odontología, Nutrición y Pediatría.

### Agendar con derivación
- Mostrar solo derivaciones propias en estado `pendiente`.
- Elegir especialidad/médico destino y slot.
- `POST /portal/citas` incluyendo `derivacion_id`.
- Una derivación habilita una sola cita activa.
- Si la derivación está vencida, usada o no corresponde, mostrar el error y recargar.

### Mis citas
- `GET /portal/citas`; filtros de estado y fechas si están disponibles.
- `GET /portal/citas/{id}`.
- Mostrar estado, código, profesional, especialidad, fecha, modalidad, motivo y si tiene derivación.

### Reprogramar
- `PUT /portal/citas/{id}`:

```json
{
  "fecha_hora_inicio": "2026-11-04T15:00:00Z",
  "motivo": "Cambio de disponibilidad",
  "version": 1
}
```

- Solo cambia el slot; no paciente, médico ni especialidad.
- `409`: recargar cita y disponibilidad.

### Cancelar
- `POST /portal/citas/{id}/cancelar` con `{ "motivo": "..." }`.
- Antes del corte configurado: libera la derivación si sigue vigente.
- Después del corte: puede consumir la derivación; explicarlo antes de confirmar.
- Mostrar política de cancelación.

### Derivaciones
- `GET /portal/derivaciones`.
- `GET /portal/derivaciones/{id}`.
- Estados: `pendiente`, `agendada`, `atendida`, `vencida`, `cancelada`.
- Si está vencida o consumida, no permitir reservar.

---

## 6. Portal del médico

Rutas: `/medico/agenda`, `/medico/citas/:id`, `/medico/derivaciones`, `/medico/pacientes/:id`.

### Agenda
- `GET /medico/agenda`; el médico se resuelve desde el token.
- Mostrar solo citas propias.
- Filtros disponibles: `search`, `limit`, `offset`.
- Si la cuenta no tiene `medico_id`, mostrar acceso denegado/contacto con administración.

### Detalle de cita
- `GET /medico/citas/{id}`.
- Si la cita es ajena: `403`; no mostrar datos.

### Cambiar estado
- `PATCH /medico/citas/{id}/estado` con `{ "estado": "confirmada" | "atendida" | "no_asistio" }`.
- Solo permitir acciones válidas según el estado actual.
- Si `409`, recargar la cita.

### Nota clínica
- `GET /medico/citas/{id}/nota`.
- `POST` para crear y `PUT` para actualizar.
- Campos: `motivo`, `evaluacion`, `plan`, `version`.
- Solo sobre cita `atendida` y del médico autenticado.
- No exponer la nota completa al paciente.
- Manejar `409` por versión.

### Derivación
- `POST /medico/citas/{id}/derivaciones`.
- Solo desde una cita de Medicina General atendida.
- Body:

```json
{
  "especialidad_destino_id": 2,
  "medico_destino_id": 5,
  "motivo": "Evaluación especializada",
  "prioridad": "normal"
}
```

- Destino debe requerir derivación.
- `201` al crear.
- `GET /medico/derivaciones` para listar las emitidas.

### Paciente atendido
- `GET /medico/pacientes/{id}` solo si existe relación de atención.
- No hay búsqueda global de pacientes para el médico.

---

## 7. Recepción y enfermería

### Recepción

Puede gestionar pacientes, citas, agenda, vinculaciones y transferencias; no notas, derivaciones ni auditoría.

- Pacientes:
  - `GET /pacientes`
  - `GET /pacientes/{id}`
  - `POST /pacientes`
  - `PUT /pacientes/{id}`
  - `DELETE /pacientes/{id}`
- Médicos: solo lectura (`GET /medicos`, `GET /medicos/{id}`).
- Especialidades: solo lectura.
- Agenda:
  - horarios: `GET/POST/PUT/DELETE /medicos/{id}/horarios`
  - bloqueos: `GET/POST/PUT/DELETE /medicos/{id}/bloqueos`
- Citas:
  - `GET/POST /citas`
  - `GET/PUT/DELETE /citas/{id}`
  - `PATCH /citas/{id}/estado`
- Vinculaciones:
  - `GET /cuentas/vinculaciones`
  - `POST /cuentas/vinculaciones/{id}/aprobar`
  - `POST /cuentas/vinculaciones/{id}/rechazar`
  - `POST /cuentas/vincular-paciente`
- Transferencias:
  - `POST /pacientes/{id}/transferir`
  - `GET /pacientes/{id}/transferencias`

### Enfermería

Solo lectura de pacientes, médicos, especialidades y citas. No mostrar botones de escritura. No tiene acceso a notas, derivaciones, auditoría ni usuarios.

---

## 8. Administración

Además de las funciones de recepción:

- Especialidades CRUD.
- Médicos CRUD y vinculación de cuenta médica.
- Usuarios y roles.
- Desbloquear cuentas y revocar sesiones.
- Auditoría y errores.
- Excepción administrativa de derivación.
- Transferencias.

Las feature flags son variables de entorno del backend; no existe un endpoint para cambiarlas desde el frontend.

### Usuarios
- `GET/POST /usuarios`
- `GET/PUT /usuarios/{id}`
- `POST /usuarios/{id}/desbloquear`
- `POST /usuarios/{id}/revocar-sesiones`
- `GET /roles`

La UI debe impedir que un admin se elimine a sí mismo o se quite el último rol administrador; no asumir que el backend lo bloquea.

### Excepción de derivación
- `POST /citas/{id}/sobrescribir-derivacion`.
- Solo admin.
- Requiere motivo y autorización explícita.
- Mostrar advertencia clínica y registrar auditoría.
- No usar como flujo normal.

### Auditoría y errores
- `GET /auditoria`: filtros de usuario, acción, recurso, rango, `trace_id`.
- `GET /errores`: filtros de nivel, servicio, instancia, `trace_id`, búsqueda.
- Solo lectura.
- No mostrar tokens, contraseñas ni notas clínicas.
- Mostrar `request_id`/`trace_id`.

---

## 9. Auditor

Solo lectura:

- `GET /auditoria`
- `GET /errores`
- `GET /usuarios`
- `GET /roles`

No mostrar datos clínicos ni botones de edición. Permitir exportación solo si se implementa en frontend y respeta privacidad.

---

## 10. Feature flags

Si la API responde:

```json
{
  "error": {
    "code": "feature_disabled",
    "message": "la funcionalidad ... no está habilitada"
  }
}
```

la UI debe:

- mostrar un estado “módulo no disponible”;
- ocultar o deshabilitar la navegación correspondiente;
- no reintentar automáticamente;
- no asumir que el endpoint no existe.

Flags:

- `PORTAL_PACIENTE_ENABLED`
- `DERIVACIONES_ENABLED`
- `TRANSFERENCIAS_PACIENTE_ENABLED`

---

## 11. Modelos de datos clave

Consultar los esquemas completos en `docs/openapi.yaml`. Campos mínimos para UI:

- **Usuario:** `id`, `username`, `email`, `nombres`, `apellidos`, `activo`, `email_verificado`, `roles`, `permisos`, `version`.
- **Paciente:** `id`, `persona`, `numero_historia`, datos de contacto, `usuario_id`, `version`.
- **Médico:** `id`, `persona`, `especialidad`, `numero_licencia`, `duracion_cita_min`, `acepta_portal`, `activo`, `version`.
- **Especialidad:** `id`, `codigo`, `nombre`, `requiere_derivacion`, `activo`.
- **Horario:** `id`, `medico_id`, `dia_semana` (1=lunes, 7=domingo), `hora_inicio`, `hora_fin`, `duracion_slot_min`, `vigente_desde`, `vigente_hasta`, `activo`, `version`.
- **Bloqueo:** `id`, `medico_id`, `inicio`, `fin`, `motivo`, `version`.
- **Cita:** `id`, `codigo`, `paciente_id`, `medico_id`, `especialidad`, `fecha_hora_inicio`, `duracion_min`, `tipo`, `motivo`, `estado`, `canal`, `derivacion_id`, `version`.
- **Derivación:** `id`, `paciente_id`, `medico_origen_id`, `medico_destino_id`, `especialidad_destino`, `estado`, `vigente_hasta`, `motivo`, `prioridad`, `version`.
- **Nota:** `id`, `cita_id`, `medico_id`, `motivo`, `evaluacion`, `plan`, `version`.
- **Preferencias:** `email_habilitado`, `recordatorios_habilitados`.
- **Auditoría/Error:** actor, acción/recurso, método/ruta, estado, `trace_id`, fechas.

---

## 12. Reglas de negocio que la UI debe reflejar

- Especialidades directas: `MED-GEN`, `FISIO`, `ODONTO`, `NUTRI`, `PED`.
- Las demás especialidades requieren derivación vigente.
- Una derivación permite una sola cita activa.
- Cancelar con al menos 24 h libera la derivación; cancelar tarde o no-show la consume.
- Reprogramar conserva la derivación si sigue vigente y no cambia especialidad/médico.
- Antelación mínima de reserva: 2 h.
- Horizonte máximo: 90 días.
- Slots alineados a la agenda y no bloqueados.
- Un paciente no puede reservar dos citas solapadas.
- Un médico no puede tener dos citas solapadas.
- Una cita atendida no se reprograma ni cancela.
- Solo una nota por cita.
- El paciente no puede ver notas clínicas completas.
- El médico solo ve pacientes con relación de atención.
- La recepción no ve notas ni derivaciones clínicas.
- Toda escritura clínica debe enviar `version` cuando aplique.

---

## 13. Pruebas de aceptación del frontend

- Login correcto/incorrecto, bloqueo y rate limit.
- Refresh automático y cierre de sesión si falla.
- Registro nuevo (`201`) y vinculación pendiente (`202`).
- Verificación de email con token válido, vencido y usado.
- Recuperación/reset de contraseña.
- Paciente solo ve sus citas, perfil y derivaciones.
- Paciente no puede reservar especialidad con derivación sin derivación.
- Dos pacientes compitiendo por un slot: uno recibe `201`, otro `409`.
- Médico solo ve su agenda y pacientes con relación.
- Médico no puede acceder a endpoints administrativos.
- Recepción no ve notas ni derivaciones.
- Enfermería no puede escribir.
- Auditor no puede editar.
- Admin puede gestionar usuarios/roles y ver auditoría.
- `503 feature_disabled` se muestra correctamente.
- `429` respeta `Retry-After`.
- `409` recarga datos antes de reintentar.
- Fechas se muestran en `America/Guayaquil`.
- Formularios responsive y accesibles.
- Nunca se muestran tokens, contraseñas, notas clínicas ni datos de otro paciente.

---

## 14. Fuera de alcance

No implementar en esta iteración:

- OAuth social adicional, MFA o login biométrico (Clerk cubre email/password).
- Recuperación de nombre de usuario: el backend solo ofrece recuperación de contraseña por email.
- Pagos.
- Carga de archivos o imágenes clínicas.
- Chat en tiempo real.
- Reportes analíticos no expuestos por la API.
- Endpoints inventados o acceso directo a PostgreSQL.
- Funciones de producción como SMTP real, TLS o despliegue cloud.

---

## 15. Entregable esperado del modelo de IA

1. Estructura de rutas y layouts por rol.
2. Cliente API con refresh y manejo de errores.
3. Guardas por permisos.
4. Pantallas de autenticación y recuperación.
5. Portal paciente completo.
6. Portal médico completo.
7. Módulos de recepción, enfermería, administración y auditoría.
8. Componentes de estados vacíos, carga, error, conflicto y feature flag.
9. Pruebas de los flujos críticos.
10. Documentación de variables de entorno y ejecución local.
