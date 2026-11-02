# MediList - API de gestión hospitalaria

Backend en **Go** para la gestión de pacientes, médicos, especialidades y citas de una clínica. Implementa una **arquitectura limpia por capas** con persistencia en **PostgreSQL**, autenticación **JWT**, control de acceso basado en roles (**RBAC**), cuentas de paciente con verificación de email, vinculación con historias existentes y auditoría de seguridad.

El modelo de dominio completo está documentado en [entidades.md](./entidades.md). Esta entrega implementa el núcleo de agenda, la infraestructura de seguridad, la Fase 2 de cuentas de paciente (registro, verificación de email, recuperación de contraseña y vinculación con historias existentes), la Fase 3 de autorización por actor, la Fase 4 de catálogo, horarios y disponibilidad, y la Fase 5 de agenda directa del paciente: reserva de las cinco especialidades de acceso directo, listado/detalle propio, reprogramación con versión y cancelación con política configurable.

---

## Arquitectura

```text
HTTP / JSON
    |
    v
transport/http      handlers, DTOs, router, middleware de auth
    |  llama casos de uso
    v
service             lógica de negocio, entidades y puertos
    |  define interfaces (ports.go)
    v
repository/postgres almacenes SQL con pgx
    |
    v
PostgreSQL
```

**Regla de dependencias**

- `transport/http` -> `service`: los handlers solo conocen casos de uso.
- `service` -> `entities`: contiene la lógica de negocio y define los **puertos** (`UsuarioStore`, `PacienteStore`, etc.).
- `repository/postgres` -> `entities`: implementa los puertos mediante almacenes SQL. No importa `service` ni `transport`.
- `cmd/api` es el **composition root**: conecta configuración, pool, almacenes, servicios y router.

La inversión de dependencias permite cambiar PostgreSQL por otro motor sin tocar la lógica de negocio.

### Estructura del proyecto

```text
MediList/
├── cmd/api/main.go                     # arranque y wiring
├── internal/
│   ├── config/config.go                # .env y variables de entorno
│   ├── observability/
│   │   ├── trace.go                    # trace_id compartido entre capas
│   │   └── logfile.go                  # archivo JSON rotativo para Promtail
│   ├── service/
│   │   ├── entities/                   # entidades del dominio
│   │   ├── ports.go                    # interfaces de repositorio
│   │   ├── auth_service.go             # login, refresh, bloqueo, cambio de clave
│   │   ├── actor.go                    # Actor de autorización y vínculos de cuenta
│   │   ├── portal_paciente_service.go  # perfil propio del paciente
│   │   ├── medico_portal_service.go    # agenda y pacientes propios del médico
│   │   ├── agenda_service.go           # CRUD de horarios y bloqueos
│   │   ├── disponibilidad_service.go   # generación de slots y reglas de reserva
│   │   ├── catalogo_service.go         # catálogo público del portal
│   │   ├── portal_cita_service.go      # reserva, reprogramación y cancelación propias
│   │   ├── cuenta_service.go           # registro, verificación, reset y vinculación
│   │   ├── error_service.go            # errores agregados (stdout + PostgreSQL)
│   │   ├── retention_service.go        # retención de errores y auditoría
│   │   ├── password.go                 # bcrypt y política de contraseñas
│   │   ├── tokens.go                   # emisión y validación JWT
│   │   ├── usuario_service.go          # usuarios, roles y auditoría
│   │   └── *_service.go                # casos de uso clínicos
│   ├── notificaciones/
│   │   └── email_dev.go                # buzón local .eml para desarrollo
│   ├── repository/postgres/
│   │   ├── db.go                       # pool pgx
│   │   ├── errors.go                   # traducción de errores SQL
│   │   ├── usuario_store.go            # usuarios, roles, bloqueos y token_version
│   │   ├── actor_store.go              # resolución de vínculos paciente/médico
│   │   ├── cuenta_store.go             # registro, tokens y solicitudes de vinculación
│   │   ├── agenda_store.go             # horarios, bloqueos y disponibilidad
│   │   ├── catalogo_store.go           # catálogo público de especialidades/médicos
│   │   ├── refresh_token_store.go      # refresh tokens hasheados
│   │   ├── auditoria_store.go          # bitácora
│   │   ├── error_store.go              # errores agregados
│   │   ├── retention_store.go          # purga de datos históricos
│   │   └── *_store.go                  # almacenes clínicos
│   └── transport/http/
│       ├── router.go                   # rutas y health check
│       ├── auth_middleware.go          # Bearer token, Actor, sesión y permisos
│       ├── audit.go                    # auditoría de escrituras y accesos a datos
│       ├── portal_handler.go           # perfil propio del paciente
│       ├── medico_portal_handler.go    # agenda y pacientes propios del médico
│       ├── agenda_handler.go           # horarios y bloqueos administrativos
│       ├── catalogo_handler.go         # catálogo y disponibilidad del portal
│       ├── portal_cita_handler.go      # citas propias del paciente
│       ├── request_context.go          # X-Request-ID, traceparent y trace_id
│       ├── report_error.go             # envío de errores a la agregación
│       ├── rate_limit.go               # límite de intentos por IP
│       ├── middleware.go               # logging, recovery y cabeceras
│       ├── dto.go                      # contratos JSON clínicos
│       ├── auth_dto.go                 # contratos JSON de auth/usuarios
│       ├── cuenta_dto.go               # contratos de registro y vinculación
│       └── *_handler.go                # endpoints
├── docs/
│   └── openapi.yaml                    # contrato OpenAPI hasta Fase 4
├── migrations/
│   ├── 001_init.sql                    # tablas clínicas, índices y triggers
│   ├── 002_seed.sql                    # especialidades iniciales
│   ├── 003_auth.sql                    # usuarios, roles, permisos, tokens y auditoría
│   ├── 004_auth_seed.sql               # roles, permisos y admin inicial
│   ├── 005_token_version.sql           # versión de sesión para revocación inmediata
│   ├── 006_errores.sql                 # errores agregados y permiso errores:leer
│   ├── 007_observabilidad.sql          # instancia, trace_id de auditoría e índices
│   ├── 008_consistencia_concurrencia.sql # exclusiones de agenda, versiones y trigger
│   ├── 009_portal_paciente.sql         # cuentas, catálogo, tokens y permisos del portal
│   ├── 010_disponibilidad.sql          # horarios recurrentes, bloqueos y exclusiones
│   ├── 011_derivaciones_metadata.sql   # derivaciones, metadata de citas y notas
│   ├── 012_cuentas_vinculacion.sql     # solicitudes de vinculación de cuentas
│   ├── 013_ownership_medico.sql        # ownership y retiro de permisos globales del médico
│   └── 014_pediatria_acceso_directo.sql # Pediatría como acceso directo (Fase 0)
├── deploy/observability/               # Loki + Promtail + Grafana + alertas
│   ├── docker-compose.yml
│   ├── loki/loki-config.yml
│   ├── promtail/promtail-config.yml
│   └── grafana/provisioning/           # datasource, dashboard y reglas
├── scripts/
│   ├── migrate.ps1                     # crea BD y aplica migraciones
│   ├── observability.ps1               # levanta/consulta el stack local
│   ├── prueba_concurrencia_disponibilidad.ps1 # prueba horarios/bloqueos concurrentes
│   ├── prueba_cuentas.ps1              # registro, tokens, reset y vinculación
│   ├── prueba_ownership.ps1            # actor, vínculos, IDOR y permisos por fila
│   ├── prueba_derivaciones.ps1         # prueba derivaciones, notas y metadata
│   ├── solapes_citas.sql               # diagnostica solapes antes de migrar
│   └── run.ps1                         # inicia la API
├── .env                                # configuración local (no versionar)
├── .env.example
├── entidades.md
└── go.mod
```

---

## Requisitos

- Go **1.26** o superior.
- PostgreSQL **15** o superior en `localhost:5432`.
- Usuario local de PostgreSQL con permisos para crear y usar la base `medilist`.
- Docker Desktop con motor Linux (opcional, solo para el stack local de observabilidad).

## Puesta en marcha

