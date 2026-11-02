# MediList — Plan para concluir el backend

> Fecha: 26/10/2026  
> Alcance: backend Go/PostgreSQL. No incluye frontend.  
> Estado: Fase 0 cerrada; fases 1–5 completadas; la Fase 6 (flujo médico, derivaciones y transferencia interna) es la siguiente.
> Documentos relacionados: `README.md` (estado actual) y `entidades.md` (modelo completo objetivo).

---

## 0. Resumen ejecutivo

La base actual ya tiene una parte importante del backend administrativo:

- autenticación con access/refresh tokens;
- RBAC con roles y permisos;
- CRUD de pacientes, médicos, especialidades y citas;
- control de concurrencia con restricciones de exclusión y versiones optimistas;
- auditoría, logs estructurados y retención.

La **Fase 2 ya está implementada**: registro público de pacientes, verificación de email, recuperación de contraseña, revocación de sesiones y vinculación con historias clínicas existentes sin duplicarlas. La **Fase 3 también está implementada**: el middleware resuelve un `Actor` con los vínculos de paciente/médico, `/auth/me` expone esos IDs, el paciente puede consultar y actualizar su contacto, el médico consulta su agenda y sus pacientes con relación de atención, y los endpoints administrativos de pacientes/citas están bloqueados para médicos incluso con tokens antiguos. Las **decisiones de la Fase 0 están cerradas** y la migración 014 deja Pediatría como acceso directo. La **Fase 4 está implementada**: catálogo administrativo completo, CRUD de horarios/bloqueos, perfil público del médico, cálculo de disponibilidad por zona horaria, antelación y horizonte, y endpoints `/portal/especialidades`, `/portal/medicos` y `/portal/disponibilidad`. La **Fase 5 también está implementada**: el paciente reserva, lista, consulta, reprograma y cancela sus propias citas en las cinco especialidades de acceso directo, con canal `portal_paciente`, versionado y control de carreras. Todavía **no existen las acciones clínicas del médico ni las derivaciones**: eso corresponde a las fases 6–8. También existen las migraciones y pruebas de esquema de disponibilidad y derivaciones.

El orden recomendado para concluir el backend es:

1. Cerrar las decisiones de negocio y el contrato de API.
2. Crear el modelo de datos y las migraciones.
3. ~~Implementar registro, verificación de cuenta y vinculación paciente/usuario.~~ (completado)
4. Implementar autorización por fila: el paciente solo ve lo suyo y el médico solo ve su agenda.
5. ~~Crear catálogo, horarios y disponibilidad.~~ (completado)
6. ~~Permitir que el paciente agende las cinco áreas de acceso directo.~~ (completado)
7. Implementar el flujo médico mínimo y las derivaciones.
8. Permitir que el paciente use una derivación para agendar al especialista.
9. Añadir notificaciones, pruebas, endurecimiento y despliegue controlado.

La regla central solicitada queda así:

> El paciente puede agendar directamente **Medicina General, Fisioterapia, Odontología, Nutrición y Pediatría**. Para cualquier otra especialidad necesita una **derivación vigente emitida por un médico de Medicina General después de una cita atendida**. Sin derivación, el backend rechaza la reserva del especialista.

Esta regla debe vivir en el servicio y en la base de datos, no solo en el frontend.

### Aclaración importante sobre “remitirlo a otro paciente”

En el texto se indica que “el doctor puede remitirlo a otro paciente”. Eso no es un flujo clínico válido: un médico no remite un paciente a otro paciente. En este documento se interpreta como:

> **El médico remite al paciente a otro profesional o a otra especialidad.**

Si lo que se necesita es representar un vínculo familiar o de tutor (por ejemplo, un paciente menor de edad o un cuidador), eso es otra entidad y se trata en la fase de tutores/representantes, no en la de derivaciones.

---

## 1. Supuestos y decisiones de producto

Este cuadro conserva las decisiones de negocio. **Fase 0 cerrada:** no quedan decisiones de producto pendientes; las marcadas como implementadas ya están reflejadas en el esquema o en las fases 1–3.

| # | Tema | Decisión | Estado |
|---|---|---|---|
| 1 | Significado de “remitir a otro paciente” | Derivación clínica a otra especialidad/profesional y cambio interno de médico tratante; no incluye transferencia externa | Confirmado |
| 2 | Especialidades de acceso directo | `MED-GEN`, `FISIO`, `ODONTO`, `NUTRI`, `PED` | Implementado en catálogo; migración 014 |
| 3 | Especialidades que exigen derivación | Todas las demás, incluida `OTORRINO` | Implementado en catálogo |
| 4 | Quién emite la derivación que habilita al especialista | Solo médico de Medicina General, desde una cita `atendida` | Implementado en trigger 011 |
| 5 | Vigencia de la derivación | 90 días por defecto, configurable | Confirmado |
| 6 | Uso de la derivación | Una derivación habilita una cita activa. Cancelar con al menos 24 h y estando vigente la devuelve a `pendiente`; cancelar después del límite o no asistir la consume. Reprogramar la conserva si sigue vigente y es la misma especialidad/médico | Confirmado; índice 011 |
| 7 | No asistencia del paciente a la cita de especialista | Consume la derivación; se requiere nueva valoración de Medicina General | Confirmado |
| 8 | Excepción de emergencia | Solo el rol `admin`, con `derivaciones:sobrescribir`, motivo obligatorio y auditoría | Confirmado |
| 9 | Registro de paciente | Requiere email verificado antes del login y del refresh; no solo antes de agendar | Implementado en Fase 2 |
| 10 | Paciente ya existente | No duplicar historia; vincular cuenta mediante recepción o verificación | Implementado en Fase 2 |
| 11 | Cuenta de médico | La crea un administrador; el médico no se autorregistra | Implementado en Fase 2 |
| 12 | Pediatría | Acceso directo para todos; `PED.requiere_derivacion = false`; sin regla por edad | Confirmado; migración 014 |
| 13 | Menores de edad | Autorregistro solo para adultos (18 años por defecto); tutor/representante en una fase posterior | Confirmado |
| 14 | Zona horaria y agenda | `America/Guayaquil` para agenda y reportes; persistir en UTC; slot de 30 min configurable por especialidad; antelación mínima de 2 h; horizonte máximo de 90 días | Confirmado |
| 15 | Alcance clínico | Este plan cubre agenda, cuenta, derivaciones, transferencia interna y nota mínima; historia clínica completa queda para P2 | Confirmado por alcance |
| 16 | Transferencia interna | Cambio de médico tratante dentro de la clínica; admin/recepción con `pacientes:transferir`, motivo y auditoría; no hay transferencia externa | Confirmado; se implementa en Fase 6 |

### 1.1 Decisión cerrada sobre “remitir”

“Remitir” cubre dos flujos distintos:

1. **Derivación clínica:** el médico de Medicina General deriva al paciente a otra especialidad o profesional. La historia y la cuenta no cambian de dueño.
2. **Transferencia interna:** administración o recepción cambia el médico tratante del paciente dentro de la misma clínica. La historia, la cuenta y las citas se conservan.

No se implementa transferencia a otra clínica ni intercambio de historias entre instituciones en este alcance.

### 1.2 Decisión cerrada sobre Pediatría

Pediatría será de **acceso directo para todos**, sin regla por edad. Se implementa con `PED.requiere_derivacion = false` en la migración 014. Esto amplía la regla original de cuatro especialidades a cinco; la fuente de verdad sigue siendo `especialidades.requiere_derivacion` y no se agregan listas hardcodeadas en el código.

### 1.3 Transferencia interna de médico tratante

- El paciente tendrá un `medico_responsable_id` opcional y un historial append-only de transferencias.
- Solo `admin` y `recepción` con `pacientes:transferir` pueden ejecutarla; se exige motivo y auditoría.
- El médico nuevo obtiene acceso por la asignación activa. El médico anterior conserva acceso solo a las citas que ya tenga programadas.
- La transferencia no borra ni reescribe la historia y no cancela automáticamente citas futuras.
- El detalle de esquema y endpoints se define en la Fase 6; la migración prevista es la 015.

---

## 2. Estado actual real

### 2.1 Lo que ya existe

- **Autenticación:** login, refresh token rotativo, logout, bloqueo por intentos fallidos, revocación inmediata por `token_version`.
- **RBAC:** roles `admin`, `medico`, `recepcion`, `enfermeria`, `auditor`; permisos granulares.
- **Usuarios:** CRUD administrativo, asignación de roles, desbloqueo y revocación de sesiones.
- **Pacientes:** CRUD con `persona`, historia clínica administrativa, datos de contacto y versión optimista.
- **Médicos:** CRUD con `persona`, especialidad, licencia y estado.
- **Especialidades:** catálogo con 12 registros, flags de acceso directo/derivación y duración por especialidad.
- **Citas:** creación, consulta, actualización, cambio de estado, filtros y versión optimista.
- **Concurrencia:** restricciones de exclusión para solapes de médico y paciente en citas `programada`/`confirmada`, horarios recurrentes y bloqueos de agenda; detección de conflictos `23P01`, `40001`, `40P01`, `55P03`.
- **Disponibilidad:** tablas `horarios_profesional` y `bloqueos_agenda`, CRUD administrativo, generación de slots en `CLINIC_TIMEZONE` y endpoints de portal.
- **Auditoría:** bitácora de acciones y consultas.
- **Observabilidad:** logs JSON, `trace_id`, `X-Request-ID`, errores en base, retención y stack de Loki/Grafana local.
- **Cuentas de paciente (Fase 2):** registro transaccional, validación de username/email/documento/edad/términos, rol `paciente`, token de verificación hasheado de un solo uso, reenvío, recuperación y reset de contraseña.
- **Vinculación de cuentas:** si el documento ya existe como paciente sin cuenta, se crea una cuenta pendiente y una `solicitud_vinculacion`; recepción/admin la aprueba o rechaza sin duplicar la historia. La cuenta de médico se vincula por administración.
- **Correo de desarrollo:** adaptador `DevEmailSender` que escribe `.eml` en un buzón local detrás del puerto `EmailSender`; no registra tokens.
- **Derivaciones y notas (esquema):** migración 011 aplicada y validada con triggers y pruebas de integración; falta la capa de servicio y endpoints.

### 2.2 Lo que falta para el flujo solicitado

- **Portal del paciente:** perfil, catálogo, disponibilidad, reserva, listado, reprogramación y cancelación de citas propias implementados; faltan las derivaciones (fase 7).
- **Portal del médico:** existe la agenda propia; faltan "mi cita" con acciones, "marcar atendida" y "emitir derivación" (fase 6).
- **Disponibilidad y reserva:** tablas, CRUD administrativo, generación de slots, zona horaria, antelación/horizonte y reserva transaccional con control de carreras implementados.
- **Especialidades solicitadas:** `FISIO`, `ODONTO`, `NUTRI` y `OTORRINO` están en el catálogo y se exponen en el portal; las cinco directas se pueden reservar desde la Fase 5.
- **Derivaciones:** la tabla, triggers y pruebas de esquema existen; faltan servicio, endpoints, consumo y liberación desde la API.
- **Autorización por fila:** un médico con `citas:leer` puede consultar todas las citas; el rol paciente existe, pero el patrón actual no está preparado para “solo lo mío”.
- **Notificaciones transaccionales:** verificación y reset ya tienen correo de desarrollo; faltan recordatorios, confirmaciones y outbox durable.
- **Nota/atención clínica mínima:** una cita solo cambia de estado; no hay endpoint para registrar la consulta que justifique una derivación.
- **Historia clínica completa:** diagnósticos, recetas, órdenes, exámenes y adjuntos no están implementados.
- **Menores de edad:** el autorregistro exige mayoría de edad configurable; falta el flujo de tutor/representante.

### 2.3 Conclusión de estado

La base de agenda, seguridad y cuentas es aprovechable, pero **el producto “paciente con cuenta que agenda y médico que deriva” no está terminado**. La Fase 2 resolvió el alta y la vinculación; ahora hay que construir el portal y el ownership sobre lo existente, sin romper la arquitectura por capas ni las garantías de concurrencia.

---

## 3. Reglas de negocio propuestas

### 3.1 Catálogo y acceso directo

| Especialidad | Código propuesto | Acceso del paciente | Observación |
|---|---|---|---|
| Medicina General | `MED-GEN` | Directo | Puerta de entrada para especialistas |
| Fisioterapia | `FISIO` | Directo | Profesional de fisioterapia |
| Odontología | `ODONTO` | Directo | Odontología general |
| Nutrición | `NUTRI` | Directo | Nutricionista |
| Otorrinolaringología | `OTORRINO` | Con derivación | Caso explícito del requerimiento |
| Cardiología | `CARD` | Con derivación | Ya existe |
| Pediatría | `PED` | Directo | Decisión de Fase 0: acceso directo para todos (migración 014) |
| Ginecología | `GINE` | Con derivación | Ya existe |
| Traumatología | `TRAUM` | Con derivación | Ya existe |
| Dermatología | `DERM` | Con derivación | Ya existe |
| Neurología | `NEURO` | Con derivación | Ya existe |
| Oftalmología | `OFT` | Con derivación | Ya existe |
| Otras | — | Con derivación por defecto | El catálogo se puede ampliar |

La fuente de verdad es `especialidades.requiere_derivacion`. No se debe codificar una lista de códigos en el handler ni en el frontend.

Campos recomendados para el catálogo:

- `requiere_derivacion BOOLEAN NOT NULL DEFAULT TRUE`;
- `activa BOOLEAN NOT NULL DEFAULT TRUE`;
- `orden INTEGER NOT NULL DEFAULT 0`;
- `duracion_cita_min INTEGER NOT NULL DEFAULT 30`.

### 3.2 Regla de derivación

Para que un paciente pueda reservar una especialidad con `requiere_derivacion = true`, deben cumplirse **todas** estas condiciones:

1. Existe una derivación con estado `pendiente` o `agendada`.
2. La derivación pertenece al mismo paciente autenticado.
3. La derivación no está vencida (`vigente_hasta > now()`).
4. La especialidad de destino coincide con la especialidad del médico seleccionado.
5. Si la derivación tiene `medico_destino_id`, el médico seleccionado es ese médico.
6. La cita de origen es una cita de Medicina General en estado `atendida`.
7. El médico de origen es el médico que atendió esa cita.
8. La derivación no fue consumida por otra cita activa o atendida.
9. El paciente no tiene otra cita activa para la misma derivación.

Si falla cualquier condición, la API debe responder con un error de negocio claro, no con un `500`.

Errores propuestos:

| Código | HTTP | Cuándo |
|---|---|---|
| `requiere_derivacion` | 409 | La especialidad exige derivación y no hay ninguna |
| `derivacion_vencida` | 409 | Existe, pero `vigente_hasta` ya pasó |
| `derivacion_no_disponible` | 409 | Está cancelada, consumida o no pertenece al paciente |
| `derivacion_especialidad_incorrecta` | 409 | La especialidad del médico no coincide |
| `derivacion_medico_incorrecto` | 409 | La derivación apunta a otro médico |
| `cita_origen_no_atendida` | 409 | Se intenta derivar desde una cita no atendida |
| `slot_no_disponible` | 409 | El horario ya fue tomado o no está disponible |
| `fuera_de_horario` | 409 | El horario no pertenece a la agenda del médico |
| `cuenta_no_verificada` | 403 | El paciente no verificó su email |
| `conflicto_version` | 409 | Actualización concurrente perdida |

