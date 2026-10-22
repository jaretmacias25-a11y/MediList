# MediList - API de gestión hospitalaria

Backend en **Go** para la gestión de pacientes, médicos, especialidades y citas de una clínica. Implementa una **arquitectura limpia por capas** con persistencia en **PostgreSQL**, autenticación **JWT**, control de acceso basado en roles (**RBAC**) y auditoría de seguridad.

El modelo de dominio completo está documentado en [entidades.md](./entidades.md). Esta entrega implementa el núcleo de agenda y la infraestructura de seguridad para extender el resto de módulos.

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
│   ├── service/
│   │   ├── entities/                   # entidades del dominio
│   │   ├── ports.go                    # interfaces de repositorio
│   │   ├── auth_service.go             # login, refresh, bloqueo, cambio de clave
│   │   ├── password.go                 # bcrypt y política de contraseñas
│   │   ├── tokens.go                   # emisión y validación JWT
│   │   ├── usuario_service.go          # usuarios, roles y auditoría
│   │   └── *_service.go                # casos de uso clínicos
│   ├── repository/postgres/
│   │   ├── db.go                       # pool pgx
│   │   ├── errors.go                   # traducción de errores SQL
│   │   ├── usuario_store.go            # usuarios, roles y bloqueos
│   │   ├── refresh_token_store.go      # refresh tokens hasheados
│   │   ├── auditoria_store.go          # bitácora
│   │   └── *_store.go                  # almacenes clínicos
│   └── transport/http/
│       ├── router.go                   # rutas y health check
│       ├── auth_middleware.go          # Bearer token y permisos
│       ├── audit.go                    # auditoría automática de escrituras
│       ├── rate_limit.go               # límite de intentos por IP
│       ├── middleware.go               # logging, recovery y cabeceras
│       ├── dto.go                      # contratos JSON clínicos
│       ├── auth_dto.go                 # contratos JSON de auth/usuarios
│       └── *_handler.go                # endpoints
├── migrations/
│   ├── 001_init.sql                    # tablas clínicas, índices y triggers
│   ├── 002_seed.sql                    # especialidades iniciales
│   ├── 003_auth.sql                    # usuarios, roles, permisos, tokens y auditoría
│   └── 004_auth_seed.sql               # roles, permisos y admin inicial
├── scripts/
│   ├── migrate.ps1                     # crea BD y aplica migraciones
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

## Puesta en marcha

1. **Configurar credenciales** en `.env`:

   ```env
   APP_ENV=development
   HTTP_PORT=8080
   DATABASE_URL=postgres://postgres:TU_PASSWORD_LOCAL@localhost:5432/medilist?sslmode=disable

   JWT_SECRET=CAMBIAR_POR_UN_SECRETO_ALEATORIO_DE_AL_MENOS_32_CARACTERES
   JWT_ISSUER=medilist
   ACCESS_TOKEN_MINUTES=15
   REFRESH_TOKEN_HOURS=168
   BCRYPT_COST=12
   MAX_LOGIN_ATTEMPTS=5
   LOCK_MINUTES=15
   ```

   Reemplaza `postgres` y `TU_PASSWORD_LOCAL` por el usuario y la contraseña que tengas configurados en tu propia instalación local de PostgreSQL. No publiques contraseñas reales en el README ni en archivos versionados.

   `JWT_SECRET` debe tener al menos 32 caracteres. En producción usa un valor aleatorio y no reutilices el de desarrollo.

2. **Crear la base y aplicar migraciones**:

   ```powershell
   powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\migrate.ps1
   ```

   El script detecta `psql.exe`, crea la base `medilist` si no existe y ejecuta los archivos de `migrations/` en orden.

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

### Flujo

1. `POST /auth/login` con `username` y `password`.
2. La API devuelve un **access token** (15 min por defecto) y un **refresh token** (7 días por defecto).
3. Envía el access token en cada solicitud:

   ```http
   Authorization: Bearer <access_token>
   ```

4. Cuando el access token expire, usa `POST /auth/refresh` con el refresh token.
5. El refresh token rota en cada uso: el anterior queda revocado.
6. `POST /auth/logout` revoca el refresh token activo.

### Endpoints de autenticación

| Método | Ruta | Autenticación | Descripción |
|---|---|---|---|
| `POST` | `/auth/login` | Pública | Inicia sesión y emite tokens |
| `POST` | `/auth/refresh` | Refresh token | Rota el refresh token y emite un nuevo par |
| `POST` | `/auth/logout` | Refresh token | Revoca el refresh token |
| `GET` | `/auth/me` | Bearer | Devuelve el usuario, roles y permisos actuales |
| `POST` | `/auth/password` | Bearer | Cambia la contraseña del usuario autenticado |

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

