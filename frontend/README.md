# MediList — Frontend

Skeleton técnico del frontend de MediList: React + Vite + TypeScript +
Tailwind CSS, Zustand para la sesión y guardas por permisos.

## Requisitos

- Node.js >= 20
- API de MediList ejecutándose en `http://localhost:8080`
- Especificación funcional: [`docs/especificacion-frontend.md`](../docs/especificacion-frontend.md)
- Contrato: [`docs/openapi.yaml`](../docs/openapi.yaml)

## Puesta en marcha

```powershell
cd frontend
npm install
npm run dev
```

La app queda en `http://localhost:5173`. En desarrollo, Vite reenvía `/api`
al backend local (`http://localhost:8080`), así que no hace falta CORS en el
entorno local.

## Variables de entorno

Copia `.env.example` a `.env.local`:

```env
VITE_API_BASE_URL=/api/v1
```

En producción, usa la URL absoluta de la API:

```env
VITE_API_BASE_URL=https://api.medilist.example/api/v1
```

Y configura en el backend el origen del frontend:

```env
CORS_ALLOWED_ORIGINS=https://app.medilist.example
```

## Estructura

```text
src/
  components/    # componentes compartidos (placeholders de esta iteración)
  hooks/         # useAccess: permisos y roles de la sesión
  lib/           # http.ts: cliente HTTP + refresh automático
  router/        # rutas, guardas y redirección por rol
  services/      # casos de uso de la API (auth, pacientes, citas, ...)
  stores/        # estado global (auth con Zustand)
  types/         # tipos del contrato OpenAPI
```

## Seguridad de sesión

- Los tokens viven solo en memoria: no se persisten en `localStorage`.
- Al recargar la página hay que iniciar sesión de nuevo.
- Un `401` dispara `POST /auth/refresh` una sola vez (promesa compartida)
  y reintenta la petición original.
- Si el refresh falla, la sesión se limpia y la app redirige al login.

## Guardas

Cada portal usa una regla de acceso:

| Ruta | Permiso mínimo | Vínculo requerido |
|---|---|---|
| `/paciente/*` | `portal:perfil:leer` | `paciente_id` |
| `/medico/*` | `medico:agenda:leer` | `medico_id` |
| `/recepcion/*` | `pacientes:leer` + rol `recepcion` | — |
| `/enfermeria/*` | `pacientes:leer` + rol `enfermeria` | — |
| `/admin/*` | rol `admin` | — |
| `/auditor/*` | `auditoria:leer` + rol `auditor` | — |

## Comandos

```powershell
npm run dev        # desarrollo
npm run typecheck  # verificación de tipos
npm run build      # compilación de producción
```