### 3.3 Ciclo de vida de la derivación

Estados:

- `pendiente`: creada por el médico, aún no usada.
- `agendada`: vinculada a una cita activa de especialista.
- `atendida`: el especialista atendió la cita.
- `vencida`: pasó `vigente_hasta` sin completarse.
- `cancelada`: anulada por el médico/administración o consumida por cancelación tardía o no-show. En los dos últimos casos se registra `motivo_cancelacion` (`cancelacion_tardia` o `no_show`) y el estado es terminal.

Transiciones:

| Estado actual | Evento | Estado nuevo |
|---|---|---|
| `pendiente` | Se agenda una cita de especialista | `agendada` |
| `pendiente` | Vence `vigente_hasta` | `vencida` |
| `pendiente` | El médico de origen o un admin la cancela | `cancelada` |
| `agendada` | El especialista marca la cita como `atendida` | `atendida` |
| `agendada` | El paciente cancela con al menos 24 h de antelación y sigue vigente | `pendiente` |
| `agendada` | El paciente cancela con menos de 24 h | `cancelada` (consumida) |
| `agendada` | El paciente no asiste | `cancelada` (consumida) |
| `agendada` | Vence `vigente_hasta` | `vencida` |
| `agendada` | El médico de origen o un admin la cancela | `cancelada` |
| `agendada` | Se reprograma a un slot válido de la misma especialidad/médico | `agendada` con la nueva cita |
| `atendida`, `vencida`, `cancelada` | — | Estado terminal, no reutilizable |

Reglas de transición:

- Solo se crea desde una cita de Medicina General `atendida`.
- La vigencia se calcula con `REFERRAL_VALID_DAYS=90` por defecto, configurable.
- `pendiente -> agendada` ocurre en la misma transacción que crea la cita del especialista.
- `agendada -> atendida` cuando el especialista marca la cita como `atendida`.
- `agendada -> pendiente` si el paciente cancela con al menos `CANCELACION_LIMITE_HORAS=24` y la derivación sigue vigente.
- `agendada -> cancelada (consumida)` si cancela después del límite o no asiste; se exige `motivo_cancelacion`.
- Reprogramar conserva la derivación si sigue vigente y no cambia la especialidad ni el médico; la cita anterior pasa a `reprogramada` y la nueva queda activa.
- `pendiente/agendada -> vencida` por paso del tiempo; se puede calcular en consulta o con un job.
- `pendiente/agendada -> cancelada` solo por médico de origen o admin, con motivo, salvo la cancelación tardía/no-show del paciente.
- Una derivación `atendida`, `vencida` o `cancelada` no puede reutilizarse.
- El no-show del paciente a la cita de especialista consume la derivación; se requiere una nueva valoración de Medicina General.

Para defensa en profundidad, la base debe impedir que dos citas activas usen la misma derivación:

```sql
CREATE UNIQUE INDEX citas_derivacion_activa_uk
ON citas (derivacion_id)
WHERE derivacion_id IS NOT NULL
  AND estado IN ('programada', 'confirmada', 'atendida');
```

### 3.4 Agenda y disponibilidad

Hoy una cita puede crearse a cualquier hora si no hay solape. Para el portal se necesita una agenda real:

- `horarios_profesional`: día de la semana, hora de inicio, hora de fin, duración del slot, vigencia.
- `bloqueos_agenda`: vacaciones, reuniones, ausencias; rangos de fecha/hora.
- `medicos.duracion_cita_min`: duración preferida del profesional (con valor por defecto de la especialidad).
- Zona horaria de la clínica configurable; en base se guarda `timestamptz`.
- Antelación mínima para reservar (por ejemplo, 2 horas).
- Máximo de días hacia adelante (por ejemplo, 90 días).
- Política de cancelación (por ejemplo, 24 horas antes).
- Los slots se generan en el servicio; no se guardan todos en una tabla.
- Un slot solo es válido si cae dentro de un horario, no está bloqueado, no está en el pasado y no tiene cita activa.

La reserva debe ser transaccional:

1. Validar paciente y médico activos.
2. Validar especialidad y derivación (si aplica).
3. Validar horario y bloqueos.
4. Insertar la cita.
5. Confirmar o reservar.
6. Si hay carrera, las restricciones de exclusión existentes y la validación devuelven `409`, nunca `500`.

### 3.5 Reglas de citas del paciente

- El paciente solo puede ver y modificar **sus** citas.
- El paciente no puede elegir otro `paciente_id`; el servidor lo deriva de la sesión.
- Puede agendar directamente las cinco especialidades de acceso directo.
- No puede agendar un especialista sin derivación.
- Puede cancelar su cita antes del límite configurado. Si cancela con al menos 24 h y la derivación sigue vigente, esta vuelve a `pendiente`; si cancela después del límite o no asiste, la derivación se consume (`cancelada` terminal con motivo).
- Puede reprogramar su cita; la operación debe usar `version` para evitar pérdidas concurrentes. La derivación se conserva si sigue vigente y no cambia la especialidad ni el médico.
- Si la cita es `atendida`, `no_asistio` o `cancelada`, no se puede reprogramar.
- El estado inicial recomendado es `programada`; se puede autoconfirmar con `AUTO_CONFIRM_PORTAL_BOOKINGS=true`.
- El paciente no puede cambiar médico, especialidad ni paciente en una reprogramación; para cambiar de profesional debe cancelar y crear otra cita.

### 3.6 Reglas de citas del médico

- El médico solo ve las citas donde `medico_id` es el suyo.
- Puede confirmar, marcar `atendida` o `no_asistio` sus propias citas.
- No puede modificar citas de otro médico.
- Puede registrar una nota mínima de consulta.
- Solo un médico de Medicina General puede emitir la derivación que habilita al paciente a ver un especialista.
- La derivación se emite desde una cita `atendida`, no desde una cita solo `confirmada`.
- El médico ve el motivo, la especialidad de destino y la vigencia de la derivación que emite.
- El médico no puede transferir la historia ni cambiar el médico tratante por sí mismo; la derivación clínica es a otra especialidad o profesional. La reasignación interna la ejecuta admin/recepción con `pacientes:transferir`.

### 3.7 Reglas de recepción y administración

- Recepción puede crear y confirmar citas para cualquier paciente, como hoy.
- Recepción puede gestionar horarios y bloqueos si tiene `agenda:gestionar`.
- Recepción puede vincular una cuenta a un paciente existente.
- Recepción no puede emitir derivaciones clínicas.
- Solo el rol `admin` puede sobrescribir la regla de derivación con `derivaciones:sobrescribir`, indicando motivo; toda sobrescritura queda auditada. Recepción no puede hacerlo.
- La sobrescritura debe ser excepcional (urgencia, error operativo), no el flujo normal.
- Admin y recepción con `pacientes:transferir` pueden cambiar el médico tratante dentro de la clínica, con motivo y auditoría. No pueden transferir la historia a otra institución.

### 3.8 Nota mínima de atención

Para que la derivación tenga sustento y no sea solo un botón, se recomienda una entidad mínima:

- `citas.id`;
- `medico_id`;
- `motivo_consulta`;
- `evaluacion`;
- `plan`;
- `creada_en`;
- `version`.

No es la historia clínica completa. Es una nota de la consulta que permite auditar por qué se derivó. Diagnósticos codificados, recetas, órdenes y adjuntos quedan para P2.

### 3.9 Transferencia interna de médico tratante

- El paciente tendrá un `medico_responsable_id` opcional, con un historial append-only en `transferencias_paciente`.
- Solo `admin` y `recepción` con `pacientes:transferir` pueden cambiar el médico tratante; el médico no puede asignarse pacientes a sí mismo.
- La operación exige `medico_destino_id` activo, motivo y `version` del paciente; se ejecuta en una transacción y se audita.
- El médico nuevo obtiene acceso por la asignación activa. El anterior conserva acceso solo mientras tenga citas `programada`, `confirmada` o `atendida`.
- No se cancelan ni reasignan automáticamente citas futuras; si se requiere, se gestionan por separado.
- No existe transferencia externa ni intercambio de historia entre instituciones en este alcance.

---

## 4. Arquitectura objetivo

### 4.1 Principios que no se deben romper

- Mantener la dirección de dependencias actual: `transport -> service -> repository`.
- Las entidades de dominio viven en `internal/service/entities`.
- Los handlers solo decodifican, autorizan a nivel de transporte y traducen errores.
- La lógica de negocio (derivación, disponibilidad, cupos, estados) vive en `service`.
- PostgreSQL no se consulta desde los handlers.
- Los DTO de entrada/salida no se mezclan con entidades de dominio.
- Toda operación sensible se audita.
- Toda operación concurrente usa transacción, bloqueo o versión; no se confía en un “check-then-insert” sin protección.

### 4.2 Actor autenticado y autorización por fila

El punto más delicado es que hoy los permisos son globales. Un paciente no puede recibir `citas:leer` porque vería todas las citas. Un médico con `citas:leer` también ve todas. Hay que introducir un contexto de actor:

```go
type Actor struct {
    UsuarioID  int64
    Roles      []string
    Permisos   []string
    PacienteID *int64
    MedicoID   *int64
    EmailVerificado bool
}
```

Reglas:

- El middleware de autenticación resuelve el actor desde el token y la base.
- `PacienteID` y `MedicoID` se resuelven desde los vínculos `pacientes.usuario_id` y `medicos.usuario_id`.
- Los endpoints de portal **no aceptan** `paciente_id` ni `medico_id` del body para decidir el dueño.
- Los repositorios reciben el actor o un filtro de propietario explícito.
- Un paciente sin vínculo recibe `403` o un estado de cuenta pendiente.
- Un médico sin vínculo recibe `403` hasta que el administrador lo vincule.

Permisos propuestos:

| Rol | Permisos nuevos principales |
|---|---|
| `paciente` | `portal:perfil:leer`, `portal:perfil:escribir`, `portal:citas:leer`, `portal:citas:escribir`, `portal:derivaciones:leer`, `catalogo:leer` |
| `medico` | `medico:agenda:leer`, `medico:agenda:escribir`, `medico:nota:escribir`, `derivaciones:leer`, `derivaciones:escribir`, `pacientes:atendidos:leer` |
| `recepcion` | `agenda:gestionar`, `pacientes:leer`, `pacientes:transferir`, `citas:leer`, `citas:escribir`, `cuentas:vincular` |
| `admin` | Todo lo anterior más `derivaciones:sobrescribir`, `especialidades:escribir`, `usuarios:escribir` |
| `auditor` | Lectura de auditoría, errores y reportes; sin escritura clínica |

Recomendación: **retirar** `citas:leer` y `pacientes:leer` globales del rol `medico` y reemplazarlos por permisos “propios” o “de pacientes atendidos”. Si se mantienen por compatibilidad, el servicio debe aplicar igualmente el filtro por `medico_id`.

### 4.3 Nuevos módulos de servicio

| Módulo | Responsabilidad |
|---|---|
| `CuentaService` | Registro público, verificación de email, recuperación de contraseña, vinculación de cuentas |
| `AgendaService` | Horarios, bloqueos, generación de slots, reglas de antelación y cancelación |
| `PortalPacienteService` | Perfil propio, catálogo visible, citas propias y derivaciones propias |
| `MedicoPortalService` | Agenda propia, transiciones de estado, nota de consulta |
| `DerivacionService` | Creación, validación, consumo, liberación, vencimiento y sobrescritura |
| `NotificacionService` | Outbox, plantillas, envío y reintentos |

No es obligatorio crear todos como paquetes separados; sí separar responsabilidades. Si se prefiere, `CitaService` puede orquestar la reserva, pero la validación de derivación debe estar en `DerivacionService` y la disponibilidad en `AgendaService`.

### 4.4 Transacciones y concurrencia

| Operación | Estrategia |
|---|---|
| Registro de paciente | Transacción única: usuario, persona, paciente, rol, token de verificación, auditoría |
| Verificación de email | `UPDATE ... WHERE usado_en IS NULL AND expira_en > now() RETURNING` |
| Reserva de cita | Transacción: validar derivación con `FOR UPDATE`, validar slot, insertar cita, actualizar derivación |
| Uso de derivación | Bloqueo de fila o `UPDATE` atómico con estado esperado |
| Cancelación de cita | `FOR UPDATE` de la cita; actualizar estado; liberar derivación si aplica |
| Reprogramación | Optimistic lock por `version`; si no coincide, `409` |
| Creación de horario | Transacción + validación de solapes del mismo profesional |
| Registro duplicado | Índices únicos de username, email, documento y vínculo |

La regla de oro: **la validación de negocio y la escritura deben ocurrir en la misma transacción**. Si se valida la derivación y luego se inserta la cita en otra conexión, dos requests pueden consumir la misma derivación.

### 4.5 Errores de dominio

Agregar errores tipados en `internal/service/entities` y mapearlos en `transport/http`:

- `ErrRequiereDerivacion`
- `ErrDerivacionVencida`
- `ErrDerivacionNoDisponible`
- `ErrDerivacionEspecialidadIncorrecta`
- `ErrDerivacionMedicoIncorrecto`
- `ErrCitaOrigenNoAtendida`
- `ErrSlotNoDisponible`
- `ErrFueraDeHorario`
- `ErrCuentaNoVerificada`
- `ErrDocumentoYaRegistrado`
- `ErrEmailYaRegistrado`
- `ErrCuentaYaVinculada`

Ninguno debe terminar en `500`. El formato de error debe conservar el `request_id`/`trace_id` actual.

### 4.6 Configuración nueva

Variables propuestas:

```env
PORTAL_PACIENTE_ENABLED=true
DERIVACIONES_ENABLED=true
TRANSFERENCIAS_PACIENTE_ENABLED=false
CLINIC_TIMEZONE=America/Guayaquil
PORTAL_MIN_NOTICE_HOURS=2
PORTAL_MAX_ADVANCE_DAYS=90
PORTAL_CANCEL_CUTOFF_HOURS=24
AUTO_CONFIRM_PORTAL_BOOKINGS=true
REFERRAL_VALID_DAYS=90
PUBLIC_BASE_URL=http://localhost:8080
SMTP_HOST=
SMTP_PORT=587
SMTP_USER=
SMTP_PASSWORD=
SMTP_FROM=
```

Los valores mostrados son defaults de desarrollo. En producción, `PORTAL_PACIENTE_ENABLED`, `DERIVACIONES_ENABLED` y `TRANSFERENCIAS_PACIENTE_ENABLED` se despliegan en `false` y se activan por entorno después de los smoke tests. Las flags se leen al iniciar la API, no por request.

Estrategia de migración: aditiva y expand/contract. Primero se agregan tablas/columnas compatibles, luego se despliega el código que las usa, después se activa la flag y solo en un release posterior se retira lo obsoleto. Nunca se elimina una columna o tabla en el mismo release que introduce el cambio.

No hardcodear códigos de especialidad en el código. La lista directa se controla con `especialidades.requiere_derivacion`.

---

## 5. Modelo de datos y migraciones

Numeración continua desde la última migración aplicada (`014_pediatria_acceso_directo.sql`). Cada migración debe ser idempotente en lo posible, probarse desde cero y probarse sobre una copia de la base actual.

### 5.1 Migración 009 — cuentas, catálogo y permisos

**Objetivo:** preparar usuarios de paciente, vínculos y catálogo.

#### `especialidades`

Agregar:

