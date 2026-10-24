# MediList — Plan para concluir el backend

> Fecha: 23/10/2026  
> Alcance: backend Go/PostgreSQL. No incluye frontend.  
> Estado: propuesta de trabajo para revisión.  
> Documentos relacionados: `README.md` (estado actual) y `entidades.md` (modelo completo objetivo).

---

## 0. Resumen ejecutivo

La base actual ya tiene una parte importante del backend administrativo:

- autenticación con access/refresh tokens;
- RBAC con roles y permisos;
- CRUD de pacientes, médicos, especialidades y citas;
- control de concurrencia con restricciones de exclusión y versiones optimistas;
- auditoría, logs estructurados y retención.

Sin embargo, **el flujo de paciente con cuenta propia, agenda de disponibilidad, portal del paciente, derivaciones y acciones propias del médico todavía no está implementado**. Lo que existe hoy es un CRUD administrativo; no existe una cuenta de paciente vinculada a su historia, ni una regla que impida a un paciente agendar directamente a un especialista.

El orden recomendado para concluir el backend es:

1. Cerrar las decisiones de negocio y el contrato de API.
2. Crear el modelo de datos y las migraciones.
3. Implementar registro, verificación de cuenta y vinculación paciente/usuario.
4. Implementar autorización por fila: el paciente solo ve lo suyo y el médico solo ve su agenda.
5. Crear catálogo, horarios y disponibilidad.
6. Permitir que el paciente agende las cuatro áreas de acceso directo.
7. Implementar el flujo médico mínimo y las derivaciones.
8. Permitir que el paciente use una derivación para agendar al especialista.
9. Añadir notificaciones, pruebas, endurecimiento y despliegue controlado.

La regla central solicitada queda así:

> El paciente puede agendar directamente **Medicina General, Fisioterapia, Odontología y Nutrición**. Para cualquier otra especialidad necesita una **derivación vigente emitida por un médico de Medicina General después de una cita atendida**. Sin derivación, el backend rechaza la reserva del especialista.

Esta regla debe vivir en el servicio y en la base de datos, no solo en el frontend.

### Aclaración importante sobre “remitirlo a otro paciente”

En el texto se indica que “el doctor puede remitirlo a otro paciente”. Eso no es un flujo clínico válido: un médico no remite un paciente a otro paciente. En este documento se interpreta como:

> **El médico remite al paciente a otro profesional o a otra especialidad.**

Si lo que se necesita es representar un vínculo familiar o de tutor (por ejemplo, un paciente menor de edad o un cuidador), eso es otra entidad y se trata en la fase de tutores/representantes, no en la de derivaciones.

---

## 1. Supuestos y decisiones que deben confirmarse

Estas decisiones deben cerrarse antes de escribir migraciones, porque cambian el modelo y los endpoints.

| # | Tema | Propuesta inicial | Estado |
|---|---|---|---|
| 1 | Significado de “remitir a otro paciente” | Remitir a otro profesional/especialidad | Confirmar |
| 2 | Especialidades de acceso directo | `MED-GEN`, `FISIO`, `ODONTO`, `NUTRI` | Confirmar |
| 3 | Especialidades que exigen derivación | Todas las demás, incluida `OTORRINO` | Confirmar |
| 4 | Quién emite la derivación que habilita al especialista | Solo médico de Medicina General, desde una cita `atendida` | Confirmar |
| 5 | Vigencia de la derivación | 90 días por defecto, configurable | Confirmar |
| 6 | Uso de la derivación | Una derivación habilita una cita activa; si se cancela antes de la atención y sigue vigente, vuelve a `pendiente` | Confirmar |
| 7 | No asistencia del paciente a la cita de especialista | Consume la derivación; se requiere nueva valoración de Medicina General | Confirmar |
| 8 | Excepción de emergencia | Recepción/admin puede sobrescribir con permiso, motivo y auditoría | Confirmar |
| 9 | Registro de paciente | Requiere verificación de email antes de agendar | Confirmar |
| 10 | Paciente ya existente | No duplicar historia; vincular cuenta mediante recepción o verificación | Confirmar |
| 11 | Cuenta de médico | La crea un administrador; el médico no se autorregistra | Confirmar |
| 12 | Pediatría | Por defecto exige derivación, salvo que se configure como excepción para menores | Confirmar |
| 13 | Menores de edad | Requieren tutor/representante en una fase posterior | Confirmar |
| 14 | Zona horaria | Guardar en UTC; configurar zona de la clínica para slots y reportes | Confirmar |
| 15 | Alcance clínico | Este plan cubre agenda, cuenta, derivaciones y nota mínima; historia clínica completa queda para P2 | Confirmar |