1. **Configurar credenciales** en `.env`:

   ```env
   APP_ENV=development
   APP_SERVICE_NAME=medilist-api
   # Opcional; si se omite se usa el hostname.
   # APP_INSTANCE_ID=medilist-api-1
   HTTP_PORT=8080
   LOG_FORMAT=json
   # Archivo que lee Promtail; si se omite, solo se escribe en stdout.
   LOG_FILE=logs/medilist.jsonl
   LOG_FILE_MAX_MB=100
   LOG_FILE_BACKUPS=5
   DATABASE_URL=postgres://postgres:TU_PASSWORD_LOCAL@localhost:5432/medilist?sslmode=disable
   # Pool y límites de espera para alta concurrencia.
   DB_MAX_CONNS=20
   DB_MIN_CONNS=2
   DB_STATEMENT_TIMEOUT_MS=30000
   DB_LOCK_TIMEOUT_MS=5000
   DB_IDLE_TX_TIMEOUT_MS=30000

   JWT_SECRET=CAMBIAR_POR_UN_SECRETO_ALEATORIO_DE_AL_MENOS_32_CARACTERES
   JWT_ISSUER=medilist
   ACCESS_TOKEN_MINUTES=15
   REFRESH_TOKEN_HOURS=168
   BCRYPT_COST=12
   MAX_LOGIN_ATTEMPTS=5
   LOCK_MINUTES=15

   # Retención; 0 deshabilita la purga de esa tabla.
   RETENTION_INTERVAL_HOURS=24
   ERROR_RETENTION_DAYS=90
   AUDIT_RETENTION_DAYS=365

   # Cuentas de paciente y correo de desarrollo.
   EMAIL_PROVIDER=outbox
   EMAIL_FROM=MediList <no-reply@medilist.local>
   EMAIL_OUTBOX_DIR=outbox/emails
   EMAIL_LINK_BASE_URL=http://localhost:3000
   EMAIL_VERIFICATION_HOURS=24
   PASSWORD_RESET_MINUTES=60
   TERMS_VERSION=2026-10
   MIN_EDAD_CUENTA=18

   # Agenda y disponibilidad del portal.
   CLINIC_TIMEZONE=America/Guayaquil
   PORTAL_MIN_NOTICE_HOURS=2
   PORTAL_MAX_ADVANCE_DAYS=90
   PORTAL_CANCEL_CUTOFF_HOURS=24
   AUTO_CONFIRM_PORTAL_BOOKINGS=true
   ```

   Reemplaza `postgres` y `TU_PASSWORD_LOCAL` por el usuario y la contraseña que tengas configurados en tu propia instalación local de PostgreSQL. No publiques contraseñas reales en el README ni en archivos versionados.

   `JWT_SECRET` debe tener al menos 32 caracteres. En producción usa un valor aleatorio y no reutilices el de desarrollo.

   `DB_MAX_CONNS` debe dimensionarse según `max_connections` de PostgreSQL y el número de réplicas de la API. `DB_LOCK_TIMEOUT_MS` evita que una transacción quede bloqueada indefinidamente por otra operación concurrente; `DB_IDLE_TX_TIMEOUT_MS` cierra transacciones que quedaron abiertas por error.

2. **Crear la base y aplicar migraciones**:

   ```powershell
   powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\migrate.ps1 -Password "TU_PASSWORD_LOCAL"
   ```

   El script detecta `psql.exe`, crea la base `medilist` si no existe y ejecuta los archivos de `migrations/` en orden.

   La migración `008_consistencia_concurrencia.sql` crea la extensión `btree_gist` y las restricciones de exclusión de agenda. El usuario de migración necesita permiso para crear extensiones, o la extensión debe estar instalada previamente por un administrador. Si existen citas solapadas de antes de esta versión, la migración se detiene de forma segura para no alterar datos clínicos: ejecuta `scripts/solapes_citas.sql`, corrige los horarios/estados indicados y vuelve a ejecutar la migración.

   La migración `010_disponibilidad.sql` agrega `horarios_profesional` y `bloqueos_agenda`. Sus restricciones de exclusión impiden solapes activos del mismo médico por día, hora y vigencia, y bloqueos puntuales solapados. La Fase 4 usa ese esquema sin cambios adicionales; `CLINIC_TIMEZONE` define la zona local (por defecto `America/Guayaquil`), `PORTAL_MIN_NOTICE_HOURS` la antelación mínima y `PORTAL_MAX_ADVANCE_DAYS` el horizonte máximo de reserva. La prueba de concurrencia se ejecuta sobre una base temporal con:

   ```powershell
   powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\prueba_concurrencia_disponibilidad.ps1 -Password "TU_PASSWORD_LOCAL"
   ```

   La migración `011_derivaciones_metadata.sql` agrega `derivaciones`, la metadata de `citas` y `notas_cita`. Refuerza en PostgreSQL las reglas de origen atendido, destino con derivación, médico correcto, nota por médico e impide dos citas activas o atendidas con la misma derivación mediante un índice único parcial. Sus pruebas se ejecutan sobre una base temporal con:

   ```powershell
   powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\prueba_derivaciones.ps1 -Password "TU_PASSWORD_LOCAL"
   ```

   La migración `012_cuentas_vinculacion.sql` agrega `solicitudes_vinculacion`. Una historia clínica existente sin cuenta no se duplica: el registro crea una cuenta pendiente y una solicitud que recepción debe aprobar o rechazar. Los índices únicos parciales impiden dos solicitudes pendientes para el mismo paciente o la misma cuenta. Las pruebas de registro, tokens vencidos/reutilizados, reset, revocación y carreras se ejecutan con:

   ```powershell
   powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\prueba_cuentas.ps1 -Password "TU_PASSWORD_LOCAL"
   ```

   La migración `013_ownership_medico.sql` retira al rol `medico` los permisos globales `pacientes:leer`, `citas:leer` y `citas:escribir`, y reafirma sus permisos propios (`medico:agenda:*`, `pacientes:atendidos:leer`, `derivaciones:*` y `medico:nota:*`). Además, la API bloquea por rol los endpoints administrativos aunque un access token antiguo todavía contenga los permisos retirados. Las pruebas de actor, vínculos y ownership se ejecutan con:

   ```powershell
   powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\prueba_ownership.ps1 -Password "TU_PASSWORD_LOCAL"
   ```

   La migración `014_pediatria_acceso_directo.sql` deja Pediatría como especialidad de acceso directo (`requiere_derivacion = false`) para todos, sin regla por edad. Con esto, las especialidades directas son Medicina General, Fisioterapia, Odontología, Nutrición y Pediatría; el resto sigue exigiendo derivación desde una cita de Medicina General atendida.

   El correo de desarrollo se escribe en `outbox/emails/*.eml`; el token no aparece en logs. Antes de producción se debe implementar un proveedor real detrás de `service.EmailSender` y cambiar `EMAIL_PROVIDER`. La API actualmente solo acepta `EMAIL_PROVIDER=outbox` y emite una advertencia si se usa en producción.

3. **Iniciar la API**:

   ```powershell
   .\scripts\run.ps1
   # o bien
   go run ./cmd/api
   ```

4. **Verificar**:

   ```powershell
   Invoke-RestMethod http://localhost:8080/health
   # {"status":"ok","database":"up"}
   ```

### Credenciales iniciales

La migración crea un administrador:

| Usuario | Contraseña temporal |
|---|---|
| `admin` | `Admin12345!2026` |

**Cambia esta contraseña inmediatamente** con `POST /api/v1/auth/password` o mediante `PUT /api/v1/usuarios/1`. El hash bcrypt se almacena en la base; la contraseña nunca se devuelve por la API.

---

## Autenticación

Base URL: `http://localhost:8080/api/v1`

### Registro y verificación de pacientes

1. `POST /auth/register` crea en una sola transacción la persona, el paciente, la cuenta, el rol `paciente`, el token de verificación y la auditoría. No emite sesión.
2. La API escribe un correo con un enlace de un solo uso en el buzón de desarrollo (`outbox/emails/`).
3. `POST /auth/verify-email` consume el token. Un token vencido o ya usado devuelve `400`.
4. `POST /auth/resend-verification` invalida el enlace anterior y emite uno nuevo.
5. `POST /auth/login` solo funciona para una cuenta de paciente cuando su email ya está verificado. Las cuentas internas existentes (`admin`, `medico`, `recepcion`, etc.) quedan exentas para no bloquear la operación durante la transición.
6. `POST /auth/forgot-password` y `POST /auth/reset-password` permiten recuperar la contraseña. El reset verifica el email si aún no lo estaba, incrementa `token_version`, limpia bloqueos y revoca todos los refresh tokens.

Si el documento ya existe como paciente sin cuenta, el registro **no crea otra historia clínica**. Devuelve `202` con una `solicitud_id`; recepción debe aprobar la vinculación después de validar la identidad. Si el documento ya tiene una cuenta vinculada, devuelve `409`.

```powershell
$registro = @{
  username = "juan.perez"
  email = "juan.perez@example.com"
  password = "UnaClaveSegura123!"
  nombres = "Juan"
  apellidos = "Pérez"
  tipo_documento = "CEDULA"
  numero_documento = "0102030405"
  fecha_nacimiento = "1990-05-12"
  sexo = "masculino"
  telefono = "+593999999999"
  direccion = "Av. Principal y Calle 1"
  acepta_terminos = $true
  version_terminos = "2026-10"
} | ConvertTo-Json

$alta = Invoke-RestMethod -Method Post `
  -Uri http://localhost:8080/api/v1/auth/register `
  -ContentType "application/json" -Body $registro