La nueva contraseña debe tener al menos 12 caracteres e incluir mayúscula, minúscula, número y carácter especial. Al cambiarla se revocan todos los refresh tokens.

---

## Roles y permisos

La API usa RBAC. Los permisos se agregan a los roles y los roles a los usuarios. El middleware valida el permiso requerido en cada ruta; el rol `admin` tiene todos los permisos.

### Roles incluidos

| Rol | Enfoque |
|---|---|
| `admin` | Acceso total, gestión de usuarios y auditoría |
| `medico` | Consulta clínica y gestión de citas |
| `recepcion` | Gestión de pacientes y agenda |
| `enfermeria` | Consulta de pacientes, médicos y citas |
| `auditor` | Consulta de usuarios, roles y auditoría |

### Matriz de permisos

| Recurso | `admin` | `medico` | `recepcion` | `enfermeria` | `auditor` |
|---|---|---|---|---|---|
| Pacientes (leer) | Sí | Sí | Sí | Sí | No |
| Pacientes (escribir) | Sí | No | Sí | No | No |
| Médicos (leer) | Sí | Sí | Sí | Sí | No |
| Médicos (escribir) | Sí | No | No | No | No |
| Especialidades (leer) | Sí | Sí | Sí | Sí | No |
| Especialidades (escribir) | Sí | No | No | No | No |
| Citas (leer) | Sí | Sí | Sí | Sí | No |
| Citas (escribir) | Sí | Sí | Sí | No | No |
| Usuarios (leer) | Sí | No | No | No | Sí |
| Usuarios (escribir) | Sí | No | No | No | No |
| Roles (leer) | Sí | No | No | No | Sí |
| Auditoría (leer) | Sí | No | No | No | Sí |

### Administración de usuarios

Solo usuarios con `usuarios:leer` o `usuarios:escribir` pueden acceder.

| Método | Ruta | Permiso | Descripción |
|---|---|---|---|
| `GET` | `/usuarios` | `usuarios:leer` | Lista usuarios; filtros `search`, `activos=true` |
| `POST` | `/usuarios` | `usuarios:escribir` | Crea usuario con roles |
| `GET` | `/usuarios/{id}` | `usuarios:leer` | Obtiene un usuario |
| `PUT` | `/usuarios/{id}` | `usuarios:escribir` | Actualiza datos, roles, estado o contraseña |
| `POST` | `/usuarios/{id}/desbloquear` | `usuarios:escribir` | Limpia el bloqueo por intentos fallidos |
| `GET` | `/roles` | `roles:leer` | Lista roles y sus permisos |
| `GET` | `/auditoria` | `auditoria:leer` | Consulta la bitácora |

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
- Si cambian los roles, la contraseña o se desactiva la cuenta, se revocan todos los refresh tokens del usuario.
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

### Pacientes

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

### Citas

| Método | Ruta | Permiso | Descripción |
|---|---|---|---|
| `GET` | `/citas` | `citas:leer` | Filtros: `paciente_id`, `medico_id`, `estado`, `desde`, `hasta`, `search` |
| `POST` | `/citas` | `citas:escribir` | Agenda una cita y valida solape de agenda |
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

### Auditoría

| Método | Ruta | Permiso | Descripción |
|---|---|---|---|
| `GET` | `/auditoria` | `auditoria:leer` | Filtros `search`, `accion`, `usuario_id`, `limit`, `offset` |

Se registran:

- Inicios de sesión exitosos y fallidos.
- Bloqueos de cuenta y reutilización de refresh tokens.
- Cambios de contraseña.
- Operaciones de escritura autenticadas (`POST`, `PUT`, `PATCH`, `DELETE`).
- Intentos de acceso denegados por falta de permisos.