### Decisión recomendada sobre “pediatría”

La regla solicitada dice que solo las cuatro áreas son de acceso directo. Pediatría es una especialidad; por tanto, con la regla literal, también requeriría derivación. Sin embargo, en la práctica un niño suele necesitar pediatra sin pasar por medicina general. Si se quiere una excepción, debe quedar configurada en el catálogo (`requiere_derivacion = false` para pediatría) o implementarse una regla por edad. No conviene esconder una excepción en el código.

---

## 2. Estado actual real

### 2.1 Lo que ya existe

- **Autenticación:** login, refresh token rotativo, logout, bloqueo por intentos fallidos, revocación inmediata por `token_version`.
- **RBAC:** roles `admin`, `medico`, `recepcion`, `enfermeria`, `auditor`; permisos granulares.
- **Usuarios:** CRUD administrativo, asignación de roles, desbloqueo y revocación de sesiones.
- **Pacientes:** CRUD con `persona`, historia clínica administrativa, datos de contacto y versión optimista.
- **Médicos:** CRUD con `persona`, especialidad, licencia y estado.
- **Especialidades:** catálogo básico con 8 registros.
- **Citas:** creación, consulta, actualización, cambio de estado, filtros y versión optimista.
- **Concurrencia:** restricciones de exclusión para solapes de médico y paciente en citas `programada`/`confirmada`; detección de conflictos `23P01`, `40001`, `40P01`, `55P03`.
- **Auditoría:** bitácora de acciones y consultas.
- **Observabilidad:** logs JSON, `trace_id`, `X-Request-ID`, errores en base, retención y stack de Loki/Grafana local.

### 2.2 Lo que falta para el flujo solicitado

- **Registro público de pacientes:** no existe `POST /auth/register`.
- **Rol `paciente`:** no existe.
- **Vínculo cuenta–paciente:** `usuarios` y `pacientes` están separados; no hay `pacientes.usuario_id`.
- **Vínculo cuenta–médico:** `usuarios` y `medicos` están separados; no hay `medicos.usuario_id`.
- **Portal del paciente:** no existen endpoints para perfil propio, catálogo, disponibilidad, citas propias ni derivaciones.
- **Portal del médico:** no existen endpoints de “mi agenda”, “mi cita”, “marcar atendida” o “emitir derivación”.
- **Disponibilidad:** no existen horarios de profesionales ni bloqueos de agenda; hoy se puede reservar cualquier hora si no hay solape.
- **Especialidades solicitadas:** no existen `FISIO`, `ODONTO`, `NUTRI` ni `OTORRINO`.
- **Derivaciones:** no existe tabla, estados, regla de acceso ni vínculo con la cita.
- **Autorización por fila:** un médico con `citas:leer` puede consultar todas las citas; un paciente no existe, pero el patrón actual no está preparado para “solo lo mío”.
- **Notificaciones:** no hay verificación de email, recuperación de contraseña, recordatorios ni outbox.
- **Nota/atención clínica mínima:** una cita solo cambia de estado; no hay registro de la consulta que justifique una derivación.
- **Historia clínica completa:** diagnósticos, recetas, órdenes, exámenes y adjuntos no están implementados.

### 2.3 Conclusión de estado

La base de agenda y seguridad es aprovechable, pero **el producto “paciente con cuenta que agenda y médico que deriva” no está terminado**. Hay que construirlo sobre lo existente, sin romper la arquitectura por capas ni las garantías de concurrencia.

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
| Pediatría | `PED` | Con derivación | Confirmar excepción para menores |
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

Estados propuestos:

- `pendiente`: creada por el médico, aún no usada.
- `agendada`: vinculada a una cita activa de especialista.
- `atendida`: el especialista atendió la cita.
- `vencida`: pasó `vigente_hasta` sin completarse.
- `cancelada`: anulada por el médico o administración.