```

Respuesta de un paciente nuevo:

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

El enlace del correo apunta a `EMAIL_LINK_BASE_URL` y contiene el token en claro solo en el buzón; la base guarda únicamente su hash SHA-256. Nunca copies tokens a logs ni a la auditoría.

### Flujo de sesión

1. `POST /auth/login` con `username` y `password`.
2. La API devuelve un **access token** (15 min por defecto) y un **refresh token** (7 días por defecto).
3. Envía el access token en cada solicitud:

   ```http
   Authorization: Bearer <access_token>
   ```

4. Cuando el access token expire, usa `POST /auth/refresh` con el refresh token.
5. El refresh token rota en cada uso: el anterior queda revocado.
6. `POST /auth/logout` revoca el refresh token activo.
7. Cada request protegido verifica en PostgreSQL que la cuenta siga activa y que `token_version` coincida con el claim `ver`. Si cambian roles, contraseña, estado o se revocan sesiones, los access tokens anteriores dejan de funcionar de inmediato.

> **Clientes con refresh concurrente:** el refresh rota de forma atómica. Si dos solicitudes usan el mismo refresh token a la vez, solo una puede rotarlo; la otra se considera reutilización y revoca todas las sesiones del usuario, incluidos los access tokens vigentes. Los clientes deben serializar el refresh (por ejemplo, con un mutex o una única tarea de renovación) y reintentar el request original después de renovar.

### Endpoints de autenticación

| Método | Ruta | Autenticación | Descripción |
|---|---|---|---|
| `POST` | `/auth/register` | Pública, rate-limited | Registra paciente o crea solicitud de vinculación |
| `POST` | `/auth/verify-email` | Pública, rate-limited | Consume el token de verificación |
| `POST` | `/auth/resend-verification` | Pública, rate-limited | Reenvía verificación sin revelar si la cuenta existe |
| `POST` | `/auth/forgot-password` | Pública, rate-limited | Solicita recuperación sin revelar si la cuenta existe |
| `POST` | `/auth/reset-password` | Pública, rate-limited | Cambia la contraseña y revoca sesiones |
| `POST` | `/auth/login` | Pública | Inicia sesión y emite tokens; bloquea pacientes sin email verificado |
| `POST` | `/auth/refresh` | Refresh token | Rota el refresh token y emite un nuevo par |
| `POST` | `/auth/logout` | Refresh token | Revoca el refresh token |
| `GET` | `/auth/me` | Bearer | Devuelve el usuario, roles, permisos, estado de verificación y los IDs `paciente_id`/`medico_id` vinculados |
| `POST` | `/auth/password` | Bearer | Cambia la contraseña del usuario autenticado |

Los límites en memoria por instancia son: 5 registros por IP/hora; 30 verificaciones por IP/minuto; 3 reenvíos por IP+email/15 minutos; 5 solicitudes de recuperación por IP+email/15 minutos; 10 resets por IP/15 minutos. La respuesta `429` incluye `Retry-After`.

Ejemplo de recuperación:

```powershell
Invoke-RestMethod -Method Post `
  -Uri http://localhost:8080/api/v1/auth/forgot-password `
  -ContentType "application/json" `
  -Body (@{ email = "juan.perez@example.com" } | ConvertTo-Json)

# El token se lee del .eml del buzón de desarrollo.
Invoke-RestMethod -Method Post `
  -Uri http://localhost:8080/api/v1/auth/reset-password `
  -ContentType "application/json" `
  -Body (@{
    token = "<token-del-correo>"
    password_nueva = "OtraClaveSegura456!"
  } | ConvertTo-Json)
```

### Vinculación de cuentas

Solo `admin` y `recepcion` pueden aprobar o rechazar solicitudes (`cuentas:vincular`). La cuenta pendiente puede verificar su email, pero no obtiene acceso a datos clínicos hasta que la vinculación se apruebe. Al rechazar, la cuenta se desactiva y se revocan sus sesiones.

| Método | Ruta | Permiso | Descripción |
|---|---|---|---|
| `GET` | `/cuentas/vinculaciones` | `cuentas:vincular` | Lista solicitudes; filtros `estado`, `search`, paginación |
| `POST` | `/cuentas/vinculaciones/{id}/aprobar` | `cuentas:vincular` | Vincula la cuenta con la historia existente |
| `POST` | `/cuentas/vinculaciones/{id}/rechazar` | `cuentas:vincular` | Rechaza con `motivo` obligatorio y desactiva la cuenta |
| `POST` | `/cuentas/vincular-paciente` | `cuentas:vincular` | Vinculación directa desde recepción |
| `POST` | `/cuentas/vincular-medico` | `usuarios:escribir` | Asocia una cuenta con rol `medico` a un perfil médico |

```powershell
Invoke-RestMethod -Method Get `
  -Uri "http://localhost:8080/api/v1/cuentas/vinculaciones?estado=pendiente&search=0102030405" `
  -Headers $headers

Invoke-RestMethod -Method Post `
  -Uri http://localhost:8080/api/v1/cuentas/vinculaciones/7/aprobar `
  -Headers $headers
```

Rechazo:

```json
{
  "motivo": "El documento no coincide con la historia clínica"
}
```

### Ejemplo de login

```powershell
$login = Invoke-RestMethod -Method Post `
  -Uri http://localhost:8080/api/v1/auth/login `
  -ContentType "application/json" `
  -Body (@{ username = "admin"; password = "Admin12345!2026" } | ConvertTo-Json)

$access  = $login.data.access_token
$refresh = $login.data.refresh_token
$headers = @{ Authorization = "Bearer $access" }
```

Respuesta:

```json
{
  "data": {
    "access_token": "eyJ...",
    "refresh_token": "b3...",
    "token_type": "Bearer",
    "expires_in": 900,
    "usuario": {
      "id": 1,
      "username": "admin",
      "roles": ["admin"],
      "permisos": ["pacientes:leer", "pacientes:escribir", "..."]
    }
  }
}
```

### Cambio de contraseña

```powershell
Invoke-RestMethod -Method Post `
  -Uri http://localhost:8080/api/v1/auth/password `
  -Headers $headers `
  -ContentType "application/json" `
  -Body (@{
    password_actual = "Admin12345!2026"
    password_nueva  = "NuevaClaveSegura#2026"
  } | ConvertTo-Json)
```

La nueva contraseña debe tener al menos 12 caracteres e incluir mayúscula, minúscula, número y carácter especial. Al cambiarla se incrementa `token_version` y se revocan todos los refresh tokens, por lo que el access token anterior también queda inválido de inmediato.

---

## Roles y permisos

La API usa RBAC. Los permisos se agregan a los roles y los roles a los usuarios. El middleware valida el permiso requerido en cada ruta; el rol `admin` tiene todos los permisos.

### Roles incluidos

| Rol | Enfoque |
|---|---|
| `admin` | Acceso total, gestión de usuarios y auditoría |
| `medico` | Agenda y pacientes propios; sin acceso global a pacientes/citas |
| `recepcion` | Gestión de pacientes, agenda y vinculación de cuentas |
| `enfermeria` | Consulta de pacientes, médicos y citas |
| `auditor` | Consulta de usuarios, roles y auditoría |
| `paciente` | Cuenta propia, perfil y agenda de citas propias |

### Matriz de permisos

| Recurso | `admin` | `medico` | `recepcion` | `enfermeria` | `auditor` | `paciente` |
|---|---|---|---|---|---|---|
| Pacientes (leer) | Sí | No global; solo atendidos | Sí | Sí | No | No |
| Pacientes (escribir) | Sí | No | Sí | No | No | No |
| Médicos (leer) | Sí | Sí | Sí | Sí | No | No |
| Médicos (escribir) | Sí | No | No | No | No | No |
| Especialidades (leer) | Sí | Sí | Sí | Sí | No | No |
| Especialidades (escribir) | Sí | No | No | No | No | No |
| Citas (leer) | Sí | No global; solo propias | Sí | Sí | No | No |
| Citas (escribir) | Sí | No global; acciones propias en Fase 6 | Sí | No | No | No |
| Perfil de portal (leer/escribir) | Sí | No | No | No | No | Sí |
| Agenda médica propia | Sí | Sí | No | No | No | No |
| Agenda administrativa (horarios/bloqueos) | Sí | No | Sí | No | No | No |
| Catálogo de portal | Sí | No | No | No | No | Sí |
| Disponibilidad de portal | Sí | No | No | No | No | Sí |
| Citas propias del portal | Sí | No | No | No | No | Sí |
| Cuentas (vincular) | Sí | No | Sí | No | No | No |
| Usuarios (leer) | Sí | No | No | No | Sí | No |
| Usuarios (escribir) | Sí | No | No | No | No | No |
| Roles (leer) | Sí | No | No | No | Sí | No |
| Auditoría (leer) | Sí | No | No | No | Sí | No |
| Errores (leer) | Sí | No | No | No | Sí | No |

El rol `paciente` ya tiene permisos `portal:*` y `catalogo:leer`; la Fase 3 expone `GET/PUT /portal/perfil`, la Fase 4 el catálogo/disponibilidad y la Fase 5 la reserva, reprogramación y cancelación de citas propias. El rol `medico` ya no tiene permisos globales de pacientes/citas: usa `medico:agenda:leer` y `pacientes:atendidos:leer`, y sus acciones de escritura propias se completarán en la Fase 6.

### Administración de usuarios

Solo usuarios con `usuarios:leer` o `usuarios:escribir` pueden acceder.

| Método | Ruta | Permiso | Descripción |
|---|---|---|---|
| `GET` | `/usuarios` | `usuarios:leer` | Lista usuarios; filtros `search`, `activos=true` |
| `POST` | `/usuarios` | `usuarios:escribir` | Crea usuario con roles |
| `GET` | `/usuarios/{id}` | `usuarios:leer` | Obtiene un usuario |
| `PUT` | `/usuarios/{id}` | `usuarios:escribir` | Actualiza datos, roles, estado o contraseña |
| `POST` | `/usuarios/{id}/desbloquear` | `usuarios:escribir` | Limpia el bloqueo por intentos fallidos |
| `POST` | `/usuarios/{id}/revocar-sesiones` | `usuarios:escribir` | Invalida de inmediato access y refresh tokens |
| `GET` | `/roles` | `roles:leer` | Lista roles y sus permisos |
| `GET` | `/auditoria` | `auditoria:leer` | Consulta la bitácora |
| `GET` | `/errores` | `errores:leer` | Consulta errores agregados |

Crear un usuario:

```powershell
$nuevo = @{
  username  = "recepcion1"
  email     = "recepcion1@medilist.local"
  nombres   = "Ana"
  apellidos = "Recepción"
  password  = "Recepcion123!"
  roles     = @("recepcion")
} | ConvertTo-Json