```sql
ALTER TABLE especialidades
    ADD COLUMN IF NOT EXISTS requiere_derivacion BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS activa BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS orden INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS duracion_cita_min INTEGER NOT NULL DEFAULT 30
        CHECK (duracion_cita_min BETWEEN 5 AND 240);
```

Seed:

- `FISIO` — Fisioterapia — directo.
- `ODONTO` — Odontología — directo.
- `NUTRI` — Nutrición — directo.
- `OTORRINO` — Otorrinolaringología — con derivación.
- `MED-GEN` — actualizar a directo.
- Las especialidades existentes restantes quedan con derivación por defecto.

#### `pacientes`

```sql
ALTER TABLE pacientes
    ADD COLUMN IF NOT EXISTS usuario_id BIGINT UNIQUE
        REFERENCES usuarios (id) ON DELETE SET NULL;
```

- Un paciente puede existir sin cuenta (paciente presencial).
- Una cuenta de paciente debe tener como máximo un perfil.
- La vinculación se hace por recepción/administración o por un flujo de verificación posterior.

#### `medicos`

```sql
ALTER TABLE medicos
    ADD COLUMN IF NOT EXISTS usuario_id BIGINT UNIQUE
        REFERENCES usuarios (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS duracion_cita_min INTEGER NOT NULL DEFAULT 30
        CHECK (duracion_cita_min BETWEEN 5 AND 240),
    ADD COLUMN IF NOT EXISTS bio TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS foto_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS acepta_portal BOOLEAN NOT NULL DEFAULT TRUE;
```

- El médico no se autorregistra.
- Administración crea el usuario y lo vincula.
- Un usuario no debería quedar vinculado a paciente y médico a la vez; validarlo en servicio.

#### `usuarios`

```sql
ALTER TABLE usuarios
    ADD COLUMN IF NOT EXISTS email_verificado_en TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS terminos_aceptados_en TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS version_terminos VARCHAR(20) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS debe_cambiar_password BOOLEAN NOT NULL DEFAULT FALSE;
```

#### `tokens_cuenta`

```sql
CREATE TABLE IF NOT EXISTS tokens_cuenta (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    usuario_id  BIGINT NOT NULL REFERENCES usuarios (id) ON DELETE CASCADE,
    tipo        VARCHAR(30) NOT NULL
        CHECK (tipo IN ('verificacion_email', 'reset_password')),
    token_hash  CHAR(64) NOT NULL UNIQUE,
    expira_en   TIMESTAMPTZ NOT NULL,
    usado_en    TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS tokens_cuenta_usuario_idx
    ON tokens_cuenta (usuario_id, tipo, expira_en);
```

- Guardar solo el hash, nunca el token plano.
- Un token es de un solo uso.
- Al usarse, marcar `usado_en` de forma atómica.

#### Rol y permisos

- Crear rol `paciente`.
- Asignar permisos de portal.
- Agregar permisos de agenda/derivaciones a `medico`, `recepcion` y `admin`.
- Retirar permisos globales peligrosos del rol `medico` o aplicar scoping por fila.
- Auditar el cambio de permisos.

#### Backfill

- Los pacientes existentes quedan con `usuario_id = NULL`.
- Los médicos existentes quedan con `usuario_id = NULL` hasta que administración cree/vincula cuentas.
- Las especialidades existentes quedan `activa = true`.
- `MED-GEN` queda directo; el resto con derivación.

### 5.2 Migración 010 — disponibilidad

#### `horarios_profesional`

```sql
CREATE TABLE IF NOT EXISTS horarios_profesional (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    medico_id         BIGINT NOT NULL REFERENCES medicos (id) ON DELETE CASCADE,
    dia_semana        SMALLINT NOT NULL CHECK (dia_semana BETWEEN 1 AND 7),
    hora_inicio       TIME NOT NULL,
    hora_fin          TIME NOT NULL,
    duracion_slot_min INTEGER NOT NULL DEFAULT 30
        CHECK (duracion_slot_min BETWEEN 5 AND 240),
    vigente_desde     DATE NOT NULL DEFAULT CURRENT_DATE,
    vigente_hasta     DATE,
    activo            BOOLEAN NOT NULL DEFAULT TRUE,
    version           BIGINT NOT NULL DEFAULT 1,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (hora_fin > hora_inicio),
    CHECK (vigente_hasta IS NULL OR vigente_hasta >= vigente_desde)
);

CREATE INDEX IF NOT EXISTS horarios_profesional_medico_idx
    ON horarios_profesional (medico_id, dia_semana, activo);
```

Reglas:

- `1 = lunes`, `7 = domingo`.
- Los horarios son locales en `CLINIC_TIMEZONE`; al generar slots se convierten a UTC.
- No se permiten dos horarios activos solapados del mismo médico para el mismo día y rango de vigencia. Validar en servicio y, si se quiere defensa extra, con una restricción de exclusión.
- Un médico puede tener varios bloques el mismo día (mañana/tarde).

#### `bloqueos_agenda`

```sql
CREATE TABLE IF NOT EXISTS bloqueos_agenda (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    medico_id   BIGINT NOT NULL REFERENCES medicos (id) ON DELETE CASCADE,
    inicio      TIMESTAMPTZ NOT NULL,
    fin         TIMESTAMPTZ NOT NULL,
    motivo      TEXT NOT NULL DEFAULT '',
    creado_por  BIGINT REFERENCES usuarios (id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (fin > inicio)
);

CREATE INDEX IF NOT EXISTS bloqueos_agenda_medico_idx
    ON bloqueos_agenda (medico_id, inicio, fin);
```

Los bloqueos se restan de los slots disponibles.

**Implementación en la migración 010:**

- `horarios_profesional_sin_solape` usa una exclusión GiST sobre `medico_id`, `dia_semana`, el rango de horas `[inicio, fin)` y el rango de vigencia `[vigente_desde, vigente_hasta]`; `vigente_hasta = NULL` significa vigencia indefinida. Solo aplica a filas `activo = true`.
- `bloqueos_agenda_medico_sin_solape` impide dos bloqueos solapados del mismo médico usando `tstzrange(inicio, fin, '[)')`.
- `hora_a_segundos(TIME)` es `IMMUTABLE` y permite indexar el rango horario sin depender de la zona horaria.
- Ambas tablas tienen trigger de `updated_at`; `horarios_profesional` tiene `version` para el futuro CRUD con versión optimista.
- La prueba `internal/repository/postgres/disponibilidad_concurrencia_test.go` abre dos transacciones y compite por el mismo horario/bloqueo. Se ejecuta con `scripts/prueba_concurrencia_disponibilidad.ps1` sobre una base temporal.

### 5.3 Migración 011 — derivaciones y metadata de citas

#### `derivaciones`

```sql
CREATE TABLE IF NOT EXISTS derivaciones (
    id                      BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    paciente_id             BIGINT NOT NULL REFERENCES pacientes (id) ON DELETE RESTRICT,
    medico_origen_id        BIGINT NOT NULL REFERENCES medicos (id) ON DELETE RESTRICT,
    cita_origen_id          BIGINT NOT NULL REFERENCES citas (id) ON DELETE RESTRICT,
    especialidad_destino_id BIGINT NOT NULL REFERENCES especialidades (id) ON DELETE RESTRICT,
    medico_destino_id       BIGINT REFERENCES medicos (id) ON DELETE SET NULL,
    motivo                  TEXT NOT NULL,
    prioridad               VARCHAR(15) NOT NULL DEFAULT 'normal'
        CHECK (prioridad IN ('normal', 'preferente', 'urgente')),
    estado                  VARCHAR(15) NOT NULL DEFAULT 'pendiente'
        CHECK (estado IN ('pendiente', 'agendada', 'atendida', 'vencida', 'cancelada')),
    vigente_desde           TIMESTAMPTZ NOT NULL DEFAULT now(),
    vigente_hasta           TIMESTAMPTZ NOT NULL,
    motivo_cancelacion      TEXT NOT NULL DEFAULT '',
    version                 BIGINT NOT NULL DEFAULT 1,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (vigente_hasta > vigente_desde)
);

CREATE INDEX IF NOT EXISTS derivaciones_paciente_idx
    ON derivaciones (paciente_id, estado, vigente_hasta);

CREATE INDEX IF NOT EXISTS derivaciones_origen_idx
    ON derivaciones (cita_origen_id);

CREATE INDEX IF NOT EXISTS derivaciones_destino_idx
    ON derivaciones (especialidad_destino_id, estado);
```

Reglas de integridad que debe garantizar el servicio:

- `cita_origen.paciente_id = paciente_id`.
- `cita_origen.medico_id = medico_origen_id`.
- `cita_origen.estado = 'atendida'`.
- La especialidad del médico de origen es `MED-GEN`.
- La especialidad destino tiene `requiere_derivacion = true`.
- `vigente_hasta` se calcula con `REFERRAL_VALID_DAYS`.
- Si se especifica `medico_destino_id`, su especialidad debe coincidir con `especialidad_destino_id`.

Se puede agregar un trigger de validación, pero la fuente principal debe ser el servicio transaccional. El trigger es defensa en profundidad, no reemplazo de la prueba de negocio.

**Implementación en la migración 011:**

- `trg_derivaciones_consistencia` valida paciente, médico y cita de origen, que la cita esté `atendida`, que el origen sea Medicina General, que la especialidad destino requiera derivación y que el médico destino coincida con esa especialidad. Los errores usan `23514`.
- `trg_citas_derivacion_consistencia` se dispara al insertar una cita o al cambiar `derivacion_id`, `paciente_id` o `medico_id`; exige la misma paciente, una derivación `pendiente` o `agendada`, vigencia futura y un médico de la especialidad destino. Marcar la cita como `atendida` no vuelve a consumir la derivación, porque el trigger no observa cambios de `estado`.
- `trg_notas_cita_medico` impide que un médico distinto al de la cita registre la nota.
- `citas_derivacion_activa_uk` incluye las citas `programada`, `confirmada` y `atendida`; una cita cancelada, no asistida o reprogramada libera la derivación.
- `creada_por_usuario_id` queda `NULL` en citas históricas. `canal` queda en `recepcion` para no reinterpretar el flujo administrativo existente.
- `notas_cita` tiene una fila por cita (`cita_id UNIQUE`), versión optimista, trigger de `updated_at` e índice por médico.

#### `citas`

Agregar:

```sql
ALTER TABLE citas
    ADD COLUMN IF NOT EXISTS derivacion_id BIGINT
        REFERENCES derivaciones (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS canal VARCHAR(20) NOT NULL DEFAULT 'recepcion'
        CHECK (canal IN ('recepcion', 'portal_paciente', 'medico', 'admin', 'legacy')),
    ADD COLUMN IF NOT EXISTS creada_por_usuario_id BIGINT
        REFERENCES usuarios (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS motivo_cancelacion TEXT NOT NULL DEFAULT '';
```

- Citas existentes: `canal = 'legacy'` o `'recepcion'`; no se debe romper el histórico.
- `creada_por_usuario_id` se llena con el actor; para citas antiguas queda `NULL`.
- `derivacion_id` solo se llena en citas de especialista con derivación.
- El índice parcial de una derivación activa por cita ya se mostró en la sección 3.3.

#### Nota mínima de consulta