Transiciones:

| Estado actual | Evento | Estado nuevo |
|---|---|---|
| `pendiente` | Se agenda una cita de especialista | `agendada` |
| `pendiente` | Vence `vigente_hasta` | `vencida` |
| `pendiente` | El médico de origen o un admin la cancela | `cancelada` |
| `agendada` | El especialista marca la cita como `atendida` | `atendida` |
| `agendada` | El paciente cancela antes del corte y sigue vigente | `pendiente` |
| `agendada` | El paciente cancela después del corte o no asiste | `vencida` o `cancelada` según la política confirmada |
| `agendada` | Vence `vigente_hasta` | `vencida` |
| `agendada` | El médico de origen o un admin la cancela | `cancelada` |
| `atendida`, `vencida`, `cancelada` | — | Estado terminal, no reutilizable |

Reglas de transición:

- Solo se crea desde una cita de Medicina General `atendida`.
- `pendiente -> agendada` ocurre en la misma transacción que crea la cita del especialista.
- `agendada -> atendida` cuando el especialista marca la cita como `atendida`.
- `agendada -> pendiente` si el paciente cancela antes del límite y la derivación sigue vigente.
- `pendiente/agendada -> vencida` por paso del tiempo; se puede calcular en consulta o con un job.
- `pendiente/agendada -> cancelada` solo por médico de origen o admin, con motivo.
- Una derivación `atendida`, `vencida` o `cancelada` no puede reutilizarse.
- El no-show del paciente a la cita de especialista consume la derivación (decisión recomendada) o requiere una regla explícita distinta.

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
- Puede agendar directamente las cuatro especialidades de acceso directo.
- No puede agendar un especialista sin derivación.
- Puede cancelar su cita antes del límite configurado.
- Puede reprogramar su cita; la operación debe usar `version` para evitar pérdidas concurrentes.
- Si cancela una cita de especialista vinculada a una derivación, la derivación vuelve a `pendiente` si sigue vigente.
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
- El médico no puede derivar a otro paciente; deriva a otra especialidad o profesional.

### 3.7 Reglas de recepción y administración

- Recepción puede crear y confirmar citas para cualquier paciente, como hoy.
- Recepción puede gestionar horarios y bloqueos si tiene `agenda:gestionar`.
- Recepción puede vincular una cuenta a un paciente existente.
- Recepción no puede emitir derivaciones clínicas.
- Admin puede sobrescribir la regla de derivación con `derivaciones:sobrescribir`, indicando motivo; toda sobrescritura queda auditada.
- La sobrescritura debe ser excepcional (urgencia, error operativo), no el flujo normal.

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
| `recepcion` | `agenda:gestionar`, `pacientes:leer`, `citas:leer`, `citas:escribir`, `cuentas:vincular` |
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

No hardcodear códigos de especialidad en el código. La lista directa se controla con `especialidades.requiere_derivacion`.

---

## 5. Modelo de datos y migraciones

Numeración tentativa, continua desde la última migración aplicada (`008_consistencia_concurrencia.sql`). Cada migración debe ser idempotente en lo posible, probarse desde cero y probarse sobre una copia de la base actual.

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

### 5.4 Migración 012 — notificaciones y outbox

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

- El registro de la cita y la notificación se hacen en la misma transacción (outbox).
- Un worker separado envía y reintenta.
- Nunca se guardan tokens de verificación/reset en texto plano ni en logs.
- Para desarrollo se puede usar un adaptador que escribe en consola; para producción, SMTP/SES/SendGrid.

### 5.5 Backfill y compatibilidad