Invoke-RestMethod -Method Post -Uri http://localhost:8080/api/v1/usuarios `
  -Headers $headers -ContentType "application/json" -Body $nuevo
```

Notas:

- El `username` es inmutable después de crear la cuenta.
- No puedes desactivar tu propia cuenta ni quitarte el rol `admin` a ti mismo.
- Si cambian los roles, la contraseña o se desactiva la cuenta, se revocan todos los refresh tokens y se incrementa `token_version`; los access tokens anteriores fallan con `401`.
- `POST /usuarios/{id}/revocar-sesiones` permite a un administrador forzar el cierre inmediato de todas las sesiones de una cuenta.
- El endpoint de desbloqueo permite rehabilitar una cuenta bloqueada por intentos fallidos.

---

## API clínica

### Especialidades

| Método | Ruta | Permiso | Descripción |
|---|---|---|---|
| `GET` | `/especialidades` | `especialidades:leer` | Lista con `search`, `limit`, `offset` |
| `POST` | `/especialidades` | `especialidades:escribir` | Crea una especialidad |
| `GET` | `/especialidades/{id}` | `especialidades:leer` | Obtiene una especialidad |
| `PUT` | `/especialidades/{id}` | `especialidades:escribir` | Actualiza una especialidad |
| `DELETE` | `/especialidades/{id}` | `especialidades:escribir` | Elimina si no está referenciada |

Acceso directo del paciente: `MED-GEN`, `FISIO`, `ODONTO`, `NUTRI` y `PED`. El resto exige derivación vigente; la fuente de verdad es `especialidades.requiere_derivacion`, no una lista en el código. El contrato OpenAPI está en `docs/openapi.yaml`.

### Pacientes

Los endpoints de esta tabla son administrativos. Además del permiso, la API exige un rol `admin`, `recepcion` o `enfermeria`; un médico no puede usarlos aunque su access token contenga permisos antiguos. El médico consulta solo pacientes con relación de atención mediante `/medico/pacientes/{id}`.

| Método | Ruta | Permiso | Descripción |
|---|---|---|---|
| `GET` | `/pacientes` | `pacientes:leer` | Lista; filtros `search`, `activos=true`, paginación |
| `POST` | `/pacientes` | `pacientes:escribir` | Crea persona + paciente en una transacción |
| `GET` | `/pacientes/{id}` | `pacientes:leer` | Obtiene el paciente con sus datos de persona |
| `PUT` | `/pacientes/{id}` | `pacientes:escribir` | Actualiza datos personales y clínicos |
| `DELETE` | `/pacientes/{id}` | `pacientes:escribir` | Baja lógica (`activo=false`) |

### Médicos

| Método | Ruta | Permiso | Descripción |
|---|---|---|---|
| `GET` | `/medicos` | `medicos:leer` | Lista; filtros `search`, `activos=true`, paginación |
| `POST` | `/medicos` | `medicos:escribir` | Crea persona + médico en una transacción |
| `GET` | `/medicos/{id}` | `medicos:leer` | Obtiene médico con especialidad |
| `PUT` | `/medicos/{id}` | `medicos:escribir` | Actualiza datos y especialidad |
| `DELETE` | `/medicos/{id}` | `medicos:escribir` | Baja lógica (`activo=false`) |

El `POST` y el `PUT` de médicos aceptan `duracion_cita_min`, `bio`, `foto_url` y `acepta_portal`. Esos campos alimentan el perfil visible del profesional en el portal.

### Agenda administrativa

Requiere `agenda:gestionar` y la guarda de rol administrativo (`admin` o `recepcion`). Los horarios son recurrentes y locales; los bloqueos son puntuales y se envían en UTC con RFC 3339.

| Método | Ruta | Permiso | Descripción |
|---|---|---|---|
| `GET` | `/medicos/{id}/horarios` | `agenda:gestionar` | Lista horarios activos e inactivos |
| `POST` | `/medicos/{id}/horarios` | `agenda:gestionar` | Crea horario; valida rango, vigencia y solape |
| `PUT` | `/medicos/{id}/horarios/{horarioId}` | `agenda:gestionar` | Actualiza con `version` |
| `DELETE` | `/medicos/{id}/horarios/{horarioId}` | `agenda:gestionar` | Baja lógica (`activo=false`) |
| `GET` | `/medicos/{id}/bloqueos` | `agenda:gestionar` | Filtros `desde`/`hasta` (YYYY-MM-DD) |
| `POST` | `/medicos/{id}/bloqueos` | `agenda:gestionar` | Crea bloqueo con `inicio`, `fin` y `motivo` |
| `PUT` | `/medicos/{id}/bloqueos/{bloqueoId}` | `agenda:gestionar` | Actualiza rango y motivo, sin solaparse |
| `DELETE` | `/medicos/{id}/bloqueos/{bloqueoId}` | `agenda:gestionar` | Elimina el bloqueo |

### Citas

Los endpoints de esta tabla son administrativos y aplican la misma guarda por rol que pacientes. El médico usa `/medico/agenda` y `/medico/citas/{id}` para sus propias citas.

| Método | Ruta | Permiso | Descripción |
|---|---|---|---|
| `GET` | `/citas` | `citas:leer` | Filtros: `paciente_id`, `medico_id`, `estado`, `desde`, `hasta`, `search` |
| `POST` | `/citas` | `citas:escribir` | Agenda una cita y valida solape de médico y paciente |
| `GET` | `/citas/{id}` | `citas:leer` | Obtiene una cita |
| `PUT` | `/citas/{id}` | `citas:escribir` | Reagenda o actualiza la cita |
| `PATCH` | `/citas/{id}/estado` | `citas:escribir` | Aplica una transición de estado |
| `DELETE` | `/citas/{id}` | `citas:escribir` | Cancela la cita (no borra el registro) |

Estados: `programada`, `confirmada`, `atendida`, `no_asistio`, `cancelada`, `reprogramada`.

Transiciones permitidas:

```text
programada -> confirmada -> atendida
     |             \-> no_asistio
     \-> cancelada
     \-> reprogramada