```sql
CREATE TABLE IF NOT EXISTS notas_cita (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    cita_id     BIGINT NOT NULL UNIQUE REFERENCES citas (id) ON DELETE CASCADE,
    medico_id   BIGINT NOT NULL REFERENCES medicos (id) ON DELETE RESTRICT,
    motivo      TEXT NOT NULL DEFAULT '',
    evaluacion  TEXT NOT NULL DEFAULT '',
    plan        TEXT NOT NULL DEFAULT '',
    version     BIGINT NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

Una cita puede tener una nota; la nota no reemplaza la historia clínica completa.

### 5.4 Migración 012 — solicitudes de vinculación

**Estado:** aplicada. Archivo `migrations/012_cuentas_vinculacion.sql`.

```sql
CREATE TABLE IF NOT EXISTS solicitudes_vinculacion (
    id                        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    usuario_id                BIGINT NOT NULL REFERENCES usuarios (id) ON DELETE CASCADE,
    paciente_id               BIGINT NOT NULL REFERENCES pacientes (id) ON DELETE CASCADE,
    estado                    VARCHAR(20) NOT NULL DEFAULT 'pendiente'
        CHECK (estado IN ('pendiente', 'aprobada', 'rechazada')),
    motivo                    TEXT NOT NULL DEFAULT '',
    resuelta_por_usuario_id   BIGINT REFERENCES usuarios (id) ON DELETE SET NULL,
    resuelta_en               TIMESTAMPTZ,
    created_at                TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS solicitudes_vinculacion_paciente_pendiente_uk
    ON solicitudes_vinculacion (paciente_id)
    WHERE estado = 'pendiente';

CREATE UNIQUE INDEX IF NOT EXISTS solicitudes_vinculacion_usuario_pendiente_uk
    ON solicitudes_vinculacion (usuario_id)
    WHERE estado = 'pendiente';
```

- El registro con documento existente crea una cuenta pendiente y una solicitud; no duplica persona ni paciente.
- Aprobar vincula `pacientes.usuario_id` y resuelve la solicitud en la misma transacción.
- Rechazar exige motivo, desactiva la cuenta y revoca sus refresh tokens.
- Los índices parciales impiden dos solicitudes pendientes para la misma historia o cuenta.

### 5.5 Migración 013 — ownership del médico

**Estado:** aplicada. Archivo `migrations/013_ownership_medico.sql`.

- Retira del rol `medico` los permisos globales `pacientes:leer`, `citas:leer` y `citas:escribir`.
- Reafirma de forma idempotente `medico:agenda:*`, `medico:nota:*`, `pacientes:atendidos:leer` y `derivaciones:*`.
- No modifica tablas ni datos clínicos.
- La API añade una guarda por roles en los endpoints administrativos para que un access token emitido antes de la migración no siga usándolos hasta expirar.

### 5.6 Migración 014 — Pediatría de acceso directo

**Estado:** aplicada. Archivo `migrations/014_pediatria_acceso_directo.sql`.

```sql
UPDATE especialidades
SET requiere_derivacion = FALSE,
    version = version + 1
WHERE codigo = 'PED'
  AND requiere_derivacion = TRUE;
```

- Pediatría queda como quinta especialidad de acceso directo.
- No se introduce una regla por edad ni una lista hardcodeada; la fuente de verdad sigue siendo `especialidades.requiere_derivacion`.
- La migración es idempotente y no modifica otras especialidades.

### 5.7 Migración 015 — transferencia interna (propuesta)

**Estado:** pendiente; se implementa en la Fase 6. Archivo previsto `migrations/015_transferencias_paciente.sql`.

```sql
ALTER TABLE pacientes
    ADD COLUMN IF NOT EXISTS medico_responsable_id BIGINT
        REFERENCES medicos (id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS pacientes_medico_responsable_idx
    ON pacientes (medico_responsable_id)
    WHERE medico_responsable_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS transferencias_paciente (
    id                     BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    paciente_id            BIGINT NOT NULL REFERENCES pacientes (id) ON DELETE RESTRICT,
    medico_origen_id       BIGINT REFERENCES medicos (id) ON DELETE SET NULL,
    medico_destino_id      BIGINT NOT NULL REFERENCES medicos (id) ON DELETE RESTRICT,
    motivo                 TEXT NOT NULL,
    creada_por_usuario_id  BIGINT NOT NULL REFERENCES usuarios (id) ON DELETE RESTRICT,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO permisos (codigo, modulo, descripcion)
VALUES ('pacientes:transferir', 'pacientes', 'Cambiar el médico tratante dentro de la clínica')
ON CONFLICT (codigo) DO NOTHING;

INSERT INTO rol_permisos (rol_id, permiso_id)
SELECT r.id, p.id
FROM roles r
JOIN permisos p ON p.codigo = 'pacientes:transferir'
WHERE r.codigo IN ('admin', 'recepcion')
ON CONFLICT DO NOTHING;
```

- Solo transferencia interna; no hay traslado a otra institución.
- El historial es append-only y la asignación actual se refleja en `pacientes.medico_responsable_id`.
- El nuevo médico obtiene acceso por asignación activa; el anterior conserva acceso solo por citas vigentes/atendidas.

### 5.8 Migración 016 — notificaciones y outbox (propuesta)

Para no perder correos si el proveedor falla:

```sql
CREATE TABLE IF NOT EXISTS notificaciones (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    usuario_id     BIGINT REFERENCES usuarios (id) ON DELETE SET NULL,
    canal          VARCHAR(15) NOT NULL CHECK (canal IN ('email', 'sms', 'push', 'in_app')),
    tipo           VARCHAR(40) NOT NULL,
    destinatario   TEXT NOT NULL,
    asunto         TEXT NOT NULL DEFAULT '',
    cuerpo         TEXT NOT NULL DEFAULT '',
    estado         VARCHAR(15) NOT NULL DEFAULT 'pendiente'
        CHECK (estado IN ('pendiente', 'enviada', 'fallida', 'cancelada')),
    intentos       INTEGER NOT NULL DEFAULT 0,
    programada_en  TIMESTAMPTZ NOT NULL DEFAULT now(),
    enviada_en     TIMESTAMPTZ,
    ultimo_error   TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS notificaciones_pendientes_idx
    ON notificaciones (estado, programada_en);
```

- El registro de la cita y la notificación se harán en la misma transacción (outbox).
- Un worker separado envía y reintenta.
- Nunca se guardan tokens de verificación/reset en texto plano ni en logs.
- La Fase 2 usa un adaptador de archivo para desarrollo; para producción, SMTP/SES/SendGrid detrás de `service.EmailSender`.

### 5.9 Backfill y compatibilidad

- No eliminar columnas existentes.
- No cambiar el significado de `citas.estado` existente.
- Las citas históricas no tienen derivación; se quedan con `derivacion_id = NULL`.
- Los pacientes históricos no tienen cuenta; se vinculan cuando lo soliciten.
- Los médicos históricos no tienen usuario; administración los vincula.
- Los permisos globales de pacientes/citas para el rol `medico` se retiraron en la migración 013; la API mantiene además una guarda por roles para tokens antiguos.
- La migración 014 solo cambia el flag de Pediatría; no altera historias clínicas.
- La migración 015 deja `medico_responsable_id` nulo en pacientes existentes; la transferencia es opcional y se registra cuando ocurra.
- Probar la migración sobre una copia de la base actual y sobre una base vacía.

---

## 6. API propuesta

### 6.1 Convenciones

- Base: `/api/v1`.
- Autenticación: `Authorization: Bearer <access_token>`.
- Respuestas exitosas: `{ "data": ... }` como en el resto de la API.
- Errores: `{ "error": { "code": "...", "message": "..." }, "request_id": "..." }`.
- Fechas: RFC 3339 con offset; en base `timestamptz`.
- Paginación: `page`, `page_size`, `total`.
- Los endpoints de portal nunca aceptan `paciente_id` del cliente para decidir el dueño.
- Los endpoints de médico nunca aceptan `medico_id` del cliente para decidir el dueño.
- Los endpoints administrativos existentes se mantienen para recepción/admin.

### 6.2 Autenticación y cuenta

| Método | Ruta | Acceso | Descripción |
|---|---|---|---|
| `POST` | `/auth/register` | Público, rate-limited | Registra paciente, persona, perfil y cuenta |
| `POST` | `/auth/verify-email` | Público | Verifica el token de email |
| `POST` | `/auth/resend-verification` | Público, rate-limited | Reenvía verificación |
| `POST` | `/auth/forgot-password` | Público, rate-limited | Solicita recuperación |
| `POST` | `/auth/reset-password` | Público, rate-limited | Cambia contraseña con token |
| `POST` | `/auth/login` | Público | Bloquea cuentas exclusivamente `paciente` sin email verificado |
| `POST` | `/auth/refresh` | Público | Rotación atómica; también bloquea paciente sin verificar |
| `POST` | `/auth/logout` | Autenticado | Ya existe |
| `GET` | `/auth/me` | Autenticado | Devuelve roles, permisos y estado de verificación; falta `paciente_id`/`medico_id` |
| `GET` | `/cuentas/vinculaciones` | `cuentas:vincular` | Lista solicitudes por estado, búsqueda y paginación |
| `POST` | `/cuentas/vinculaciones/{id}/aprobar` | `cuentas:vincular` | Vincula la cuenta con la historia existente |
| `POST` | `/cuentas/vinculaciones/{id}/rechazar` | `cuentas:vincular` | Rechaza con motivo y desactiva la cuenta pendiente |
| `POST` | `/cuentas/vincular-paciente` | `cuentas:vincular` | Vinculación directa desde recepción |
| `POST` | `/cuentas/vincular-medico` | `usuarios:escribir` | Vincula una cuenta con rol `medico` a un perfil existente |

Registro propuesto:

```json
{
  "username": "juan.perez",
  "email": "juan.perez@example.com",
  "password": "UnaClaveSegura123!",
  "nombres": "Juan",
  "apellidos": "Pérez",
  "tipo_documento": "cedula",
  "numero_documento": "0102030405",
  "fecha_nacimiento": "1990-05-12",
  "sexo": "masculino",
  "telefono": "+593999999999",
  "direccion": "Av. Principal y Calle 1",
  "acepta_terminos": true,
  "version_terminos": "2026-10"
}
```

Respuesta recomendada:

```json
{
  "data": {
    "usuario_id": 42,
    "paciente_id": 18,
    "email": "juan.perez@example.com",
    "email_verificado": false,
    "vinculacion_pendiente": false,
    "mensaje": "Revisa tu correo para verificar la cuenta."
  }
}
```

Si el documento existe sin cuenta, la respuesta es `202` y agrega `solicitud_id` con `vinculacion_pendiente: true`.

Decisiones implementadas:

- No se emiten tokens hasta verificar el email; el login y refresh de una cuenta exclusivamente `paciente` se rechazan con `403 email_no_verificado`.
- El reenvío de verificación y la solicitud de reset responden de forma genérica para no filtrar si la cuenta existe. El registro con email duplicado devuelve `409`, porque el alta no puede completarse sin ambigüedad.
- Si el documento ya existe como paciente **sin cuenta**, no se crea otro paciente: se crea una cuenta pendiente y una solicitud de vinculación. Si ya tiene una cuenta vinculada, devuelve `409`.
- Si el documento no existe, se crean `persona`, `paciente`, `usuario`, rol `paciente`, token y auditoría en una sola transacción.
- Se guarda la aceptación y la versión vigente de términos; el registro exige la mayoría de edad configurable.
- Los tokens de verificación/reset se guardan como hash SHA-256, vencen y son de un solo uso. El reset verifica el email si faltaba, limpia bloqueos, incrementa `token_version` y revoca refresh tokens.
- Se auditan registro, verificación, reset, reenvío, solicitud de recuperación y aprobación/rechazo de vinculación.
- El adaptador de correo es de desarrollo (`outbox`); antes de producción se debe implementar un proveedor real detrás de `service.EmailSender`.

### 6.3 Portal del paciente

| Método | Ruta | Permiso | Descripción |
|---|---|---|---|
| `GET` | `/portal/perfil` | `portal:perfil:leer` | Datos propios y estado de cuenta (implementado en Fase 3) |
| `PUT` | `/portal/perfil` | `portal:perfil:escribir` | Actualiza contacto; no documento ni datos clínicos (implementado en Fase 3) |
| `GET` | `/portal/especialidades` | `catalogo:leer` | Catálogo visible, con `requiere_derivacion` |
| `GET` | `/portal/medicos` | `catalogo:leer` | Médicos activos; filtro por especialidad |
| `GET` | `/portal/disponibilidad` | `portal:citas:leer` | Slots disponibles por médico/rango |
| `POST` | `/portal/citas` | `portal:citas:escribir` | Agenda cita propia (implementado en Fase 5) |
| `GET` | `/portal/citas` | `portal:citas:leer` | Lista solo citas propias (implementado en Fase 5) |
| `GET` | `/portal/citas/{id}` | `portal:citas:leer` | Detalle solo si es propia (implementado en Fase 5) |
| `PUT` | `/portal/citas/{id}` | `portal:citas:escribir` | Reprograma con `version` (implementado en Fase 5) |
| `POST` | `/portal/citas/{id}/cancelar` | `portal:citas:escribir` | Cancela si cumple política (implementado en Fase 5) |
| `GET` | `/portal/derivaciones` | `portal:derivaciones:leer` | Derivaciones propias |
| `GET` | `/portal/derivaciones/{id}` | `portal:derivaciones:leer` | Detalle propio |

Reserva directa:

```json
{
  "medico_id": 12,
  "fecha_hora_inicio": "2026-11-03T09:00:00-05:00",
  "motivo": "Dolor lumbar",
  "derivacion_id": null
}
```

Reserva de especialista:

```json
{
  "medico_id": 25,
  "fecha_hora_inicio": "2026-11-10T10:30:00-05:00",
  "motivo": "Evaluación otorrinolaringológica",
  "derivacion_id": 87
}
```

Validaciones del servidor:

- `paciente_id` se obtiene del actor, nunca del body.
- El médico debe estar activo, aceptar portal y pertenecer a la especialidad.
- La fecha debe ser un slot válido y futuro.
- Si la especialidad requiere derivación, validar la derivación en la misma transacción.
- Si no requiere, ignorar `derivacion_id` o rechazarlo si viene informado.
- El estado inicial es `programada` o `confirmada` según configuración.
- `canal` se fuerza a `portal_paciente`.
- `creada_por_usuario_id` se toma del actor.

Respuesta de cita recomendada:

```json
{
  "data": {
    "id": 501,
    "paciente_id": 42,
    "medico_id": 25,
    "especialidad": "Otorrinolaringología",
    "fecha_hora_inicio": "2026-11-10T15:30:00Z",
    "duracion_min": 30,
    "estado": "programada",
    "canal": "portal_paciente",
    "derivacion_id": 87,
    "version": 1
  }
}
```

### 6.4 Portal del médico

| Método | Ruta | Permiso | Descripción |
|---|---|---|---|
| `GET` | `/medico/agenda` | `medico:agenda:leer` | Citas propias por rango/estado (básico implementado en Fase 3) |
| `GET` | `/medico/citas/{id}` | `medico:agenda:leer` | Detalle solo si es su cita (lectura implementada en Fase 3) |
| `PATCH` | `/medico/citas/{id}/estado` | `medico:agenda:escribir` | Confirma, atiende o marca no asistió |
| `POST` | `/medico/citas/{id}/nota` | `medico:nota:escribir` | Nota mínima de consulta |
| `PUT` | `/medico/citas/{id}/nota` | `medico:nota:escribir` | Actualiza nota con `version` |
| `POST` | `/medico/citas/{id}/derivaciones` | `derivaciones:escribir` | Emite derivación desde cita atendida |
| `GET` | `/medico/derivaciones` | `derivaciones:leer` | Derivaciones emitidas por el médico |
| `GET` | `/medico/pacientes/{id}` | `pacientes:atendidos:leer` | Paciente con relación de atención (implementado en Fase 3) |

Transiciones permitidas al médico:

- `programada -> confirmada`
- `programada/confirmada -> atendida`
- `programada/confirmada -> no_asistio`
- No puede pasar a `cancelada` salvo que se implemente una cancelación médica con motivo.
- No puede modificar una cita `atendida`, `no_asistio`, `cancelada` o `reprogramada` sin una operación administrativa especial.

Derivación propuesta:

```json
{
  "especialidad_destino_id": 9,
  "medico_destino_id": null,
  "motivo": "Paciente con hipoacusia y otalgia recurrente; requiere valoración especializada.",
  "prioridad": "normal"
}
```

Validaciones:

- La cita debe ser del médico autenticado.
- La cita debe estar `atendida`.
- La especialidad de origen del médico debe ser `MED-GEN` para habilitar el portal.
- La especialidad destino debe requerir derivación.
- Si se indica médico destino, debe pertenecer a la especialidad.
- Calcular `vigente_hasta` con `REFERRAL_VALID_DAYS`.
- Crear la derivación en estado `pendiente`.
- Auditar `crear_derivacion`.

### 6.5 Administración y recepción

| Método | Ruta | Acceso | Descripción |
|---|---|---|---|
| `POST` | `/pacientes/{id}/cuenta` | `cuentas:vincular` | Crea/vincula cuenta a paciente existente |
| `POST` | `/pacientes/{id}/transferir` | `pacientes:transferir` | Cambia el médico tratante; exige `medico_destino_id`, motivo y `version` |
| `GET` | `/pacientes/{id}/transferencias` | `pacientes:transferir` | Historial de transferencias internas |
| `POST` | `/medicos/{id}/cuenta` | `usuarios:escribir` | Crea/vincula cuenta de médico |
| `GET` | `/medicos/{id}/horarios` | `agenda:gestionar` | Lista horarios |
| `POST` | `/medicos/{id}/horarios` | `agenda:gestionar` | Crea horario |
| `PUT` | `/medicos/{id}/horarios/{horarioId}` | `agenda:gestionar` | Actualiza con `version` |
| `DELETE` | `/medicos/{id}/horarios/{horarioId}` | `agenda:gestionar` | Desactiva horario |
| `GET` | `/medicos/{id}/bloqueos` | `agenda:gestionar` | Lista bloqueos |
| `POST` | `/medicos/{id}/bloqueos` | `agenda:gestionar` | Crea bloqueo |
| `PUT` | `/medicos/{id}/bloqueos/{bloqueoId}` | `agenda:gestionar` | Actualiza rango y motivo |
| `DELETE` | `/medicos/{id}/bloqueos/{bloqueoId}` | `agenda:gestionar` | Elimina/cancela bloqueo |
| `POST` | `/citas` | `citas:escribir` | Ya existe; recepción/admin agenda |
| `PATCH` | `/citas/{id}/estado` | `citas:escribir` | Ya existe; administración |
| `POST` | `/citas/{id}/sobrescribir-derivacion` | `derivaciones:sobrescribir` | Excepción auditada |
| `GET` | `/derivaciones` | `derivaciones:leer` | Consulta administrativa |
| `GET` | `/especialidades` | `especialidades:leer` | Ya existe; incluir flags |
| `PUT` | `/especialidades/{id}` | `especialidades:escribir` | Cambiar `requiere_derivacion`, orden, duración |

Reglas:

- Recepción puede agendar para un paciente existente; el `canal` es `recepcion`.
- Recepción no puede emitir derivaciones.
- Admin puede sobrescribir con motivo obligatorio, permiso y auditoría.
- La sobrescritura no debe crear una derivación falsa; debe dejar registro de la excepción y de quién la autorizó.
- La vinculación de cuenta debe validar que el paciente/médico no tenga ya otra cuenta.

### 6.6 Respuestas y errores

- `201` al crear cita, cuenta, horario o derivación.
- `200` al consultar o actualizar.
- `204` al cancelar/eliminar cuando no se devuelve cuerpo.
- `400` por payload inválido.
- `401` por token inválido/expirado.
- `403` por falta de permiso, cuenta no verificada o recurso ajeno.
- `404` cuando el recurso no existe o no pertenece al actor (no filtrar existencia).
- `409` por regla de negocio o conflicto de concurrencia.
- `429` por rate limit.
- `500` solo para fallos inesperados, con `trace_id` y error auditado.

No usar `403` para “requiere derivación” si se quiere que el frontend muestre un flujo de negocio; `409` con `code=requiere_derivacion` es más claro. La decisión debe ser consistente en todos los endpoints.

### 6.7 OpenAPI

El esqueleto vive en `docs/openapi.yaml`. Ya incluye autenticación, catálogo, pacientes, médicos, citas administrativas, portal del paciente, portal del médico, cuentas, usuarios, auditoría y errores, con el sobre `data`/`meta`, el formato de error y los códigos base. Los endpoints de las fases 4–8 están listados en `x-pendientes` y se agregarán al contrato al implementarse.

Debe mantenerse sincronizado con los handlers; no dejarlo como archivo abandonado. Al agregar una ruta se actualizan el path, los esquemas de entrada/salida y los ejemplos de `409`.

---

## 7. Roadmap por fases, en orden de prioridad y dependencia

La secuencia está diseñada para no construir endpoints sobre datos que todavía no existen y para no abrir el portal antes de cerrar la autorización por fila.

| Fase | Prioridad | Objetivo | Depende de | Esfuerzo relativo |
|---|---|---|---|---|
| 0 | P0 | Decisiones, reglas y contrato de API | — | S |
| 1 | P0 | Modelo de datos y migraciones | Fase 0 | L |
| 2 | P0 | Registro, verificación y cuentas | Fase 1 | L |
| 3 | P0 | Actor, ownership y permisos por fila | Fases 1–2 | M |
| 4 | P0 | Catálogo, horarios y disponibilidad | Fases 1, 3 | L |
| 5 | P0 | Agenda directa del paciente | Fases 2–4 | M |
| 6 | P0 | Flujo médico mínimo y derivaciones | Fases 3–5 | L |
| 7 | P0 | Reserva de especialista con derivación | Fases 5–6 | L |
| 8 | P1 | Notificaciones y recordatorios | Fases 2, 5–7 | M |
| 9 | P1 | Endurecimiento de seguridad y operación | Fases 2–8 | M |
| 10 | P0 | Pruebas finales, despliegue y documentación | Todas | M |

> Esfuerzo relativo: S = pequeño, M = medio, L = grande. No es una estimación contractual; sirve para ordenar el trabajo.

### Fase 0 — Cerrar decisiones y contrato

**Estado:** cerrada.

**Objetivo:** evitar retrabajo en migraciones y endpoints.

Tareas:

- [x] Confirmar la interpretación de “remitir a otro paciente”: derivación clínica y transferencia interna de médico tratante, sin transferencia externa.
- [x] Confirmar las especialidades de acceso directo: `MED-GEN`, `FISIO`, `ODONTO`, `NUTRI` y `PED`.
- [x] Confirmar a Pediatría como acceso directo para todos, sin regla por edad.
- [x] Definir vigencia de derivación (90 días), política de no-show, cancelación (corte de 24 h) y reprogramación.
- [x] Definir verificación de email antes del login y del refresh.
- [x] Definir el flujo para pacientes que ya existen sin cuenta: vinculación, sin duplicar historia.
- [x] Definir permisos finales de `paciente`, `medico`, `recepcion`, `enfermeria`, `auditor` y `admin`.
- [x] Definir zona horaria (`America/Guayaquil`), slot de 30 min configurable, antelación de 2 h y horizonte de 90 días.
- [x] Crear el esqueleto de `docs/openapi.yaml`.
- [x] Definir códigos de error y formato de respuesta.
- [x] Definir feature flags y estrategia de migración.

**Criterio de salida:** decisiones registradas y contrato de API revisado. No se escribe código de negocio todavía.

### Fase 1 — Modelo de datos y migraciones

**Objetivo:** dejar la base preparada sin romper lo existente.

Tareas:

- Migración `009`: cuentas, vínculos, rol paciente, permisos, tokens, términos, catálogo.
- Migración `010`: horarios y bloqueos.
- Migración `011`: derivaciones, metadata de citas y nota mínima.
- Migración `012`: solicitudes de vinculación.
- Migración `013`: ownership del médico y retiro de permisos globales.
- Migración `014` (P1): outbox de notificaciones.
- Crear entidades: `Derivacion`, `HorarioProfesional`, `BloqueoAgenda`, `NotaCita`, `TokenCuenta`, `Notificacion`.
- Crear stores PostgreSQL correspondientes.
- Actualizar stores existentes para `usuario_id`, `canal`, `derivacion_id`.
- Agregar índices y restricciones.
- Probar migraciones desde cero y sobre copia de la base actual.
- Documentar el orden y el rollback.

**Criterio de salida:** migraciones aplican sin pérdida de datos; tests de repositorio pasan; la base actual sigue funcionando.

### Fase 2 — Registro, verificación y cuentas

**Estado:** completado.

**Objetivo:** que un paciente pueda crear una cuenta real y quedar vinculado a su perfil.

Tareas:

- [x] `POST /auth/register` transaccional.
- [x] Validación de username, email, documento, edad y términos.
- [x] Creación de `persona`, `paciente`, `usuario`, rol `paciente`.
- [x] Token de verificación de email con hash, expiración y un solo uso.
- [x] `POST /auth/verify-email` y reenvío.
- [x] `POST /auth/forgot-password` y `POST /auth/reset-password`.
- [x] Revocación de sesiones al cambiar contraseña.
- [x] Adaptador de email de desarrollo (archivo `.eml`) y puerto para proveedor real.
- [x] Rate limiting de registro, verificación, reenvío, recuperación, reset y login.
- [x] Auditoría de registro, verificación, reset, reenvío y solicitudes.
- [x] Flujo para documento ya existente: no duplicar; solicitud de vinculación aprobada/rechazada por recepción.
- [x] Creación/vinculación de cuenta de médico por administración.

**Criterio de salida:** un paciente se registra, verifica, inicia sesión y su usuario queda ligado a un único paciente. Un documento duplicado no crea otra historia. Los tests cubren tokens vencidos, reutilizados y carreras de registro.

### Fase 3 — Actor, ownership y permisos por fila

**Estado:** completada.

**Objetivo:** cerrar el riesgo de que un paciente o médico vea datos ajenos.

Tareas:

- [x] Extender el middleware para construir `Actor`.
- [x] Resolver `PacienteID` y `MedicoID` desde la base.
- [x] Extender `/auth/me`.
- [x] Implementar `GET/PUT /portal/perfil`.
- [x] Implementar `GET /medico/agenda` básico.
- [x] Aplicar filtros de ownership en citas y pacientes; derivaciones queda pendiente de su API en la Fase 6.
- [x] Retirar o neutralizar permisos globales del rol `medico`.
- [x] Probar IDOR: paciente A contra paciente B; médico A contra médico B; paciente contra datos administrativos.
- [x] Auditar accesos a datos propios y ajenos.

**Criterio de salida:** ningún actor puede leer o modificar recursos de otro cambiando un ID en la URL o en el body. Los tests de autorización fallan si se quita el filtro.

### Fase 4 — Catálogo, horarios y disponibilidad

**Estado:** completada (Paso 8).

**Objetivo:** que el paciente vea opciones reales y slots válidos.

Tareas:

- [x] Actualizar catálogo con `requiere_derivacion`, `activa`, orden y duración.
- [x] Seed de `FISIO`, `ODONTO`, `NUTRI`, `OTORRINO`.
- [x] CRUD administrativo de horarios y bloqueos.
- [x] Perfil visible del médico: especialidad, bio, foto, duración, acepta portal.
- [x] Generación de slots por rango, zona horaria, antelación y máximo de días.
- [x] Restar bloqueos y citas activas.
- [x] `GET /portal/especialidades`, `/portal/medicos`, `/portal/disponibilidad`.
- [x] Validar que un slot no se pueda reservar fuera de horario.
- [x] Pruebas de zona horaria, cambios de horario y solapes.

**Criterio de salida:** los slots mostrados son reservables y no incluyen horarios bloqueados, pasados o ya ocupados. La reserva concurrente del mismo slot (un `201` y un `409`) se cierra en la Fase 5, cuando exista el endpoint de reserva.

### Fase 5 — Agenda directa del paciente

**Estado:** completada (Paso 9).

**Objetivo:** que el paciente agende solo las cinco áreas permitidas.

Tareas:

- [x] `POST /portal/citas`.
- [x] `GET /portal/citas`, detalle, cancelación y reprogramación.
- [x] Forzar `paciente_id` desde el actor.
- [x] Forzar `canal = portal_paciente`.
- [x] Validar especialidad directa; rechazar especialista sin derivación.
- [x] Validar médico activo, acepta portal y especialidad correcta.
- [x] Validar slot, antelación, máximo de días y política de cancelación.
- [x] Usar `version` en reprogramación.
- [x] Integrar con las restricciones de exclusión existentes.
- [x] Auditar creación, cancelación y reprogramación.
- [x] Notificación básica de confirmación: `EmailSender.EnviarCitaCreada` escribe el correo de confirmación en el buzón de desarrollo; el outbox transaccional queda para la Fase 8.

**Criterio de salida:** el paciente agenda Medicina General, Fisioterapia, Odontología, Nutrición y Pediatría; no puede agendar Cardiología u Otorrinolaringología sin derivación; no puede ver ni modificar citas ajenas; las carreras devuelven `409`.

### Fase 6 — Flujo médico mínimo, derivaciones y transferencia interna

**Objetivo:** que el médico atienda y derive correctamente, y que administración pueda reasignar el médico tratante sin romper el ownership.

Tareas:

- `GET /medico/agenda` con filtros y ownership.
- `PATCH /medico/citas/{id}/estado` con transiciones válidas.
- `POST/PUT /medico/citas/{id}/nota`.
- `POST /medico/citas/{id}/derivaciones`.
- `GET /medico/derivaciones`.
- Validar cita atendida, médico de origen, especialidad `MED-GEN`, especialidad destino y vigencia.
- Crear derivación en estado `pendiente` con `vigente_hasta = now() + REFERRAL_VALID_DAYS`.
- Auditar atención, nota y derivación.
- Aplicar migración 015: `pacientes.medico_responsable_id`, historial `transferencias_paciente` y permiso `pacientes:transferir`.
- `POST /pacientes/{id}/transferir` para `admin`/`recepción`, con `medico_destino_id`, motivo, `version` y auditoría.
- `GET /pacientes/{id}/transferencias` para consultar el historial.
- Extender `PerteneceAMedico` para incluir la asignación activa como relación de acceso.
- Probar que un médico no puede derivar desde una cita ajena, no atendida o de otra especialidad.
- Probar que un médico no puede transferirse pacientes a sí mismo ni leer pacientes sin asignación o cita.

**Criterio de salida:** un médico de Medicina General atiende una cita y emite una derivación válida; un médico de otra especialidad no puede habilitar el acceso directo del portal; una transferencia interna cambia el acceso de forma auditada y el médico anterior no conserva acceso salvo por citas vigentes o atendidas.

### Fase 7 — Reserva de especialista con derivación

**Objetivo:** cerrar la regla central del requerimiento.

Tareas:

- `GET /portal/derivaciones` y detalle.
- Mostrar al paciente la derivación vigente y su destino.
- `POST /portal/citas` con `derivacion_id`.
- Validación transaccional con `FOR UPDATE`.
- Consumir la derivación y marcarla `agendada`.
- Índice único parcial para impedir dos citas activas con la misma derivación.
- Cancelación de cita de especialista: liberar la derivación a `pendiente` solo si se cancela con al menos `PORTAL_CANCEL_CUTOFF_HOURS=24` y sigue vigente.
- Cancelación tardía: marcar la derivación `cancelada` con `motivo_cancelacion=cancelacion_tardia`; no reutilizable.
- Reprogramación: mantener la misma derivación si no cambia de especialidad/médico y sigue vigente.
- Marcar derivación `atendida` cuando la cita se atiende.
- Job o consulta para marcar `vencida`.
- No-show: consumir la derivación (`cancelada`, `motivo_cancelacion=no_show`); se requiere nueva valoración de Medicina General.
- Sobrescritura administrativa solo para `admin` con `derivaciones:sobrescribir`, motivo y auditoría.
- Prueba E2E completa.

**Criterio de salida:** el paciente no puede saltarse la regla; la derivación se usa una sola vez; cancelar con 24 h o más la libera; cancelar tarde o no asistir la consume; una derivación vencida no permite reservar; la sobrescritura queda auditada y solo la ejecuta un admin.

### Fase 8 — Notificaciones y recordatorios

**Objetivo:** que el flujo sea usable y no dependa de que el usuario revise la pantalla.

Tareas:

- Worker de outbox con reintentos y backoff.
- Proveedor real de email.
- Plantillas de verificación, reset, cita creada, confirmada, cancelada, reprogramada y derivación emitida.
- Recordatorios configurables (por ejemplo, 24 h y 2 h antes).
- Notificación al paciente cuando el médico emite una derivación.
- Notificación al médico cuando se agenda una cita de su derivación.
- Preferencias de notificación.
- No incluir tokens ni datos clínicos sensibles en logs.
- Idempotencia para no duplicar envíos.

**Criterio de salida:** los correos se encolan en la misma transacción que el evento; los fallos se reintentan; no se pierden notificaciones si el proveedor está caído.

### Fase 9 — Endurecimiento de seguridad y operación

**Objetivo:** pasar de “funciona” a “se puede exponer”.

Tareas:

- Rate limiting distribuido para registro, login, refresh, reset y reservas.
- CORS con allowlist del frontend.
- TLS en proxy reverso; HSTS y headers de seguridad.
- Secretos fuera del repositorio.
- Revisión de IDOR y escalamiento de roles.
- Auditoría completa de acciones nuevas.
- Redacción de PII y datos clínicos en logs.
- Métricas: registros, verificaciones, reservas, conflictos de slot, derivaciones, conversión, cancelaciones, fallos de email.
- Alertas de fallos de auditoría y de outbox.
- Pruebas de carga de login, disponibilidad y reserva.
- Revisión de la retención de datos y consentimiento.
- Documentar runbook y backups.

**Criterio de salida:** checklist de seguridad aprobada; no hay endpoints de portal sin ownership; los errores esperados no generan `500`; las métricas y alertas existen.

### Fase 10 — Pruebas finales, despliegue y documentación

**Objetivo:** liberar con migraciones seguras y rollback claro.

Tareas:

- Suite unitaria, de integración, E2E, concurrencia y seguridad.
- Prueba de migración desde base vacía y desde copia de producción.
- Prueba de regresión de los endpoints existentes.
- OpenAPI actualizado.
- README actualizado con flujo de paciente, médico y derivaciones.
- Feature flags para activar portal y derivaciones por entorno.
- Despliegue en staging; smoke tests.
- Plan de rollback: migraciones aditivas, API anterior compatible.
- Monitoreo post-despliegue.

**Criterio de salida:** el flujo E2E pasa en staging; las migraciones se pueden aplicar con backup; existe runbook; el equipo sabe cómo revertir la activación sin borrar datos.

---

## 8. Pruebas y criterios de aceptación

### 8.1 Pruebas unitarias de servicio

- Catálogo: una especialidad directa no exige derivación; una especialidad con derivación sí.
- Derivación: no se crea desde cita no atendida, cita ajena o especialidad de origen distinta de Medicina General.
- Derivación: no se usa vencida, cancelada, consumida o de otro paciente.
- Derivación: especialidad y médico destino deben coincidir.
- Cita: no se agenda fuera de horario, en bloqueo, en pasado o con antelación insuficiente.
- Cita: no se reprograma una cita atendida, cancelada o no asistida.
- Cuenta: no se registra dos veces el mismo email, username o documento.
- Cuenta: token de verificación/reset vencido o usado no funciona.
- Permisos: paciente no puede usar endpoints administrativos; médico no puede ver agenda ajena.

### 8.2 Pruebas de integración con PostgreSQL

- Migraciones desde cero hasta la última.
- Migraciones sobre copia de la base actual.
- Restricciones de exclusión de citas.
- Índice único parcial de derivación activa.
- Triggers de derivación: origen atendido, paciente y médico correctos, destino con derivación y médico destino de la especialidad.
- Triggers de cita con derivación: mismo paciente, derivación vigente y disponible, médico de la especialidad destino.
- Trigger de nota: solo el médico de la cita puede escribirla; una sola nota por cita.
- Metadata de citas: `canal`, `creada_por_usuario_id`, `motivo_cancelacion` y `derivacion_id` con defaults compatibles.
- Transacción de registro con rollback ante fallo.
- Transacción de reserva con derivación.
- Liberación de derivación al cancelar.
- Versionado optimista de citas, horarios y notas.
- Stores con filtros de ownership.

### 8.3 Prueba E2E del flujo solicitado

1. Paciente se registra con documento nuevo.
2. Verifica email.
3. Inicia sesión.
4. Consulta especialidades y ve cinco directas.
5. Agenda Medicina General.
6. Intenta agendar Otorrinolaringología y recibe `409 requiere_derivacion`.
7. Médico de Medicina General inicia sesión, ve su agenda, marca la cita como `atendida`.
8. Médico registra nota mínima.
9. Médico emite derivación a Otorrinolaringología.
10. Paciente ve la derivación `pendiente`.
11. Paciente agenda Otorrinolaringología con esa derivación.
12. La derivación pasa a `agendada`.
13. Paciente intenta usar la misma derivación para otra cita y recibe `409`.
14. Paciente cancela la cita antes del corte; la derivación vuelve a `pendiente`.
15. Paciente reagenda con la misma derivación; funciona.
16. Especialista atiende; la derivación pasa a `atendida`.
17. Paciente intenta reutilizarla; recibe `409`.
18. Paciente intenta ver la cita de otro paciente; recibe `404`/`403`.
19. Médico intenta ver la agenda de otro médico; recibe `404`/`403`.
20. Admin sobrescribe una derivación con motivo; queda auditoría.

### 8.4 Pruebas de concurrencia

- Dos pacientes reservan el mismo slot: exactamente uno `201`, el otro `409`.
- Dos requests usan la misma derivación: exactamente uno agenda, el otro `409`.
- Cancelación y uso de derivación simultáneos: estado final coherente.
- Dos registros con el mismo email/documento: uno crea, el otro `409`.
- Dos reprogramaciones con la misma `version`: una `200`, una `409`.
- Dos médicos intentan atender la misma cita: una transición válida, la otra `409`.
- Creación simultánea de horarios solapados: no quedan dos horarios activos incompatibles.
- La prueba automatizada de horarios y bloqueos está en `internal/repository/postgres/disponibilidad_concurrencia_test.go` y se ejecuta con `scripts/prueba_concurrencia_disponibilidad.ps1`.
- Dos transacciones reservan con la misma derivación: una confirma y la otra recibe `23505`, verificado por `TestDerivacionConcurrenteSoloPermiteUnaCita`.
- Las pruebas de derivaciones, notas y metadata están en `internal/repository/postgres/derivaciones_migracion_test.go` y se ejecutan con `scripts/prueba_derivaciones.ps1`.

### 8.5 Pruebas de seguridad

- IDOR horizontal y vertical.
- Paciente no puede cambiar `paciente_id` en el body.
- Médico no puede cambiar `medico_id` en el body.
- Paciente no puede usar `POST /citas` administrativo.
- Recepción no puede emitir derivaciones.
- Token de verificación/reset reutilizado.
- Rate limit en registro/login/reset.
- Enumeración de cuentas por mensajes de error.
- Inyección SQL en filtros (pgx parametrizado).
- Logs sin tokens, contraseñas ni notas clínicas completas.

### 8.6 Pruebas de migración y regresión

- Aplicar 009–014 sobre base vacía.
- Aplicar sobre copia de la base actual con citas, pacientes y médicos existentes.
- Verificar que las citas históricas siguen consultables.
- Verificar que los roles actuales no pierden acceso administrativo.
- Verificar que el login existente sigue funcionando.
- Verificar que las restricciones de concurrencia de la migración 008 siguen activas.

### 8.7 CI mínimo

- `gofmt`/`goimports`.
- `go vet ./...`.
- `go test -count=1 ./...`.
- `go test -race` en paquetes críticos.
- Integración con PostgreSQL en un servicio de CI.
- Validación de migraciones.
- Validación de OpenAPI.
- Escaneo de secretos y dependencias.

### 8.8 Definición de Done por historia

Una historia no está terminada si:

- solo funciona en el camino feliz;
- no tiene prueba de ownership;
- no tiene prueba de concurrencia cuando aplica;
- devuelve `500` ante una regla de negocio;
- no audita la acción;
- no actualiza OpenAPI;
- no tiene migración probada;
- deja permisos globales en el rol equivocado.

---

## 9. Seguridad, auditoría y observabilidad

### 9.1 Checklist de seguridad

- [x] Registro público con rate limit y verificación de email.
- [x] Contraseñas con bcrypt y política existente.
- [x] Tokens de verificación/reset hasheados, de un solo uso y con expiración.
- [x] Reset de contraseña revoca sesiones.
- [ ] Paciente solo accede a sus datos.
- [ ] Médico solo accede a su agenda y a pacientes con relación de atención.
- [ ] Recepción no accede a funciones clínicas.
- [ ] Admin con MFA antes de producción (si se habilita).
- [ ] CORS con allowlist.
- [ ] TLS en el proxy.
- [x] Secretos fuera del repositorio.
- [ ] Rate limiting distribuido.
- [x] Validación de payloads y límites de tamaño.
- [ ] Redacción de PII en logs.
- [x] Consentimiento y términos versionados.
- [ ] Backups y restauración probados.
- [ ] Revisión de dependencias y contenedores.

### 9.2 Eventos de auditoría nuevos

| Acción | Recurso | Datos mínimos |
|---|---|---|
| `registro` | `usuario` | usuario_id, email, IP, request_id |
| `verificacion_email` | `usuario` | usuario_id, resultado |
| `reset_password` | `usuario` | usuario_id, resultado, IP |
| `vincular_cuenta` | `paciente`/`medico` | actor, recurso, antes/después |
| `crear_horario` | `horario` | médico, rango, actor |
| `actualizar_horario` | `horario` | antes/después, actor |
| `crear_bloqueo` | `agenda` | médico, rango, motivo |
| `agendar_cita_portal` | `cita` | paciente, médico, slot, canal |
| `cancelar_cita_portal` | `cita` | cita, motivo, actor |
| `reprogramar_cita_portal` | `cita` | cita, slot anterior/nuevo |
| `atender_cita` | `cita` | cita, médico, estado anterior/nuevo |
| `crear_nota` | `cita` | cita, médico |
| `crear_derivacion` | `derivacion` | paciente, origen, destino, vigencia |
| `usar_derivacion` | `derivacion` | derivación, cita, actor |
| `liberar_derivacion` | `derivacion` | derivación, motivo |
| `sobrescribir_derivacion` | `cita` | actor, motivo, autorización |

No guardar en auditoría el contenido completo de la nota clínica ni tokens.

La Fase 2 registra `registro`, `registro_fallido`, `verificacion_email`, `verificacion_email_fallida`, `reset_password`, `reset_password_fallido`, `reenvio_verificacion`, `solicitud_reset_password`, `vinculacion_cuenta_aprobada`, `vinculacion_cuenta_rechazada`, `vinculacion_paciente` y `vinculacion_medico`. Un token inválido/vencido no permite identificar la cuenta, pero el intento queda auditado sin `usuario_id`/`username` y sujeto al rate limiter; el request también aparece en el log de solicitudes sin el token.

### 9.3 Observabilidad

- Reutilizar el `trace_id` y `X-Request-ID` existentes.
- Añadir `usuario_id`, `rol`, `paciente_id`/`medico_id` cuando sea seguro; no añadir datos clínicos.
- Métricas:
  - registros y verificaciones;
  - logins fallidos y bloqueos;
  - citas creadas por canal;
  - conflictos de slot;
  - derivaciones creadas/usadas/vencidas;
  - cancelaciones y no-show;
  - errores de outbox;
  - latencia de disponibilidad y reserva.
- Alertas:
  - picos de `409` por slot;
  - fallos de auditoría;
  - outbox atascado;
  - errores `500` en portal;
  - intentos de acceso denegado repetidos.
- El stack Loki/Grafana actual es local; para producción sigue pendiente el reemplazo de Promtail y la recolección durable. Eso es un trabajo separado de este roadmap, pero debe estar en el checklist de salida.

### 9.4 Privacidad

- Minimizar datos en respuestas de portal.
- No exponer notas clínicas completas al paciente salvo decisión explícita.
- No exponer datos de otros pacientes en catálogos de médicos.
- Definir retención de citas, auditoría y notificaciones.
- Documentar consentimiento informado y tratamiento de datos.
- Si se implementan menores, agregar tutor y consentimiento del representante.

---

## 10. Despliegue y Definition of Done

### 10.1 Orden de despliegue

1. Backup de base.
2. Aplicar migraciones 009–014 en staging.
3. Ejecutar pruebas de migración y regresión.
4. Desplegar API con feature flags apagados.
5. Crear/verificar catálogo y permisos.
6. Vincular cuentas de médicos de prueba.
7. Activar `PORTAL_PACIENTE_ENABLED` en staging.
8. Ejecutar E2E completo.
9. Activar `DERIVACIONES_ENABLED` en staging.
10. Repetir E2E.
11. Producción: migraciones, API, smoke tests, activación gradual.
12. Monitorear errores, conflictos y outbox.

### 10.2 Compatibilidad

- Los endpoints actuales de administración siguen funcionando.
- El rol `medico` puede perder permisos globales; coordinar con el frontend actual.
- Las citas históricas no necesitan derivación.
- Los pacientes históricos no necesitan cuenta para seguir siendo atendidos por recepción.
- Las migraciones son aditivas; evitar `DROP COLUMN` en la primera release.

### 10.3 Definition of Done del backend solicitado

- [ ] Un paciente se registra, verifica su email y entra.
- [ ] Su cuenta está vinculada a un único paciente.
- [ ] Ve solo su perfil, sus citas y sus derivaciones.
- [ ] Agenda directamente Medicina General, Fisioterapia, Odontología, Nutrición y Pediatría.
- [ ] No puede agendar una especialidad con derivación (por ejemplo, Cardiología) sin una derivación vigente.
- [ ] Un médico ve solo su agenda.
- [ ] Un médico de Medicina General marca una cita como atendida y emite una derivación.
- [ ] El paciente usa la derivación para agendar al especialista.
- [ ] La derivación no se puede reutilizar ni usar vencida.
- [ ] Cancelar antes del corte libera la derivación.
- [ ] Recepción/admin puede gestionar horarios y sobrescribir con auditoría.
- [ ] No hay IDOR en los flujos anteriores.
- [ ] Las carreras devuelven `409`, no `500`.
- [ ] Migraciones probadas desde cero y sobre base actual.
- [ ] OpenAPI y README actualizados.
- [ ] Auditoría y métricas de los eventos nuevos.
- [ ] Pruebas unitarias, integración, E2E, concurrencia y seguridad en CI.

---

## 11. Backlog priorizado

### P0 — Necesario para el primer cierre funcional

- Decisiones de negocio y OpenAPI.
- Migraciones 009–011.
- Registro, verificación y recuperación de contraseña.
- Rol `paciente`, vínculo paciente–usuario y médico–usuario.
- Actor y autorización por fila.
- Catálogo con flags de derivación y especialidades nuevas.
- Horarios, bloqueos y disponibilidad.
- Agenda directa del paciente.
- Flujo médico mínimo, nota y derivación.
- Reserva de especialista con derivación.
- Cancelación, reprogramación y liberación de derivación.
- Sobrescritura administrativa excepcional, con motivo y auditoría.
- Pruebas E2E, concurrencia y seguridad.
- Auditoría de eventos nuevos.

### P1 — Necesario para producción usable

- Notificaciones reales y recordatorios.
- Outbox con reintentos.
- Reportes administrativos y operativos.
- Rate limiting distribuido.
- CORS/TLS/secretos.
- Métricas y alertas.
- Pruebas de carga.
- Documentación operativa.
- Migración y rollback ensayados.

### P2 — Fuera del primer cierre

- Historia clínica completa: diagnósticos codificados, recetas, órdenes, exámenes, adjuntos.
- Facturación, seguros y convenios.
- Farmacia e inventario.
- Hospitalización y cirugía.
- Telemedicina.
- Lista de espera y optimización de agenda.
- Multi-sede y multi-consultorio.
- Tutores/menores con consentimiento.
- Programa de fidelidad o analítica avanzada.
- Aplicación móvil.

Estos P2 no deben bloquear el flujo de cuenta, agenda y derivaciones. Si se intentan meter en la primera release, el backend no se cierra.

---

## 12. Riesgos y preguntas abiertas

| Riesgo / pregunta | Impacto | Mitigación |
|---|---|---|
| “Remitir a otro paciente” es ambiguo | Modelo incorrecto | Cerrado: derivación a especialidad/profesional + transferencia interna de médico tratante |
| Paciente ya existe sin cuenta | Duplicidad de historia | No crear otro; vincular con verificación/recepción |
| Documento duplicado en registro | Dos historias clínicas | Índice único y flujo de vinculación |
| Médico con permisos globales | Privacidad | Actor + ownership; retirar permisos globales |
| Paciente agenda especialista sin derivación | Se rompe la regla central | Validación transaccional + índice único |
| Dos requests usan la misma derivación | Doble reserva | `FOR UPDATE` + índice parcial |
| Horarios sin zona horaria | Slots incorrectos | Configurar `CLINIC_TIMEZONE`; tests |
| No-show | derivación consumida o reutilizable | Cerrado: consume y exige nueva valoración de Medicina General |
| Pediatría | Regla incómoda para menores | Cerrado: acceso directo para todos, sin regla por edad |
| Profesionales no médicos | Rol `medico` semánticamente incorrecto | Usar `tipo_profesional`; evaluar rol `profesional_salud` |
| Email no configurado | No hay verificación/reset | Adaptador de desarrollo y proveedor real en P1 |
| Migración sobre datos reales | Pérdida de datos | Backup, ensayo, migraciones aditivas |
| Frontend actual usa permisos globales | Ruptura al cambiar roles | Coordinar release; mantener compatibilidad temporal |
| Observabilidad local | No hay producción | Completar reemplazo de Promtail y recolección durable |
| Historia clínica completa | Scope creep | Mantener P2; nota mínima en P0 |

### Decisiones cerradas antes de la Fase 1

| # | Pregunta | Decisión |
|---|---|---|
| 1 | ¿La derivación es a una especialidad, a un médico específico o a ambos? | A la especialidad y, opcionalmente, a un médico específico mediante `medico_destino_id`. |
| 2 | ¿La derivación se puede usar una vez o varias veces dentro de su vigencia? | Una cita activa. Cancelar con 24 h o más la libera; reprogramar la conserva; atendida, vencida o consumida no se reutiliza. |
| 3 | ¿Qué pasa si el paciente no asiste al especialista? | La derivación se consume (`cancelada`, `motivo_cancelacion=no_show`) y requiere nueva valoración de Medicina General. |
| 4 | ¿Qué pasa si el paciente cancela con poca antelación? | Si cancela con menos de 24 h, la derivación se consume. El corte es configurable con `PORTAL_CANCEL_CUTOFF_HOURS`. |
| 5 | ¿Pediatría es acceso directo para menores? | Sí, Pediatría es acceso directo para todos, sin regla por edad (migración 014). |
| 6 | ¿Se exige email verificado para login o solo para agendar? | Antes del login y del refresh; no solo antes de agendar. |
| 7 | ¿Qué documento de identidad se usa y cómo se valida? | Documento oficial normalizado (cédula/pasaporte) con validación estructural básica; integración con registro civil queda para P2. |
| 8 | ¿Se permite que un médico sea también paciente? | El modelo lo permite con vínculos separados; no se implementa una experiencia dual en P0. |
| 9 | ¿Qué profesionales entran en `FISIO`, `ODONTO` y `NUTRI`? | Se registran como profesionales en `medicos` con su especialidad; `tipo_profesional` queda para P2. |
| 10 | ¿La clínica opera en una sola zona horaria y sede? | P0 asume una sede y `America/Guayaquil`; multi-sede queda para P2. |

---

## 13. Orden recomendado de PRs

Para reducir riesgo, dividir en PRs pequeños y verificables:

1. **PR 1:** migración 009 + entidades + stores de cuenta/catálogo + tests.
2. **PR 2:** migración 010 + horarios/bloqueos + tests.
3. **PR 3:** migración 011 + derivaciones/notas + tests.
4. **PR 4:** registro/verificación/reset + rol paciente + tests.
5. **PR 5:** actor/ownership + `/auth/me` + perfil + tests IDOR.
6. **PR 6:** catálogo y disponibilidad + endpoints administrativos.
7. **PR 7:** portal de citas directas + concurrencia.
8. **PR 8:** agenda médica + estados + nota.
9. **PR 9:** creación de derivaciones + validaciones.
10. **PR 10:** reserva de especialista + liberación + vencimiento + override.
11. **PR 11:** outbox/notificaciones.
12. **PR 12:** endurecimiento, OpenAPI, README y E2E final.

Cada PR debe incluir:

- migración (si aplica);
- entidades y DTOs;
- servicio y store;
- handler y ruta;
- permisos;
- auditoría;
- tests;
- actualización de OpenAPI/README.

---

## 14. Ruta mínima para una demo funcional

Si se necesita una demo rápida antes de completar P1, el orden mínimo es:

1. Fase 0: reglas confirmadas (completada).
2. Fase 1: migraciones 009–011.
3. Fase 2: registro/verificación con adaptador de email de archivo (completada).
4. Fase 3: actor/ownership (completada).
5. Fase 4: horarios y disponibilidad.
6. Fase 5: cita directa.
7. Fase 6: médico atiende y deriva.
8. Fase 7: paciente agenda especialista con derivación.

No se debe demo con datos sembrados que salten la regla de derivación: la demo debe probar el rechazo `409` y la autorización posterior.

---

## 15. Cierre

El backend actual es una buena base administrativa, pero el flujo solicitado requiere una capa de producto que hoy no existe: cuenta de paciente, disponibilidad real, acciones propias del médico y derivaciones.

La prioridad no es agregar pantallas ni endpoints aislados; es **cerrar primero el modelo y la autorización**, porque son los que evitan duplicar historias, filtrar datos entre pacientes/médicos y permitir que un paciente se salte la regla de Medicina General.

Si se sigue este orden, al final de la Fase 7 se tendrá el flujo completo solicitado; las fases 8–10 lo vuelven usable, seguro y desplegable.

---

## 16. Estado de implementación

### Paso 1 — Migración 009: cuentas, catálogo y permisos

**Estado:** completado.

**Archivo:** `migrations/009_portal_paciente.sql`.

Incluye:

- columnas de catálogo: `requiere_derivacion`, `activa`, `orden`, `duracion_cita_min`;
- especialidades nuevas: `FISIO`, `ODONTO`, `NUTRI` y `OTORRINO`;
- `MED-GEN` marcada como acceso directo;
- vínculos `pacientes.usuario_id` y `medicos.usuario_id`;
- campos de verificación y términos en `usuarios`;
- tabla `tokens_cuenta`;
- rol `paciente` y permisos de portal, agenda, notas, derivaciones y vinculación;
- permisos nuevos asignados a `paciente`, `medico`, `recepcion` y `admin`.

**Validación realizada:**

- migración aplicada desde una base vacía (001–009);
- migración 009 ejecutada dos veces para verificar idempotencia;
- migración aplicada sobre una copia de `medilist`;
- migración aplicada a `medilist`;
- `go build ./...`, `go vet ./...` y `go test -count=1 ./...` pasan;
- bases temporales eliminadas.

**Decisión de compatibilidad:** los permisos globales actuales de `medico` se conservan por ahora. El ownership y su retiro se harán cuando existan los endpoints de portal.

### Paso 2 — Entidades y stores de la migración 009

**Estado:** completado.

Incluye:

- `entities.Usuario`: `EmailVerificadoEn`, `TerminosAceptadosEn`, `VersionTerminos`, `DebeCambiarPassword` y helper `EmailVerificado()`;
- `entities.Paciente` y `entities.Medico`: vínculo `UsuarioID`;
- `entities.Medico`: `DuracionCitaMin`, `Bio`, `FotoURL` y `AceptaPortal`;
- `entities.Especialidad`: `RequiereDerivacion`, `Activa`, `Orden`, `DuracionCitaMin` y helper `AccesoDirecto()`;
- stores de usuarios, pacientes, médicos y especialidades leyendo/escribiendo los campos nuevos;
- métodos de cuenta `MarcarEmailVerificado`, `RegistrarAceptacionTerminos` y `EstablecerDebeCambiarPassword`;
- métodos de vinculación `PacienteStore.VincularUsuario` y `MedicoStore.VincularUsuario`;
- defaults seguros: especialidad nueva activa y con derivación obligatoria, duración de 30 minutos; médico nuevo con portal habilitado y duración de 30 minutos;
- las actualizaciones de pacientes/médicos preservan el vínculo de cuenta y el perfil público;
- pruebas de mapeo de los cuatro stores en `internal/repository/postgres/scan_portal_test.go`.

**Validación realizada:**

- `go build ./...`, `go vet ./...`, `gofmt -l .` y `go test -count=1 ./...` pasan;
- consultas reales de lectura contra `medilist` para usuarios, pacientes, médicos y especialidades;
- validación de escritura, lectura, defaults, actualización y vinculación en la base temporal `medilist_step2_check`, aplicando migraciones 001–009 desde cero;
- base temporal y artefactos de prueba eliminados.

No se modificaron endpoints ni DTOs de portal. Los flags de catálogo y el perfil público todavía se configuran por store; su exposición administrativa se hará junto con las rutas correspondientes.

### Paso 3 — Migración 010 de disponibilidad

**Estado:** completado.

**Archivo:** `migrations/010_disponibilidad.sql`.

Incluye:

- tabla `horarios_profesional` con día de semana, hora local, duración de slot, vigencia, estado y versión;
- tabla `bloqueos_agenda` con rangos UTC, motivo y usuario creador;
- función inmutable `hora_a_segundos(TIME)`;
- restricción `horarios_profesional_sin_solape` para horarios activos del mismo médico, día, hora y vigencia;
- restricción `bloqueos_agenda_medico_sin_solape` para bloqueos solapados del mismo médico;
- índices de consulta y triggers de `updated_at`;
- comentarios de tabla para documentar horas locales y bloqueos UTC.

**Pruebas de concurrencia:**

- `TestHorarioConcurrenteNoPermiteSolape`: dos transacciones compiten por horarios solapados; exactamente una confirma y la otra recibe `23P01`.
- `TestBloqueoConcurrenteNoPermiteSolape`: el mismo escenario para bloqueos puntuales.
- `TestHorariosCompatiblesSePermiten`: horarios adyacentes, de otro día, con vigencia disjunta o inactivos.
- `TestBloqueosCompatiblesSePermiten`: bloqueos adyacentes y de otro médico.
- Script repetible: `scripts/prueba_concurrencia_disponibilidad.ps1`.

**Validación realizada:**

- migración aplicada desde una base vacía (001–010);
- migración 010 ejecutada dos veces para verificar idempotencia;
- migración aplicada sobre una copia de `medilist`;
- migración aplicada a `medilist`;
- prueba de concurrencia ejecutada sobre base temporal creada desde cero;
- `gofmt -l .`, `go build ./...`, `go vet ./...` y `go test -count=1 ./...` pasan;
- bases temporales eliminadas.

No se crearon todavía entidades, stores, servicios ni endpoints de disponibilidad. La generación de slots y la validación de reservas corresponde a la Fase 4.

### Paso 4 — Migración 011 de derivaciones y metadata de citas

**Estado:** completado.

**Archivo:** `migrations/011_derivaciones_metadata.sql`.

Incluye:

- tabla `derivaciones` con paciente, médico y cita de origen, especialidad y médico destino, motivo, prioridad, estado, vigencia, versión y motivo de cancelación;
- índices por paciente/estado/vigencia, cita de origen, especialidad destino y médico destino;
- trigger `trg_derivaciones_consistencia` para validar el origen clínico y el destino;
- columnas nuevas en `citas`: `derivacion_id`, `canal`, `creada_por_usuario_id` y `motivo_cancelacion`;
- índice único parcial `citas_derivacion_activa_uk` para impedir dos citas activas o atendidas con la misma derivación;
- trigger `trg_citas_derivacion_consistencia` para validar paciente, especialidad, estado y vigencia al vincular una cita;
- tabla `notas_cita` con una nota por cita, versión optimista, `updated_at` e índice por médico;
- trigger `trg_notas_cita_medico` para que solo el médico de la cita escriba la nota;
- triggers de `updated_at` para `derivaciones` y `notas_cita`.

**Pruebas de integración:**

- `TestDerivacionValidaYRechazaOrigenInvalido`: una derivación válida se crea; se rechazan cita no atendida, paciente distinto, médico de origen distinto, destino sin derivación y médico destino de otra especialidad.
- `TestDerivacionUnaSolaCitaActiva`: la segunda cita activa falla con `23505`; cancelar la primera libera la derivación; una cita atendida sigue consumiéndola.
- `TestDerivacionCitaValidaPacienteEspecialidadYEstado`: se rechazan cita de otro paciente, médico de otra especialidad, derivación cancelada y derivación vencida.
- `TestDerivacionConcurrenteSoloPermiteUnaCita`: dos transacciones compiten por la misma derivación; una confirma y la otra recibe `23505`.
- `TestNotaCitaSoloMedicoDeLaCita`: se rechaza la nota de otro médico y la segunda nota de la misma cita.
- `TestCitasMetadataPorDefecto`: las citas nuevas quedan con `canal = recepcion`, sin actor, sin derivación y sin motivo de cancelación.
- Script repetible: `scripts/prueba_derivaciones.ps1`.

**Validación realizada:**

- migración aplicada desde una base vacía (001–011);
- migración 011 ejecutada dos veces para verificar idempotencia;
- migración aplicada sobre una copia de `medilist`;
- migración aplicada a `medilist`;
- pruebas de derivaciones, notas y metadata ejecutadas sobre base temporal creada desde cero, incluida la prueba de concurrencia;
- suite completa con `TEST_DATABASE_URL` y suite normal sin integración;
- `gofmt -l .`, `go build ./...`, `go vet ./...` y `go test -count=1 ./...` pasan;
- bases temporales eliminadas;
- `medilist` conserva 12 especialidades, 0 pacientes, 0 médicos, 0 citas, 0 derivaciones, 0 notas y solo el usuario `admin`.

No se crearon todavía entidades, stores, servicios ni endpoints de derivaciones o notas. Esa lógica corresponde a las fases de médico y reserva de especialista; la migración 011 solo refuerza el esquema y sus invariantes.

### Paso 5 — Fase 2: registro, verificación y cuentas

**Estado:** completado.

**Migración:** `migrations/012_cuentas_vinculacion.sql`.

Incluye:

- tabla `solicitudes_vinculacion` con estado `pendiente`, `aprobada` o `rechazada`, motivo, actor y fecha de resolución;
- índice único parcial para impedir dos solicitudes pendientes de la misma historia;
- índice único parcial para impedir dos solicitudes pendientes de la misma cuenta;
- trigger de `updated_at`.

**Capa de aplicación:**

- `entities.TokenCuenta`, `entities.SolicitudVinculacion` y filtros;
- puertos `CuentaStore` y `EmailSender`; búsquedas `PacienteStore.BuscarPorDocumento` y `UsuarioStore.BuscarPorEmail`;
- `CuentaStore` transaccional para alta, cuenta pendiente, consumo de tokens, reset, aprobación, rechazo y vínculo de médico;
- `CuentaService` con validación de username/email/documento/edad/términos y enlaces configurables;
- `DevEmailSender` con buzón local `.eml`, permisos `0700/0600` y sin tokens en logs;
- `CuentaHandler` y DTOs para registro, verificación, reenvío, recuperación, reset y vinculación;
- bloqueo de login y refresh para cuentas exclusivamente `paciente` sin email verificado (`403 email_no_verificado`);
- configuración `EMAIL_*`, `TERMS_VERSION` y `MIN_EDAD_CUENTA`; `.env.example` y `.gitignore` actualizados.

**Rutas registradas:**

- `POST /api/v1/auth/register`
- `POST /api/v1/auth/verify-email`
- `POST /api/v1/auth/resend-verification`
- `POST /api/v1/auth/forgot-password`
- `POST /api/v1/auth/reset-password`
- `GET /api/v1/cuentas/vinculaciones`
- `POST /api/v1/cuentas/vinculaciones/{id}/aprobar`
- `POST /api/v1/cuentas/vinculaciones/{id}/rechazar`
- `POST /api/v1/cuentas/vincular-paciente`
- `POST /api/v1/cuentas/vincular-medico`

**Pruebas automatizadas:**

- `TestCuentaStoreRegistroVerificaTokenUnSoloUso`: alta transaccional y segundo uso rechazado.
- `TestCuentaStoreDocumentoDuplicadoNoDuplicaPaciente`: el documento conserva una persona y un paciente.
- `TestCuentaStoreRegistroConcurrenteMismoDocumento`: dos registros compiten; exactamente uno crea y el otro recibe `409`.
- `TestCuentaStoreTokenVencidoYReutilizado`: token vencido no verifica; token nuevo funciona una sola vez.
- `TestCuentaStoreResetPasswordRevocaSesiones`: cambia hash, verifica email, incrementa `token_version` y revoca refresh tokens.
- `TestCuentaStoreSolicitudVinculacionAprobarYListar`: no duplica la historia y la aprueba.
- `TestCuentaStoreRechazarSolicitudDesactivaCuenta`: rechazo con desactivación.
- `TestCuentaStoreVincularMedico`: vínculo de cuenta con perfil médico.
- Tests de servicio para validaciones, correo, reenvío genérico, reset y auditoría, incluidos tokens inválidos auditados; test de login sin email verificado y de contraseña incorrecta que no revela el estado.
- Script repetible: `scripts/prueba_cuentas.ps1`.

**Validación realizada:**

- migración 012 aplicada a `medilist` y a una base temporal desde cero;
- migración 012 ejecutada dos veces sobre `medilist` para verificar idempotencia, con índices y trigger presentes;
- pruebas de cuentas y suite completa con `TEST_DATABASE_URL`;
- `gofmt -l .`, `go build ./...`, `go vet ./...` y `go test -count=1 ./...` pasan;
- flujo HTTP real: registro, verificación, login bloqueado antes de verificar, reenvío, recuperación, reset, rechazo de reutilización de token y vinculación de un paciente histórico sin duplicarlo;
- base de prueba temporal eliminada y datos HTTP de prueba eliminados;
- `medilist` queda con 1 usuario (`admin`), 0 pacientes, 0 personas, 0 solicitudes y 0 tokens de cuenta.

**Siguiente:** Fase 3 completada (ver Paso 6).

### Paso 6 — Fase 3: Actor, ownership y portal básico

**Estado:** completado.

**Migración:** `migrations/013_ownership_medico.sql`.

Incluye:

- retiro de `pacientes:leer`, `citas:leer` y `citas:escribir` del rol `medico`;
- reafirmación idempotente de `medico:agenda:*`, `medico:nota:*`, `pacientes:atendidos:leer` y `derivaciones:*`;
- nota de compatibilidad: la guarda por roles de la API bloquea tokens antiguos que todavía conserven permisos globales.

**Capa de aplicación:**

- `service.Actor` con `UsuarioID`, `Username`, `Email`, `Roles`, `Permisos`, `PacienteID`, `MedicoID` y `EmailVerificado`;
- `entities.VinculosUsuario` y puerto `ActorStore`;
- `ActorStore` en PostgreSQL resuelve `pacientes.usuario_id` y `medicos.usuario_id` solo para perfiles activos;
- el middleware construye el Actor una vez por request y lo guarda en el contexto;
- `GET /auth/me` devuelve `paciente_id` y `medico_id` cuando existen;
- `PortalPacienteService` obtiene y actualiza únicamente el perfil vinculado al Actor; el update solo toca teléfono, dirección y contacto de emergencia, con control de `version`;
- `MedicoPortalService` fuerza `MedicoID` en la agenda, comprueba propiedad en el detalle de cita y exige una cita `programada`, `confirmada` o `atendida` para consultar un paciente;
- guarda `soloPersonalAdministrativo` para `/pacientes` y `/citas`: aunque un token antiguo conserve los permisos retirados, un médico no puede usar los endpoints globales;
- auditoría de lecturas propias y administrativas, con sufijo `_denegado` cuando el handler rechaza por ownership;
- rutas nuevas: `GET/PUT /portal/perfil`, `GET /medico/agenda`, `GET /medico/citas/{id}` y `GET /medico/pacientes/{id}`.

**Pruebas automatizadas:**

- `TestActorStoreResuelveVinculosActivosYOwnership`: vínculos activos, paciente inactivo y relación de atención.
- `TestMigracionRolMedicoSinPermisosGlobales`: el rol `medico` no conserva permisos globales y sí los propios.
- `TestActorPermisosYAlcanceGlobal`, `TestActorExigeVinculosClinicos`.
- `TestPortalPacientePerfilExigeVinculo`, `TestPortalPacientePerfilNoDevuelvePerfilDeOtroUsuario`, `TestPortalPacienteActualizarContactoConservaIdentidad`, `TestPortalPacienteActualizarContactoRechazaVersionVieja`.
- `TestMedicoPortalAgendaFuerzaMedicoDelActor`, `TestMedicoPortalNoDevuelveCitaDeOtroMedico`, `TestMedicoPortalPacienteSinRelacionNoSeConsulta`.
- `TestSoloPersonalAdministrativoBloqueaMedico`, `TestAuditDataAccessMarcaIntentoDenegado`.
- Script repetible: `scripts/prueba_ownership.ps1`.

**Validación realizada:**

- migración 013 aplicada a `medilist` y a una base temporal desde cero;
- `scripts/prueba_ownership.ps1` pasa sobre la base temporal;
- `go test ./...` pasa sin `TEST_DATABASE_URL`;
- flujo HTTP real sobre base temporal: login de paciente/médico, `paciente_id`/`medico_id` en `/auth/me`, perfil propio, rechazo de `paciente_id` en el body, actualización de contacto, agenda propia, `403` contra cita/paciente ajenos, acceso global del administrador y auditoría de accesos propios/denegados;
- el rol `medico` de `medilist` queda solo con permisos propios y de catálogo;
- `derivaciones` y `notas_cita` todavía no tienen endpoints; la misma regla de ownership se aplicará al implementar su API en la Fase 6.

### Paso 7 — Cierre de Fase 0: decisiones y contrato

**Estado:** completado.

Decisiones cerradas:

- “Remitir” incluye derivación clínica a otra especialidad/profesional y transferencia interna de médico tratante; no hay transferencia externa.
- Pediatría es de acceso directo para todos, sin regla por edad; migración 014.
- Derivación: 90 días configurable, una cita activa, cancelación con 24 h o más la libera, cancelación tardía o no-show la consume, reprogramación la conserva si sigue vigente y es la misma especialidad/médico.
- Excepción de emergencia: solo `admin` con `derivaciones:sobrescribir`, motivo y auditoría.
- Email verificado antes del login y del refresh; autorregistro solo para adultos.
- Agenda: `America/Guayaquil`, slots de 30 min configurables, antelación de 2 h y horizonte de 90 días.
- Transferencia interna en Fase 6, con `pacientes:transferir`, motivo y auditoría.

Artefactos:

- `migrations/014_pediatria_acceso_directo.sql` aplicada a `medilist` y a una base temporal desde cero; segunda ejecución idempotente (`UPDATE 0`).
- `internal/repository/postgres/catalogo_migracion_test.go` con `TestMigracionPediatriaAccesoDirecto`.
- `docs/openapi.yaml` con los endpoints implementados y `x-pendientes` para las fases 5–8; validado con Redocly.
- `README.md`, `doc.md` y `entidades.md` actualizados.

Validación:

- `gofmt -l .` sin salida.
- `go build ./...`, `go vet ./...` y `go test -count=1 ./...` pasan.
- `TestMigracionPediatriaAccesoDirecto` pasa sobre base temporal.
- La base `medilist` conserva solo el usuario `admin` y el catálogo actualizado; no se agregaron datos clínicos.

### Paso 8 — Fase 4: catálogo, horarios y disponibilidad

**Estado:** completado.

**Archivos principales:**

- `internal/service/entities/agenda.go`: `HorarioProfesional`, `BloqueoAgenda`, `IntervaloOcupado`, `SlotDisponible` y `Disponibilidad`.
- `internal/service/agenda_service.go`: CRUD, validación de rangos, solapes y versiones.
- `internal/service/disponibilidad_service.go`: generación de slots, zona horaria, antelación, horizonte, bloqueos y citas activas.
- `internal/service/catalogo_service.go`: catálogo público de especialidades y médicos.
- `internal/repository/postgres/agenda_store.go`: persistencia y consultas de agenda.
- `internal/repository/postgres/catalogo_store.go`: lecturas públicas del portal.
- `internal/transport/http/agenda_handler.go`, `catalogo_handler.go`, `agenda_dto.go`: endpoints y DTOs.
- `internal/config/config.go`: `CLINIC_TIMEZONE`, `PORTAL_MIN_NOTICE_HOURS`, `PORTAL_MAX_ADVANCE_DAYS` y carga embebida de zonas IANA.
- `docs/openapi.yaml`: contrato de los endpoints de la Fase 4.

**Decisiones de implementación:**

- Los horarios se guardan como hora local `HH:MM`; la conversión a UTC se hace al generar cada slot.
- Los bloqueos se guardan en UTC y se comparan como intervalos `[inicio, fin)`.
- Un slot se descarta si empieza antes de `now + PORTAL_MIN_NOTICE_HOURS`, si cae fuera del horizonte, si se cruza con un bloqueo o con una cita `programada`/`confirmada`.
- Las fechas `desde`/`hasta` son inclusivas; el rango máximo es el horizonte configurado.
- El portal solo muestra especialidades activas y médicos activos que aceptan portal.
- El perfil público no expone documento, email, teléfono ni dirección del médico.
- El CRUD de horarios/bloqueos exige `agenda:gestionar` y la guarda administrativa; las lecturas se auditan y las escrituras pasan por `auditMutations`.
- `DELETE /medicos/{id}/horarios/{horarioId}` es baja lógica para conservar citas históricas.
- `ValidarSlot` reutiliza `Consultar` para que la reserva de la Fase 5 solo acepte inicios alineados a un slot generado, con la duración del horario y sin bloqueos/citas activas.

**Pruebas añadidas:**

- `TestDisponibilidadConvierteHoraLocalAUTC`
- `TestDisponibilidadRestaBloqueosYCitasActivas`
- `TestDisponibilidadRespetaAntelacionMinima`
- `TestDisponibilidadRechazaRangoFueraDelHorizonte`
- `TestDisponibilidadRespetaVigenciaDelHorario`
- `TestValidarSlotExigeUnSlotGenerado`
- `TestDisponibilidadNoExponeMedicoInactivoODePortalCerrado`
- `TestAgendaCrearHorarioValido`
- `TestAgendaCrearHorarioRechazaSolape`
- `TestAgendaCrearHorarioRechazaRangoInvalido`
- `TestAgendaActualizarHorarioExigeVersionVigente`
- `TestAgendaCrearBloqueoValidaMotivoYSolape`
- `TestAgendaActualizarBloqueoValidaSolape`
- `TestAgendaStoreCRUDYConsultasDeDisponibilidad` (integración PostgreSQL)
- `TestValidateZonaHorariaYHorizonteDelPortal`

**Validación realizada:**

- `gofmt -l .` sin salida.
- `go build ./...`, `go vet ./...` y `go test -count=1 ./...` pasan.
- `TestAgendaStoreCRUDYConsultasDeDisponibilidad` pasa con `TEST_DATABASE_URL` sobre `medilist`.
- `docs/openapi.yaml` validado con Redocly sin errores de contrato (solo advertencias de estilo).
- E2E sobre una base temporal migrada desde cero: se creó un médico y un horario, `/portal/disponibilidad` devolvió 2 slots en UTC, un bloqueo dejó 1, un solape de horario y un `version` obsoleto devolvieron `409`, la actualización correcta devolvió `200`, y al eliminar bloqueo/horario la disponibilidad volvió a 4/0 slots. La base temporal se eliminó al finalizar.

**Pendiente inmediato:** Fase 6 (flujo médico, derivaciones y transferencia interna).

### Paso 9 — Fase 5: agenda directa del paciente

**Estado:** completado.

**Archivos principales:**

- `internal/service/portal_cita_service.go`: reserva, listado, detalle, reprogramación y cancelación con ownership por actor.
- `internal/repository/postgres/cita_store.go`: persistencia de `canal`, `creada_por_usuario_id`, `derivacion_id`, `motivo_cancelacion` y cancelación optimista.
- `internal/service/entities/cita.go`: campos de metadata y tipo `CanalCita`.
- `internal/transport/http/portal_cita_handler.go`, `portal_dto.go`: endpoints y DTOs del portal.
- `internal/config/config.go`: `PORTAL_CANCEL_CUTOFF_HOURS` y `AUTO_CONFIRM_PORTAL_BOOKINGS`.
- `internal/notificaciones/email_dev.go`: confirmación básica de cita en el buzón local.
- `docs/openapi.yaml`: contrato de `/portal/citas`.

No requiere migración nueva: reutiliza las columnas `canal`, `creada_por_usuario_id`, `derivacion_id` y `motivo_cancelacion` de la migración 011.

**Reglas implementadas:**

- El `paciente_id` se obtiene exclusivamente de `Actor.PacienteVinculado()` y se verifica contra `pacientes.usuario_id`.
- `canal` se fuerza a `portal_paciente` y `creada_por_usuario_id` se toma del actor.
- Solo se aceptan especialidades con `requiere_derivacion=false`; las demás devuelven `409 requiere_derivacion`.
- El médico debe estar activo, aceptar portal y tener una especialidad activa.
- La reserva usa `DisponibilidadService.ValidarSlot`; la duración sale del slot, no del body.
- El estado inicial es `confirmada` si `AUTO_CONFIRM_PORTAL_BOOKINGS=true`; si no, `programada`.
- La reprogramación exige `version`, conserva paciente/médico/especialidad y vuelve a validar el slot.
- La cancelación exige `PORTAL_CANCEL_CUTOFF_HOURS` de antelación, registra motivo y es idempotente si la cita ya está cancelada.
- Las citas terminales (`atendida`, `no_asistio`, `cancelada`, `reprogramada`) no se reprograman.
- Las restricciones de exclusión de PostgreSQL resuelven la carrera de dos pacientes por el mismo slot.
- La confirmación se escribe en el buzón de desarrollo mediante `EmailSender.EnviarCitaCreada`; un fallo de correo no revierte la cita.

**Pruebas añadidas:**

- `TestPortalCitaCrearFuerzaPacienteCanalYDuracion`
- `TestPortalCitaCrearEscribeConfirmacionBasica`
- `TestPortalCitaCrearRechazaEspecialidadConDerivacion`
- `TestPortalCitaCrearRechazaMedicoSinPortal`
- `TestPortalCitaListarFuerzaPacienteDelActor`
- `TestPortalCitaObtenerRechazaCitaAjena`
- `TestPortalCitaReprogramarExigeVersionYConservaMedico`
- `TestPortalCitaCancelarAntesDelCorte`
- `TestPortalCitaCancelarDespuesDelCorte`
- `TestCitaStorePersisteMetadataDePortalYCancela` (integración PostgreSQL)
- `TestCitaStoreConcurrenteSoloPermiteUnSlot` (integración PostgreSQL)

**Validación realizada:**

- `gofmt -l .` sin salida.
- `go build ./...`, `go vet ./...` y `go test -count=1 ./...` pasan.
- `docs/openapi.yaml` validado con Redocly sin errores de contrato.
- E2E sobre base temporal desde cero: paciente verificado agenda un slot de Medicina General (`201`, `confirmada`, canal `portal_paciente`), lista su cita, la reprograma (`200`, misma paciente/médico) y la cancela (`200`, motivo registrado); Cardiología devuelve `409 requiere_derivacion`. La base temporal se eliminó al finalizar.

**Pendiente inmediato:** Fase 6 (flujo médico, derivaciones y transferencia interna).