- No eliminar columnas existentes.
- No cambiar el significado de `citas.estado` existente.
- Las citas históricas no tienen derivación; se quedan con `derivacion_id = NULL`.
- Los pacientes históricos no tienen cuenta; se vinculan cuando lo soliciten.
- Los médicos históricos no tienen usuario; administración los vincula.
- Los permisos globales actuales se mantienen temporalmente y se aplica scoping por fila; luego se retiran en una migración de limpieza.
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
| `POST` | `/auth/login` | Público | Ya existe; validar email verificado si se decide |
| `POST` | `/auth/refresh` | Público | Ya existe; rotación atómica |
| `POST` | `/auth/logout` | Autenticado | Ya existe |
| `GET` | `/auth/me` | Autenticado | Extender con `paciente_id`, `medico_id`, `email_verificado`, roles |

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
    "email": "juan.perez@example.com",
    "email_verificado": false,
    "mensaje": "Revisa tu correo para verificar la cuenta"
  }
}
```

Decisiones:

- No devolver tokens hasta verificar el email, salvo que se implemente un modo “cuenta limitada”.
- Si el email ya existe, responder de forma genérica para no filtrar cuentas.
- Si el documento ya existe como paciente, **no crear otro paciente**. Responder `409` con instrucción de vinculación o crear una solicitud de vinculación.
- Si el documento no existe, crear `persona`, `paciente`, `usuario` y rol `paciente` en una sola transacción.
- Guardar aceptación de términos y versión.
- Auditar `registro`, `verificacion_email`, `reset_password`.

### 6.3 Portal del paciente

| Método | Ruta | Permiso | Descripción |
|---|---|---|---|
| `GET` | `/portal/perfil` | `portal:perfil:leer` | Datos propios y estado de cuenta |
| `PUT` | `/portal/perfil` | `portal:perfil:escribir` | Actualiza contacto; no documento ni datos clínicos |
| `GET` | `/portal/especialidades` | `catalogo:leer` | Catálogo visible, con `requiere_derivacion` |
| `GET` | `/portal/medicos` | `catalogo:leer` | Médicos activos; filtro por especialidad |
| `GET` | `/portal/disponibilidad` | `portal:citas:leer` | Slots disponibles por médico/rango |
| `POST` | `/portal/citas` | `portal:citas:escribir` | Agenda cita propia |
| `GET` | `/portal/citas` | `portal:citas:leer` | Lista solo citas propias |
| `GET` | `/portal/citas/{id}` | `portal:citas:leer` | Detalle solo si es propia |
| `PUT` | `/portal/citas/{id}` | `portal:citas:escribir` | Reprograma con `version` |
| `POST` | `/portal/citas/{id}/cancelar` | `portal:citas:escribir` | Cancela si cumple política |
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
| `GET` | `/medico/agenda` | `medico:agenda:leer` | Citas propias por rango/estado |
| `GET` | `/medico/citas/{id}` | `medico:agenda:leer` | Detalle solo si es su cita |
| `PATCH` | `/medico/citas/{id}/estado` | `medico:agenda:escribir` | Confirma, atiende o marca no asistió |
| `POST` | `/medico/citas/{id}/nota` | `medico:nota:escribir` | Nota mínima de consulta |
| `PUT` | `/medico/citas/{id}/nota` | `medico:nota:escribir` | Actualiza nota con `version` |
| `POST` | `/medico/citas/{id}/derivaciones` | `derivaciones:escribir` | Emite derivación desde cita atendida |
| `GET` | `/medico/derivaciones` | `derivaciones:leer` | Derivaciones emitidas por el médico |
| `GET` | `/medico/pacientes/{id}` | `pacientes:atendidos:leer` | Paciente con relación de atención |

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
| `POST` | `/medicos/{id}/cuenta` | `usuarios:escribir` | Crea/vincula cuenta de médico |
| `GET` | `/horarios` | `agenda:gestionar` | Lista horarios |
| `POST` | `/horarios` | `agenda:gestionar` | Crea horario |
| `PUT` | `/horarios/{id}` | `agenda:gestionar` | Actualiza con `version` |
| `DELETE` | `/horarios/{id}` | `agenda:gestionar` | Desactiva horario |
| `GET` | `/bloqueos` | `agenda:gestionar` | Lista bloqueos |
| `POST` | `/bloqueos` | `agenda:gestionar` | Crea bloqueo |
| `DELETE` | `/bloqueos/{id}` | `agenda:gestionar` | Elimina/cancela bloqueo |
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

Crear `docs/openapi.yaml` (o `api/openapi.yaml`) antes de implementar el frontend. Debe incluir:

- esquemas de autenticación;
- endpoints de portal y médico;
- códigos de error de negocio;
- ejemplos de cita directa y con derivación;
- estados y transiciones;
- parámetros de paginación y filtros;
- ejemplos de `409`.

La documentación debe mantenerse sincronizada con los handlers; no dejarla como archivo abandonado.

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

**Objetivo:** evitar retrabajo en migraciones y endpoints.

Tareas:

- Confirmar la interpretación de “remitir a otro paciente”.
- Confirmar las cuatro especialidades de acceso directo.
- Confirmar si Pediatría es excepción para menores.
- Definir vigencia de derivación, política de no-show, cancelación y reprogramación.
- Definir si el registro exige verificación de email antes de login o solo antes de agendar.
- Definir el flujo para pacientes que ya existen sin cuenta.
- Definir permisos finales de `paciente`, `medico`, `recepcion` y `admin`.
- Definir zona horaria, duración de slots y antelación mínima.
- Crear el esqueleto de `docs/openapi.yaml`.
- Definir códigos de error y formato de respuesta.
- Definir feature flags y estrategia de migración.

**Criterio de salida:** decisiones registradas y contrato de API revisado. No se escribe código de negocio todavía.

### Fase 1 — Modelo de datos y migraciones

**Objetivo:** dejar la base preparada sin romper lo existente.

Tareas:

- Migración `009`: cuentas, vínculos, rol paciente, permisos, tokens, términos, catálogo.
- Migración `010`: horarios y bloqueos.
- Migración `011`: derivaciones, metadata de citas y nota mínima.
- Migración `012` (P1): outbox de notificaciones.
- Crear entidades: `Derivacion`, `HorarioProfesional`, `BloqueoAgenda`, `NotaCita`, `TokenCuenta`, `Notificacion`.
- Crear stores PostgreSQL correspondientes.
- Actualizar stores existentes para `usuario_id`, `canal`, `derivacion_id`.
- Agregar índices y restricciones.
- Probar migraciones desde cero y sobre copia de la base actual.
- Documentar el orden y el rollback.

**Criterio de salida:** migraciones aplican sin pérdida de datos; tests de repositorio pasan; la base actual sigue funcionando.

### Fase 2 — Registro, verificación y cuentas

**Objetivo:** que un paciente pueda crear una cuenta real y quedar vinculado a su perfil.

Tareas:

- `POST /auth/register` transaccional.
- Validación de username, email, documento, edad y términos.
- Creación de `persona`, `paciente`, `usuario`, rol `paciente`.
- Token de verificación de email con hash, expiración y un solo uso.
- `POST /auth/verify-email` y reenvío.
- `POST /auth/forgot-password` y `POST /auth/reset-password`.
- Revocación de sesiones al cambiar contraseña.
- Adaptador de email de desarrollo (consola/archivo) y interfaz para proveedor real.
- Rate limiting de registro, login y recuperación.
- Auditoría de registro, verificación y reset.
- Flujo para documento ya existente: no duplicar; vinculación por recepción o solicitud verificable.
- Creación/vinculación de cuenta de médico por administración.

**Criterio de salida:** un paciente se registra, verifica, inicia sesión y su usuario queda ligado a un único paciente. Un documento duplicado no crea otra historia. Los tests cubren tokens vencidos, reutilizados y carreras de registro.

### Fase 3 — Actor, ownership y permisos por fila

**Objetivo:** cerrar el riesgo de que un paciente o médico vea datos ajenos.

Tareas:

- Extender el middleware para construir `Actor`.
- Resolver `PacienteID` y `MedicoID` desde la base.
- Extender `/auth/me`.
- Implementar `GET/PUT /portal/perfil`.
- Implementar `GET /medico/agenda` básico.
- Aplicar filtros de ownership en citas, pacientes y derivaciones.
- Retirar o neutralizar permisos globales del rol `medico`.
- Probar IDOR: paciente A contra paciente B; médico A contra médico B; paciente contra datos administrativos.
- Auditar accesos a datos propios y ajenos.

**Criterio de salida:** ningún actor puede leer o modificar recursos de otro cambiando un ID en la URL o en el body. Los tests de autorización fallan si se quita el filtro.

### Fase 4 — Catálogo, horarios y disponibilidad

**Objetivo:** que el paciente vea opciones reales y slots válidos.

Tareas:

- Actualizar catálogo con `requiere_derivacion`, `activa`, orden y duración.
- Seed de `FISIO`, `ODONTO`, `NUTRI`, `OTORRINO`.
- CRUD administrativo de horarios y bloqueos.
- Perfil visible del médico: especialidad, bio, foto, duración, acepta portal.
- Generación de slots por rango, zona horaria, antelación y máximo de días.
- Restar bloqueos y citas activas.
- `GET /portal/especialidades`, `/portal/medicos`, `/portal/disponibilidad`.
- Validar que un slot no se pueda reservar fuera de horario.
- Pruebas de zona horaria, cambios de horario y solapes.

**Criterio de salida:** los slots mostrados son reservables y no incluyen horarios bloqueados, pasados o ya ocupados. Dos pacientes que reservan el mismo slot producen un solo `201` y un `409`.

### Fase 5 — Agenda directa del paciente

**Objetivo:** que el paciente agende solo las cuatro áreas permitidas.

Tareas:

- `POST /portal/citas`.
- `GET /portal/citas`, detalle, cancelación y reprogramación.
- Forzar `paciente_id` desde el actor.
- Forzar `canal = portal_paciente`.
- Validar especialidad directa; rechazar especialista sin derivación.
- Validar médico activo, acepta portal y especialidad correcta.
- Validar slot, antelación, máximo de días y política de cancelación.
- Usar `version` en reprogramación.
- Integrar con las restricciones de exclusión existentes.
- Auditar creación, cancelación y reprogramación.
- Notificación básica de confirmación (aunque el envío real llegue en Fase 8).

**Criterio de salida:** el paciente agenda Medicina General, Fisioterapia, Odontología y Nutrición; no puede agendar Cardiología u Otorrinolaringología sin derivación; no puede ver ni modificar citas ajenas; las carreras devuelven `409`.

### Fase 6 — Flujo médico mínimo y derivaciones

**Objetivo:** que el médico atienda y derive correctamente.

Tareas:

- `GET /medico/agenda` con filtros y ownership.
- `PATCH /medico/citas/{id}/estado` con transiciones válidas.
- `POST/PUT /medico/citas/{id}/nota`.
- `POST /medico/citas/{id}/derivaciones`.
- `GET /medico/derivaciones`.
- Validar cita atendida, médico de origen, especialidad `MED-GEN`, especialidad destino y vigencia.
- Crear derivación en estado `pendiente`.
- Auditar atención, nota y derivación.
- Probar que un médico no puede derivar desde una cita ajena, no atendida o de otra especialidad.

**Criterio de salida:** un médico de Medicina General atiende una cita y emite una derivación válida; un médico de otra especialidad no puede habilitar el acceso directo del portal.

### Fase 7 — Reserva de especialista con derivación

**Objetivo:** cerrar la regla central del requerimiento.

Tareas:

- `GET /portal/derivaciones` y detalle.
- Mostrar al paciente la derivación vigente y su destino.
- `POST /portal/citas` con `derivacion_id`.
- Validación transaccional con `FOR UPDATE`.
- Consumir la derivación y marcarla `agendada`.
- Índice único parcial para impedir dos citas activas con la misma derivación.
- Cancelación de cita de especialista: liberar derivación si sigue vigente.
- Reprogramación: mantener la misma derivación si no cambia de especialidad/médico.
- Marcar derivación `atendida` cuando la cita se atiende.
- Job o consulta para marcar `vencida`.
- No-show: consumir la derivación según la decisión confirmada.
- Sobrescritura administrativa con permiso, motivo y auditoría.
- Prueba E2E completa.

**Criterio de salida:** el paciente no puede saltarse la regla; la derivación se usa una sola vez; cancelar antes del corte la libera; una derivación vencida no permite reservar; la sobrescritura queda auditada.

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
- Transacción de registro con rollback ante fallo.
- Transacción de reserva con derivación.
- Liberación de derivación al cancelar.
- Versionado optimista de citas, horarios y notas.
- Stores con filtros de ownership.

### 8.3 Prueba E2E del flujo solicitado

1. Paciente se registra con documento nuevo.
2. Verifica email.
3. Inicia sesión.
4. Consulta especialidades y ve cuatro directas.
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

- Aplicar 009–012 sobre base vacía.
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

- [ ] Registro público con rate limit y verificación de email.
- [ ] Contraseñas con bcrypt y política existente.
- [ ] Tokens de verificación/reset hasheados, de un solo uso y con expiración.
- [ ] Reset de contraseña revoca sesiones.
- [ ] Paciente solo accede a sus datos.
- [ ] Médico solo accede a su agenda y a pacientes con relación de atención.
- [ ] Recepción no accede a funciones clínicas.
- [ ] Admin con MFA antes de producción (si se habilita).
- [ ] CORS con allowlist.
- [ ] TLS en el proxy.
- [ ] Secretos fuera del repositorio.
- [ ] Rate limiting distribuido.
- [ ] Validación de payloads y límites de tamaño.
- [ ] Redacción de PII en logs.
- [ ] Consentimiento y términos versionados.
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
2. Aplicar migraciones 009–012 en staging.
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
- [ ] Agenda directamente Medicina General, Fisioterapia, Odontología y Nutrición.
- [ ] No puede agendar un especialista sin derivación.
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
| “Remitir a otro paciente” es ambiguo | Modelo incorrecto | Confirmar que es derivación a especialidad/profesional |
| Paciente ya existe sin cuenta | Duplicidad de historia | No crear otro; vincular con verificación/recepción |
| Documento duplicado en registro | Dos historias clínicas | Índice único y flujo de vinculación |
| Médico con permisos globales | Privacidad | Actor + ownership; retirar permisos globales |
| Paciente agenda especialista sin derivación | Se rompe la regla central | Validación transaccional + índice único |
| Dos requests usan la misma derivación | Doble reserva | `FOR UPDATE` + índice parcial |
| Horarios sin zona horaria | Slots incorrectos | Configurar `CLINIC_TIMEZONE`; tests |
| No-show | derivación consumida o reutilizable | Confirmar política antes de implementar |
| Pediatría | Regla incómoda para menores | Confirmar excepción configurable |
| Profesionales no médicos | Rol `medico` semánticamente incorrecto | Usar `tipo_profesional`; evaluar rol `profesional_salud` |
| Email no configurado | No hay verificación/reset | Adaptador de desarrollo y proveedor real en P1 |
| Migración sobre datos reales | Pérdida de datos | Backup, ensayo, migraciones aditivas |
| Frontend actual usa permisos globales | Ruptura al cambiar roles | Coordinar release; mantener compatibilidad temporal |
| Observabilidad local | No hay producción | Completar reemplazo de Promtail y recolección durable |
| Historia clínica completa | Scope creep | Mantener P2; nota mínima en P0 |

### Preguntas que deben responderse antes de la Fase 1

1. ¿La derivación es a una especialidad, a un médico específico o a ambos?
2. ¿La derivación se puede usar una vez o varias veces dentro de su vigencia?
3. ¿Qué pasa si el paciente no asiste al especialista?
4. ¿Qué pasa si el paciente cancela con poca antelación?
5. ¿Pediatría es acceso directo para menores?
6. ¿Se exige email verificado para login o solo para agendar?
7. ¿Qué documento de identidad se usa y cómo se valida?
8. ¿Se permite que un médico sea también paciente?
9. ¿Qué profesionales entran en `FISIO`, `ODONTO` y `NUTRI`: solo médicos o profesionales independientes?
10. ¿La clínica opera en una sola zona horaria y sede?

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

1. Fase 0: confirmar reglas.
2. Fase 1: migraciones 009–011.
3. Fase 2: registro/verificación con adaptador de email de consola.
4. Fase 3: actor/ownership.
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

### Siguiente paso — Paso 2: entidades y stores de la migración 009

Actualizar:

- `entities.Usuario` con verificación, términos y cambio de contraseña;
- `entities.Paciente` y `entities.Medico` con `UsuarioID` y campos de perfil;
- `entities.Especialidad` con los flags del catálogo;
- `UsuarioStore`, `PacienteStore`, `MedicoStore` y `EspecialidadStore` para leer/escribir los campos nuevos;
- pruebas de repositorio y de mapeo.

No se implementarán todavía endpoints de registro ni de portal. Eso corresponde a la Fase 2.