```

### Portal del paciente

El perfil se resuelve exclusivamente desde `pacientes.usuario_id`. El body no admite `paciente_id`, `medico_id`, documento, nombres ni datos clínicos; un campo desconocido produce `400`.

| Método | Ruta | Permiso | Descripción |
|---|---|---|---|
| `GET` | `/portal/perfil` | `portal:perfil:leer` | Devuelve la historia propia y el estado de la cuenta |
| `PUT` | `/portal/perfil` | `portal:perfil:escribir` | Actualiza teléfono, dirección y contacto de emergencia; acepta `version` |
| `GET` | `/portal/especialidades` | `catalogo:leer` | Catálogo activo con `requiere_derivacion` y duración |
| `GET` | `/portal/medicos` | `catalogo:leer` | Médicos activos que aceptan portal; filtro `especialidad_id` |
| `GET` | `/portal/disponibilidad` | `portal:citas:leer` | Slots reservables por `medico_id`, `desde` y `hasta` |
| `POST` | `/portal/citas` | `portal:citas:escribir` | Agenda una cita propia en una especialidad directa |
| `GET` | `/portal/citas` | `portal:citas:leer` | Lista solo las citas propias; filtros de estado/fecha |
| `GET` | `/portal/citas/{id}` | `portal:citas:leer` | Detalle solo si la cita es propia |
| `PUT` | `/portal/citas/{id}` | `portal:citas:escribir` | Reprograma el slot propio; exige `version` |
| `POST` | `/portal/citas/{id}/cancelar` | `portal:citas:escribir` | Cancela si se respeta el corte configurado |

`/portal/medicos` devuelve solo datos públicos del profesional (nombres, especialidad, bio, foto y duración), nunca documento, email, teléfono ni dirección. `/portal/disponibilidad` genera slots en `CLINIC_TIMEZONE`, los entrega en UTC, resta bloqueos y citas `programada`/`confirmada`, aplica la antelación mínima y el horizonte máximo. Las fechas `desde`/`hasta` son inclusivas y el rango no puede superar el horizonte configurado.

En `/portal/citas`, `paciente_id` se obtiene exclusivamente del actor. El servidor fuerza `canal=portal_paciente`, usa la duración real del slot, valida que la especialidad no requiera derivación y aplica `AUTO_CONFIRM_PORTAL_BOOKINGS`. La reprogramación conserva paciente, médico y especialidad, y la cancelación exige al menos `PORTAL_CANCEL_CUTOFF_HOURS` de antelación. Las citas `atendida`, `no_asistio`, `cancelada` o `reprogramada` no se pueden reprogramar. Al crear la cita se escribe una confirmación básica en el buzón de desarrollo; el fallo de correo no revierte la reserva y el outbox real queda para la Fase 8.

### Portal del médico

El médico se resuelve exclusivamente desde `medicos.usuario_id`. La agenda ignora cualquier `medico_id` enviado en la query o el body y siempre filtra por el vínculo del actor. Los detalles comprueban la propiedad y devuelven `403` si la cita o el paciente no pertenecen al médico.

| Método | Ruta | Permiso | Descripción |
|---|---|---|---|
| `GET` | `/medico/agenda` | `medico:agenda:leer` | Citas propias; filtros `estado`, `desde`, `hasta`, paginación |
| `GET` | `/medico/citas/{id}` | `medico:agenda:leer` | Detalle solo si la cita es propia |
| `GET` | `/medico/pacientes/{id}` | `pacientes:atendidos:leer` | Paciente con cita `programada`, `confirmada` o `atendida` con el médico |

### Concurrencia y consistencia

Las operaciones que pueden competir entre sí están protegidas también en PostgreSQL, no solo con validaciones previas en el servicio:

- **Solapamiento de agenda:** las restricciones `citas_medico_sin_solape` y `citas_paciente_sin_solape` usan `EXCLUDE USING gist` sobre intervalos `[inicio, fin)`. Solo cuentan las citas `programada` y `confirmada`; una cita cancelada, atendida, no asistida o reprogramada libera el horario. Dos solicitudes simultáneas para el mismo médico o el mismo paciente producen una cita y un `409 Conflict`, nunca dos.
- **Horarios y bloqueos:** `horarios_profesional_sin_solape` impide dos horarios activos del mismo médico que se solapen en día, hora y vigencia. `bloqueos_agenda_medico_sin_solape` impide dos bloqueos puntuales solapados del mismo médico. Las horas de los horarios son locales de la clínica; los bloqueos se guardan en UTC. La prueba de concurrencia crea dos transacciones que compiten por el mismo horario/bloqueo y exige exactamente un conflicto `23P01`.
- **Actualizaciones perdidas:** especialidades, pacientes, médicos, citas y usuarios tienen una columna `version`. Las respuestas incluyen `version` y los `PUT` pueden enviarla; si el registro cambió desde que el cliente lo leyó, la escritura devuelve `409` en lugar de sobrescribir cambios ajenos. Aunque el cliente no envíe `version`, el servicio usa la versión leída al inicio de la operación y la base vuelve a comprobarla al escribir.
- **Transiciones de estado:** dos cambios simultáneos sobre la misma cita solo aplican uno; el otro devuelve `409`. La cancelación es idempotente si la cita ya quedó cancelada.
- **Participantes activos:** al crear una cita se bloquean las filas del paciente y del médico dentro de la transacción para que una baja concurrente no se cuele entre la validación y el `INSERT`. Un trigger de PostgreSQL refuerza la validación de participantes y fechas.
- **Autenticación:** el contador de intentos fallidos se incrementa con bloqueo de fila; la rotación del refresh token consume el anterior y crea el nuevo en una sola transacción; el refresh guarda la `token_version` con la que fue emitido para detectar una revocación concurrente.
- **Pool y esperas:** `DB_MAX_CONNS`, `DB_MIN_CONNS`, `DB_STATEMENT_TIMEOUT_MS`, `DB_LOCK_TIMEOUT_MS` y `DB_IDLE_TX_TIMEOUT_MS` acotan el consumo de conexiones y el tiempo de espera por bloqueos. Un timeout de bloqueo se traduce en `409` para que el cliente reintente, no en un `500`.

Los conflictos `23P01` (exclusión), `40001` (serialización), `40P01` (deadlock) y `55P03` (lock timeout) se traducen a errores de dominio reintentables.

### Auditoría

| Método | Ruta | Permiso | Descripción |
|---|---|---|---|
| `GET` | `/auditoria` | `auditoria:leer` | Filtros `search`, `accion`, `usuario_id`, `trace_id`, `limit`, `offset` |

Se registran:

- Inicios de sesión exitosos y fallidos.
- Bloqueos de cuenta y reutilización de refresh tokens.
- Cambios de contraseña.
- Operaciones de escritura autenticadas (`POST`, `PUT`, `PATCH`, `DELETE`).
- Intentos de acceso denegados por falta de permisos.
- Consultas a `/auditoria` y `/errores`, porque exponen información sensible.
- Accesos autorizados a datos clínicos propios o administrativos y los intentos denegados por ownership en `/portal/perfil`, `/medico/agenda`, `/medico/citas/{id}`, `/medico/pacientes/{id}`, `/pacientes` y `/citas`. Los intentos rechazados se registran con el sufijo `_denegado`.
- Consultas administrativas de horarios y bloqueos, y escrituras de agenda con `agenda:gestionar`.
- `trace_id` en cada evento para cruzarlo con los logs y la tabla `errores_aplicacion`.

No se almacenan contraseñas, tokens ni cuerpos de solicitud en la bitácora.

### Observabilidad y agregación de errores

La API está preparada para un agregador de logs (Loki, ELK/OpenSearch, CloudWatch, Datadog, etc.):

- **Logs estructurados JSON:** con `LOG_FORMAT=json` cada línea incluye `time`, `level`, `msg`, `service`, `env` e `instance` (`APP_INSTANCE_ID` o hostname). Las solicitudes agregan `trace_id`, `method`, `path`, `status`, `duration_ms`, `usuario` y `usuario_id`. En `APP_ENV=production` la API exige `LOG_FORMAT=json` para no romper la agregación.
- **Correlación:** cada request recibe un `trace_id`. Se acepta el estándar W3C `traceparent` (tiene prioridad) o un `X-Request-ID` válido; si no, se genera uno. El valor vuelve en `X-Request-ID` y en `request_id` dentro de las respuestas de error.
- **Errores agregados:** los errores internos (`5xx`) y los pánicos recuperados se emiten a stdout y se guardan en `errores_aplicacion` con `trace_id`, `servicio`, `instancia`, `nivel`, `componente`, ruta, método, estado, código, mensaje, `usuario_id`, usuario, IP, user agent y `stack_trace`. La escritura en base es *best-effort* (timeout de 2 s): si falla, el error sigue disponible en stdout y queda un log de la falla de persistencia.
- **Doble destino:** el mismo error se emite a stdout (para que lo recolecte el agente de logs) y a PostgreSQL (para consulta y auditoría desde la API). Ambos se correlacionan por `trace_id`.
- **Auditoría correlacionada:** `auditoria.trace_id` permite cruzar un login, un acceso denegado o una consulta sensible con sus logs y errores técnicos. Las lecturas de `/auditoria` y `/errores` también se auditan.
- **Errores del servidor HTTP:** `http.Server.ErrorLog` usa el mismo logger estructurado, de modo que los errores internos de `net/http` no quedan en texto plano fuera del agregador.
- **Microservicios:** configura un `APP_SERVICE_NAME` distinto por servicio, propaga `traceparent`/`X-Request-ID` en las llamadas salientes y envía el stdout JSON al mismo agregador. El agregador puede reconstruir una transacción completa agrupando por `trace_id`; los campos `service`, `instancia` y `componente` distinguen servicio, réplica y capa.
- **Retención automática:** un job interno purga `errores_aplicacion` y `auditoria` según `ERROR_RETENTION_DAYS` y `AUDIT_RETENTION_DAYS`. Se ejecuta al arrancar y cada `RETENTION_INTERVAL_HOURS`; `0` deshabilita la purga de esa tabla.
- **Consulta:** `GET /api/v1/errores` con permiso `errores:leer` (admin y auditor). Filtros: `nivel`, `servicio`, `instancia`, `trace_id`, `search`, `limit`, `offset`.

Ejemplo de consulta:

```powershell
$errores = Invoke-RestMethod -Method Get `
  -Uri "http://localhost:8080/api/v1/errores?nivel=error&limit=20" `
  -Headers $headers