No se almacenan contraseñas, tokens ni cuerpos de solicitud en la bitácora.

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
  }
}
```

Códigos:

| Código | Significado |
|---|---|
| `400` | Validación |
| `401` | Token ausente, inválido o credenciales incorrectas |
| `403` | Sin permiso para el recurso |
| `404` | No encontrado |
| `409` | Conflicto (documento duplicado, solape de agenda) |
| `423` | Cuenta bloqueada temporalmente |
| `429` | Demasiadas solicitudes |
| `500` | Error interno |

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

## Entidades implementadas

| Entidad | Tabla | Almacén |
|---|---|---|
| Especialidad | `especialidades` | `EspecialidadStore` |
| Persona | `personas` | Gestionada dentro de pacientes y médicos |
| Paciente | `pacientes` | `PacienteStore` |
| Médico | `medicos` | `MedicoStore` |
| Cita | `citas` | `CitaStore` |
| Usuario | `usuarios` | `UsuarioStore` |
| Rol | `roles` | `RolStore` |
| Permiso | `permisos` | Catálogo RBAC |
| UsuarioRol | `usuario_roles` | `UsuarioStore` |
| RolPermiso | `rol_permisos` | `RolStore` |
| RefreshToken | `refresh_tokens` | `RefreshTokenStore` |
| Auditoría | `auditoria` | `AuditoriaStore` |

El resto del modelo (historia clínica, hospitalización, farmacia, facturación) puede agregarse replicando el patrón: entidad en `service/entities`, puerto en `service/ports.go`, caso de uso en `service`, almacén en `repository/postgres` y handler en `transport/http`.

## Reglas de negocio implementadas

1. Paciente y médico se crean junto con su `persona` en una **transacción**.
2. Documento de persona y número de licencia son **únicos** a nivel de base de datos.
3. Citas con el mismo médico no pueden **solaparse** (comparación por intervalos).
4. La fecha de una cita nueva no puede estar en el pasado.
5. Las transiciones de estado se validan en el dominio, no en el handler.
6. Las citas se **cancelan**, no se eliminan; los pacientes y médicos tienen **baja lógica**.
7. Los errores SQL (`unique_violation`, `foreign_key_violation`, `check_violation`) se traducen a errores de dominio.
8. `updated_at` se actualiza automáticamente mediante triggers de PostgreSQL.
9. Un usuario bloqueado no puede iniciar sesión hasta que expire el bloqueo o un administrador lo desbloquee.
10. El refresh token se guarda como hash SHA-256; nunca en texto plano.
11. La reutilización de un refresh token revocado revoca todas las sesiones del usuario.

## Seguridad implementada

- **Contraseñas:** bcrypt con costo 12; mínimo 12 caracteres con mayúscula, minúscula, número y símbolo; máximo 72 bytes.
- **JWT:** HS256, secreto de al menos 32 caracteres, emisor validado, expiración obligatoria y algoritmo restringido.
- **Tokens:** access token de 15 minutos; refresh token de 7 días, rotativo, almacenado como hash y con detección de reutilización.
- **Fuerza bruta:** bloqueo de cuenta tras 5 intentos fallidos durante 15 minutos; límite de 30 solicitudes de login por minuto e IP.
- **Enumeración de usuarios:** misma respuesta para usuario inexistente o contraseña incorrecta; comparación bcrypt ficticia para igualar tiempos.
- **RBAC:** permisos por recurso y acción, verificados en el middleware antes de llegar al handler.
- **Auditoría:** logins, bloqueos, refresh, cambios de contraseña, escrituras y accesos denegados.
- **Datos en tránsito y caché:** `Cache-Control: no-store`, `Pragma: no-cache`, CSP, `X-Frame-Options: DENY`, `nosniff` y política de permisos restrictiva.
- **Entradas:** límite de cuerpo JSON de 1 MiB, `DisallowUnknownFields`, consultas SQL parametrizadas.
- **Secretos:** `.env` está en `.gitignore`; usa `.env.example` como plantilla.
- **Sesiones:** al desactivar una cuenta, cambiar roles o cambiar contraseña se revocan los refresh tokens.

### Límites y recomendaciones para producción

- El access token conserva los permisos vigentes al momento de emitirse. Si cambias roles, el cambio aplica plenamente al renovar el token (máximo 15 minutos). Para revocación inmediata, agrega una versión de token verificada en base de datos.
- La API no implementa TLS por sí misma: publícala detrás de un reverse proxy con HTTPS.
- Para desarrollo se usa el usuario `postgres`. En producción crea un rol de base de datos con privilegios mínimos (solo DML sobre las tablas necesarias) y no uses superusuario.
- No hay CORS configurado. Si la API se consume desde un frontend en otro origen, agrega un middleware con una lista blanca de orígenes.
- No hay MFA, recuperación de contraseña por correo ni cifrado de campos a nivel de aplicación. Evalúa `pgcrypto` o cifrado de disco según el marco regulatorio aplicable.
- La validación de solape se hace en la capa de servicio. Para alta concurrencia, agrega una restricción de exclusión en PostgreSQL o un advisory lock por médico.
- Programa la limpieza periódica de `refresh_tokens` expirados y define retención para `auditoria`.

## Próximos pasos sugeridos

1. Módulo de historia clínica (`EncuentroAtencion`, diagnósticos, recetas).
2. Hospitalización y camas.
3. Farmacia e inventario.
4. Facturación y seguros.
5. Pruebas de integración con base de datos y pruebas de servicios con almacenes en memoria.
6. Documentación OpenAPI/Swagger.
7. Contenedores Docker y pipeline CI.