```

Para producción, envía el stdout JSON al agregador, ajusta la retención automática a tu política y alerta sobre incrementos de `nivel=error`, `estado_http=500`, fallas de persistencia (`no se pudo persistir el error agregado`) y fallas de retención. No se registran contraseñas, tokens ni cuerpos de solicitud; aun así, restringe el acceso a `errores:leer` y `auditoria:leer` porque los mensajes pueden incluir detalles internos de SQL y datos de seguridad.

### Stack local con Loki, Promtail y Grafana

El repositorio incluye un stack reproducible para desarrollo y pruebas. No es un despliegue endurecido para producción.

Requisitos: Docker Desktop con el motor Linux iniciado.

1. Inicia la API con `LOG_FILE` configurado (el `.env` de ejemplo ya lo incluye):

   ```powershell
   .\scripts\run.ps1
   ```

2. Levanta el stack:

   ```powershell
   .\scripts\observability.ps1 up
   ```

3. Abre los servicios:

   | Servicio | URL | Credenciales |
   |---|---|---|
   | Grafana | `http://localhost:3000` | `admin` / `medilist-admin` (por defecto) |
   | Dashboard | `http://localhost:3000/d/medilist-observabilidad` | Las mismas de Grafana |
   | Loki | `http://localhost:3100` | Sin autenticación en local |
   | Promtail | `http://localhost:9080` | Sin autenticación en local |
   | Receptor de alertas | `http://localhost:8081/alert` | Sin autenticación en local |

   Cambia la contraseña creando `deploy/observability/.env` con:

   ```env
   GRAFANA_ADMIN_PASSWORD=una-clave-local-segura
   ```

4. Comandos útiles:

   ```powershell
   .\scripts\observability.ps1 status
   .\scripts\observability.ps1 logs
   .\scripts\observability.ps1 down
   ```

   `down` conserva los volúmenes de Grafana y Loki. Para borrarlos usa `docker compose -f .\deploy\observability\docker-compose.yml down -v`.

Qué incluye:

- **Promtail** lee `logs/*.jsonl`, extrae `level`, `service`, `env` e `instance` como etiquetas y envía a Loki. La API rota el archivo por tamaño (`LOG_FILE_MAX_MB`) y conserva `LOG_FILE_BACKUPS` copias; Promtail sigue el archivo activo y no reprocesa las copias rotadas.
- **Loki** conserva 30 días (`720h`) y usa almacenamiento local en el volumen `loki-data`.
- **Grafana** aprovisiona automáticamente el datasource Loki, el dashboard `MediList - Observabilidad` y las reglas de alerta:
  - solicitudes con `status >= 500` en los últimos 5 minutos;
  - logs con `level=ERROR` en los últimos 5 minutos;
  - fallos `no se pudo persistir el error agregado` en los últimos 5 minutos;
  - fallos de retención de errores o auditoría en la última hora.
- **Webhook local:** las alertas se envían a `alert-sink`, que imprime el JSON recibido en `docker compose logs alert-sink`. Sustituye esa URL por Slack, Teams, PagerDuty o tu endpoint real en `deploy/observability/grafana/provisioning/alerting/contact-points.yml`.

Verifica que los logs llegan a Loki:

```powershell
$query = [uri]::EscapeDataString('{job="medilist-api"}')
Invoke-RestMethod "http://localhost:3100/loki/api/v1/query_range?query=$query&limit=5"
```

Para probar la regla de 5xx sin provocar una falla real, escribe una línea sintética en un archivo aparte. No escribas directamente en `medilist.jsonl`: la API lo mantiene abierto para escritura.

```powershell
$line = @{
  time = (Get-Date).ToUniversalTime().ToString("o")
  level = "INFO"
  msg = "solicitud HTTP"
  service = "medilist-api"
  env = "development"
  instance = "local"
  trace_id = "prueba-5xx"
  method = "GET"
  path = "/prueba-5xx"
  status = 500
} | ConvertTo-Json -Compress

# UTF-8 sin BOM para que Promtail pueda parsear el JSON.
$utf8 = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText((Join-Path (Get-Location) "logs\prueba-5xx.jsonl"), $line + "`n", $utf8)

# Opcional: hace que Promtail descubra el archivo de inmediato.
docker compose -f .\deploy\observability\docker-compose.yml restart promtail
```

Después de la evaluación (aproximadamente uno o dos minutos), revisa:

```powershell
docker compose -f .\deploy\observability\docker-compose.yml logs --tail=20 alert-sink
Remove-Item .\logs\prueba-5xx.jsonl
```

Notas para producción:

- Cambia la contraseña de Grafana, no publiques los puertos `3000`, `3100`, `9080` ni `8081` fuera de la red interna y termina TLS en un reverse proxy.
- Reemplaza `alert-sink` por un receptor real y protege el webhook con HTTPS, autenticación o firma.
- Para varias réplicas, usa un archivo o etiqueta `instance` distinto por réplica y un recolector por nodo (Promtail DaemonSet, Fluent Bit, etc.). No compartas un único archivo entre réplicas.
- Si usas Kubernetes, reemplaza el bind mount de `logs/` por un volumen efímero o `emptyDir` y ejecuta Promtail como DaemonSet.
- El archivo `logs/medilist.jsonl` puede contener rutas, usuarios, IP y stack traces. Restringe sus permisos y no lo publiques. Si solo usas stdout, deja `LOG_FILE` vacío.
- El stack local no sustituye la retención de PostgreSQL ni la auditoría; son mecanismos complementarios.

---

## Formato de respuesta

```json
{
  "data": { "...": "..." }
}
```

Listas:

```json
{
  "data": [],
  "meta": { "limit": 50, "offset": 0, "count": 0 }
}
```

Errores:

```json
{
  "error": {
    "code": "validation_error",
    "message": "fecha_hora_inicio no puede estar en el pasado"
  },
  "request_id": "07882f7de83c103cd94b29005fb1954a"
}
```

El `request_id` también se devuelve en la cabecera `X-Request-ID` y permite correlacionar el error con los logs, la auditoría y la tabla `errores_aplicacion`. Si el cliente envía W3C `traceparent`, se usa su trace-id como prioridad.

Códigos:

| Código | Significado |
|---|---|
| `400` | Validación o token de cuenta vencido/reutilizado |
| `401` | Token ausente, inválido, expirado o sesión revocada |
| `403` | Sin permiso para el recurso o email no verificado (`email_no_verificado`) |
| `404` | No encontrado |
| `409` | Conflicto (documento duplicado, solicitud ya resuelta, solape de agenda) |
| `423` | Cuenta bloqueada temporalmente |
| `429` | Demasiadas solicitudes; incluye `Retry-After` |
| `500` | Error interno |

El registro devuelve `201` para una historia nueva y `202` cuando crea una solicitud de vinculación. El reenvío y la solicitud de reset devuelven `202` aunque la cuenta no exista, para no filtrar información.

---

## Ejemplos rápidos

### Crear paciente

```powershell
$body = @{
  persona = @{
    tipo_documento   = "CEDULA"
    numero_documento = "1712345678"
    nombres          = "Ana"
    apellidos        = "Pérez"
    fecha_nacimiento = "1990-05-10"
    sexo             = "femenino"
    telefono         = "0999999999"
    email            = "ana.perez@example.com"
    direccion        = "Quito"
  }
  grupo_sanguineo              = "O"
  factor_rh                    = "+"
  contacto_emergencia_nombre   = "Luis Pérez"
  contacto_emergencia_telefono = "0988888888"
} | ConvertTo-Json -Depth 5

Invoke-RestMethod -Uri http://localhost:8080/api/v1/pacientes `
  -Method Post -ContentType "application/json" -Headers $headers -Body $body
```

### Crear médico

```powershell
$body = @{
  persona = @{
    tipo_documento   = "CEDULA"
    numero_documento = "1700000001"
    nombres          = "Carlos"
    apellidos        = "Mora"
    fecha_nacimiento = "1980-02-20"
    sexo             = "masculino"
    telefono         = "0977777777"
    email            = "carlos.mora@example.com"
    direccion        = "Quito"
  }
  especialidad_id  = 1
  numero_licencia  = "LIC-0001"
  tipo_profesional = "medico"
} | ConvertTo-Json -Depth 5

Invoke-RestMethod -Uri http://localhost:8080/api/v1/medicos `
  -Method Post -ContentType "application/json" -Headers $headers -Body $body
```

### Agendar cita

```powershell
$body = @{
  paciente_id       = 1
  medico_id         = 1
  fecha_hora_inicio = "2026-11-05T09:00:00-05:00"
  duracion_min      = 30
  tipo              = "primera_vez"
  motivo            = "Chequeo general"
} | ConvertTo-Json

Invoke-RestMethod -Uri http://localhost:8080/api/v1/citas `
  -Method Post -ContentType "application/json" -Headers $headers -Body $body
```

### Confirmar cita

```powershell
Invoke-RestMethod -Uri http://localhost:8080/api/v1/citas/1/estado `
  -Method Patch -ContentType "application/json" -Headers $headers `
  -Body '{"estado":"confirmada"}'
```

---

## Pruebas

Suite normal (las pruebas de integración se omiten si no existe `TEST_DATABASE_URL`):

```powershell
gofmt -l .
go build ./...
go vet ./...
go test -count=1 ./...
```

Pruebas de persistencia y concurrencia sobre una base temporal creada desde cero:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\prueba_cuentas.ps1 -Password "TU_PASSWORD_LOCAL"
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\prueba_ownership.ps1 -Password "TU_PASSWORD_LOCAL"
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\prueba_derivaciones.ps1 -Password "TU_PASSWORD_LOCAL"
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\prueba_concurrencia_disponibilidad.ps1 -Password "TU_PASSWORD_LOCAL"
```

Las pruebas de ownership cubren la resolución de vínculos, la retirada de permisos globales del rol `medico`, el perfil propio, la agenda forzada por médico, el bloqueo de citas/pacientes ajenos, la guarda por roles en endpoints administrativos y la auditoría de accesos denegados.

Las pruebas de la Fase 4 cubren:

- conversión de horas locales de `America/Guayaquil` a UTC;
- resta de bloqueos y citas activas;
- antelación mínima, horizonte máximo y vigencia de horarios;
- validación de un slot concreto (`ValidarSlot`) y rechazo de inicios no alineados o bloqueados;
- validación de solapes y control de versión de horarios;
- CRUD de horarios/bloqueos y consulta de citas activas contra PostgreSQL (`TestAgendaStoreCRUDYConsultasDeDisponibilidad`).

La Fase 5 no agrega una migración nueva: reutiliza las columnas de metadata de citas de la migración 011. La prueba de integración del store se ejecuta con `TEST_DATABASE_URL` apuntando a una base migrada; se omiten automáticamente si la variable no existe. También se validó el flujo HTTP completo sobre una base temporal desde cero: crear médico, crear horario, consultar disponibilidad (2 slots), crear bloqueo (1 slot), rechazar solape y versión obsoleta con `409`, actualizar con `200` y desactivar horario (0 slots).

Las pruebas de la Fase 5 cubren:

- creación forzando paciente del actor, `canal=portal_paciente`, autor y duración del slot;
- rechazo de especialidades con derivación y médicos que no aceptan portal;
- listado y detalle con ownership por paciente;
- reprogramación con `version`, conservando paciente/médico;
- cancelación antes/después del corte configurado;
- persistencia de `canal`, `creada_por_usuario_id` y `motivo_cancelacion` en PostgreSQL;
- carrera de dos pacientes por el mismo slot: una cita y un `409`.

El E2E de Fase 5 se ejecutó sobre una base temporal desde cero con un paciente verificado: reserva `201`, cita `confirmada` con canal `portal_paciente`, listado propio, reprogramación `200`, cancelación `200` y rechazo de Cardiología con `409 requiere_derivacion`.

Las pruebas de cuentas cubren registro transaccional, documento duplicado, dos registros concurrentes del mismo documento, verificación de un solo uso, token vencido y reutilizado, reset con revocación de refresh tokens, aprobación/rechazo de vinculación y vinculación de médico. También se prueba que el login de un paciente sin email verificado no emite sesión y que una contraseña incorrecta no revela el estado de verificación.

---

## Entidades implementadas

| Entidad | Tabla | Almacén |
|---|---|---|
| Especialidad | `especialidades` | `EspecialidadStore` |
| Persona | `personas` | Gestionada dentro de pacientes y médicos |
| Paciente | `pacientes` | `PacienteStore` |
| Médico | `medicos` | `MedicoStore` |
| Cita | `citas` | `CitaStore` |
| HorarioProfesional | `horarios_profesional` | `AgendaStore` |
| BloqueoAgenda | `bloqueos_agenda` | `AgendaStore` |
| Disponibilidad (calculada) | `horarios_profesional`, `bloqueos_agenda`, `citas` | `DisponibilidadService` |
| Usuario | `usuarios` | `UsuarioStore` |
| Rol | `roles` | `RolStore` |
| Permiso | `permisos` | Catálogo RBAC |
| UsuarioRol | `usuario_roles` | `UsuarioStore` |
| RolPermiso | `rol_permisos` | `RolStore` |
| RefreshToken | `refresh_tokens` | `RefreshTokenStore` |
| TokenCuenta | `tokens_cuenta` | `CuentaStore` |
| SolicitudVinculacion | `solicitudes_vinculacion` | `CuentaStore` |
| Auditoría | `auditoria` | `AuditoriaStore` |
| ErrorAplicacion | `errores_aplicacion` | `ErrorStore` |
| Retención | `auditoria`, `errores_aplicacion` | `RetentionStore` |

`Cita` persiste además `canal`, `creada_por_usuario_id`, `derivacion_id` y `motivo_cancelacion`; el portal fuerza `canal=portal_paciente` y el actor como creador. El adaptador de correo de desarrollo (`DevEmailSender`) no es una entidad de base: escribe archivos `.eml` en `EMAIL_OUTBOX_DIR` detrás del puerto `service.EmailSender`. El resto del modelo (historia clínica, hospitalización, farmacia, facturación) puede agregarse replicando el patrón: entidad en `service/entities`, puerto en `service/ports.go`, caso de uso en `service`, almacén en `repository/postgres` y handler en `transport/http`.

## Reglas de negocio implementadas

1. Paciente y médico se crean junto con su `persona` en una **transacción**.
2. Documento de persona y número de licencia son **únicos** a nivel de base de datos.
3. Citas con el mismo médico o el mismo paciente no pueden **solaparse**; la garantía se aplica con restricciones de exclusión de PostgreSQL sobre las citas activas, incluso bajo solicitudes simultáneas.
4. La fecha de una cita nueva no puede estar en el pasado.
5. Las transiciones de estado se validan en el dominio, no en el handler.
6. Las citas se **cancelan**, no se eliminan; los pacientes y médicos tienen **baja lógica**.
7. Los errores SQL (`unique_violation`, `foreign_key_violation`, `check_violation`, longitud excedida, nulabilidad y formato inválido) se traducen a errores de dominio.
8. `updated_at` se actualiza automáticamente mediante triggers de PostgreSQL.
9. Un usuario bloqueado no puede iniciar sesión hasta que expire el bloqueo o un administrador lo desbloquee.
10. Los slots del portal se calculan desde horarios activos y vigentes, se convierten desde `CLINIC_TIMEZONE` a UTC y descartan bloqueos, citas activas, horarios pasados y fechas fuera de la antelación/horizonte.
11. Una especialidad nueva exige derivación por defecto; el catálogo se actualiza con `requiere_derivacion`, `activa`, `orden` y `duracion_cita_min`.
12. Un horario o bloqueo solo se modifica con `version` vigente y no puede solaparse con otro activo del mismo médico.
13. El paciente solo agenda especialidades con `requiere_derivacion=false`; el servidor fuerza su `paciente_id`, el canal `portal_paciente` y la duración del slot.
14. La reprogramación de portal exige `version`, no cambia paciente/médico y vuelve a validar el slot; la cancelación respeta `PORTAL_CANCEL_CUTOFF_HOURS`.
15. Dos pacientes que compiten por el mismo slot obtienen una sola cita y un `409` gracias a las restricciones de exclusión de PostgreSQL.
10. El refresh token se guarda como hash SHA-256; nunca en texto plano.
11. La reutilización de un refresh token revocado revoca todas las sesiones del usuario.
12. Cada access token incluye la versión de sesión (`ver`); si no coincide con `usuarios.token_version`, se rechaza aunque la firma sea válida.
13. Las actualizaciones concurrentes de especialidades, pacientes, médicos, citas y usuarios se detectan con `version`; la segunda escritura devuelve `409` y no sobrescribe la primera.
14. El contador de intentos fallidos, la rotación de refresh tokens y la revocación de sesiones se ejecutan con transacciones y bloqueos de fila para que dos solicitudes simultáneas no se pisen.
15. El registro público valida username, email, documento, edad mínima configurable, política de contraseña y la versión vigente de términos antes de escribir.
16. Una cuenta formada solo por el rol `paciente` requiere `email_verificado_en` para login y refresh. Las cuentas internas existentes quedan exentas.
17. Los tokens de verificación y reset se guardan hasheados, vencen y son de un solo uso. Reenviar un enlace invalida el anterior del mismo tipo.
18. El reset de contraseña verifica el email si faltaba, limpia el bloqueo, incrementa `token_version` y revoca todos los refresh tokens en la misma transacción.
19. Si el documento ya existe como paciente sin cuenta, el registro crea una `solicitud_vinculacion`; no crea persona ni paciente nuevos. La aprobación vincula la cuenta en una transacción y el rechazo desactiva la cuenta pendiente.
20. Una cuenta y una historia clínica tienen como máximo un vínculo activo. La cuenta de médico se crea por administración y se vincula con un perfil existente mediante un permiso administrativo.

## Seguridad implementada

- **Contraseñas:** bcrypt con costo 12; mínimo 12 caracteres con mayúscula, minúscula, número y símbolo; máximo 72 bytes.
- **JWT:** HS256, secreto de al menos 32 caracteres, emisor validado, expiración obligatoria y algoritmo restringido.
- **Tokens:** access token de 15 minutos; refresh token de 7 días, rotativo, almacenado como hash y con detección de reutilización. La rotación consume el token anterior y crea el nuevo en una sola transacción.
- **Fuerza bruta:** bloqueo de cuenta tras 5 intentos fallidos durante 15 minutos; el contador se incrementa de forma atómica en PostgreSQL y el límite de 30 solicitudes de login por minuto e IP se aplica en memoria por instancia.
- **Enumeración de usuarios:** misma respuesta para usuario inexistente o contraseña incorrecta; comparación bcrypt ficticia para igualar tiempos. El reenvío de verificación y la solicitud de reset responden de forma genérica aunque el email no exista.
- **Registro público:** rate limit por IP, validación de edad, términos, contraseña y documento, y transacción única para evitar historias huérfanas. La cuenta queda inutilizable para login hasta verificar el email.
- **Tokens de cuenta:** verificación y reset se guardan solo como SHA-256, con expiración y `usado_en`; el consumo es un `UPDATE ... WHERE usado_en IS NULL AND expira_en > $2 RETURNING`, por lo que dos solicitudes simultáneas no pueden usar el mismo token. Los intentos con token inválido/vencido se auditan sin asociarlos a una cuenta y están sujetos al rate limit.
- **Recuperación:** el reset incrementa `token_version`, revoca refresh tokens y limpia bloqueos; los access tokens anteriores dejan de ser válidos de inmediato.
- **Vinculación:** una historia existente no se duplica; la aprobación valida paciente, cuenta y solicitud bajo bloqueo de fila. La aprobación/rechazo requiere `cuentas:vincular` y queda auditada.
- **Correo de desarrollo:** escribe archivos `.eml` con permisos restrictivos y nunca registra el token; el enlace solo está en el buzón local. No uses `EMAIL_PROVIDER=outbox` en producción.
- **RBAC:** permisos por recurso y acción, verificados en el middleware antes de llegar al handler.
- **Auditoría:** logins, bloqueos, refresh, cambios de contraseña, escrituras y accesos denegados.
- **Datos en tránsito y caché:** `Cache-Control: no-store`, `Pragma: no-cache`, CSP, `X-Frame-Options: DENY`, `nosniff` y política de permisos restrictiva.
- **Entradas:** límite de cuerpo JSON de 1 MiB, `DisallowUnknownFields`, consultas SQL parametrizadas y traducción de errores de longitud/formato a `400`.
- **Secretos:** `.env` está en `.gitignore`; usa `.env.example` como plantilla.
- **Sesiones:** al desactivar una cuenta, cambiar roles, cambiar contraseña o llamar a `revocar-sesiones` se incrementa `token_version`; el access token anterior falla de inmediato y los refresh tokens quedan revocados.
- **Observabilidad:** logs JSON, W3C `traceparent`/`X-Request-ID`, `trace_id` en auditoría, tabla `errores_aplicacion` con `instancia`, retención automática y endpoints restringidos `GET /errores` y `GET /auditoria`.
- **Concurrencia:** restricciones de exclusión para citas, horarios recurrentes y bloqueos de agenda; versiones optimistas para escrituras, transacciones para participantes y bloqueos de fila para autenticación; los conflictos de concurrencia se devuelven como `409` reintentables.

### Límites y recomendaciones para producción

- El middleware de autenticación consulta `usuarios.token_version` en cada request protegido para lograr revocación inmediata. Es una consulta indexada por clave primaria; si el volumen lo exige, evalúa una caché con invalidación explícita, nunca con TTL largo.
- La API no implementa TLS por sí misma: publícala detrás de un reverse proxy con HTTPS.
- Para desarrollo se usa el usuario `postgres`. En producción crea un rol de base de datos con privilegios mínimos (solo DML sobre las tablas necesarias) y no uses superusuario.
- No hay CORS configurado. Si la API se consume desde un frontend en otro origen, agrega un middleware con una lista blanca de orígenes.
- No hay MFA ni cifrado de campos a nivel de aplicación. Evalúa `pgcrypto` o cifrado de disco según el marco regulatorio aplicable.
- La recuperación de contraseña usa un buzón local de desarrollo. Antes de producción implementa un proveedor real detrás de `service.EmailSender`, configura SPF/DKIM/DMARC y protege el buzón o la cola de envío.
- El rate limiter de registro, verificación, reenvío, recuperación y reset también está en memoria y es por instancia; con varias réplicas usa un limitador compartido.
- La aprobación de vinculación depende de que recepción valide la identidad por un canal aparte. El sistema evita duplicar la historia, pero no sustituye la verificación documental humana.
- El solapamiento de agenda ya está garantizado en PostgreSQL con restricciones de exclusión para médico y paciente. Requiere la extensión `btree_gist` y que las citas históricas no tengan solapes al aplicar la migración `008`; usa `scripts/solapes_citas.sql` para revisarlas.
- El rate limiter de login/refresh, registro, verificación, reenvío, recuperación y reset está en memoria y es por instancia. Si ejecutas varias réplicas, usa un limitador compartido (Redis, API gateway o WAF) para que el límite sea global.
- Los clientes deben serializar la renovación del refresh token. Un refresh concurrente con el mismo token se interpreta como reutilización y cierra todas las sesiones del usuario.
- Las versiones optimistas hacen que dos escrituras simultáneas no se sobrescriban: el cliente que recibe `409` debe volver a consultar el recurso y reintentar con la versión actual.
- La retención de `auditoria` y `errores_aplicacion` es automática; revisa sus valores antes de producción. Programa también la limpieza de `refresh_tokens` expirados. Para alto volumen, particiona `errores_aplicacion` por fecha o envíalos solo a un agregador externo.


## Contribuyentes y asistencia

<table>
  <tr>
    <td align="center">
      <a href="https://github.com/jaretmacias25-a11y">
        <img src="https://github.com/jaretmacias25-a11y.png?size=96" width="80" height="80" alt="Jareth Macias" />
        <br />
        <sub><b>Jareth Macias</b></sub>
      </a>
      <br />
      <sub>Autor</sub>
    </td>
    <td align="center">
      <a href="https://github.com/openai/codex">
        <img src="https://github.com/openai.png?size=96" width="80" height="80" alt="Codex" />
        <br />
        <sub><b>Codex</b></sub>
      </a>
      <br />
      <sub>Asistencia de IA</sub>
    </td>
    <td align="center">
      <a href="https://github.com/deepseek-ai">
        <img src="https://github.com/deepseek-ai.png?size=96" width="80" height="80" alt="DeepSeek" />
        <br />
        <sub><b>DeepSeek</b></sub>
      </a>
      <br />
      <sub>Asistencia de IA</sub>
    </td>
    <td align="center">
      <a href="https://github.com/anomalyco/opencode">
        <img src="https://github.com/anomalyco.png?size=96" width="80" height="80" alt="OpenCode" />
        <br />
        <sub><b>OpenCode</b></sub>
      </a>
      <br />
      <sub>Asistencia de IA</sub>
    </td>
  </tr>
</table>
