# Runbook de operación y seguridad — Fase 9

Este documento cubre el despliegue endurecido de MediList: TLS en proxy inverso,
CORS, secretos, rate limiting distribuido, métricas, alertas, retención,
backups y respuesta a incidentes. La API sigue siendo la misma aplicación Go;
el proxy y PostgreSQL son parte del perímetro operativo.

## 1. Topología recomendada

```text
Internet
   │  HTTPS 443
   ▼
Proxy inverso (Nginx/Caddy/ALB) ── HTTP 8080 ──► API MediList (1..N réplicas)
                                                     │
                                                     ▼
                                              PostgreSQL 15+
                                                     │
                                      ┌──────────────┴──────────────┐
                                      ▼                             ▼
                              Loki/Promtail (opcional)       SMTP/outbox
```

- La API no termina TLS. El proxy debe redirigir HTTP a HTTPS y añadir HSTS.
- La API solo debe escuchar en la red privada o en `127.0.0.1` cuando el proxy
  está en la misma máquina.
- Si hay varias réplicas, todas comparten PostgreSQL; el rate limiting y el
  outbox son globales.
- `GET /metrics` debe quedar accesible solo para el recolector de métricas
  (red privada, security group o allowlist del proxy).

## 2. Secretos y configuración

Los secretos viven en variables de entorno o en un gestor de secretos, nunca en
el repositorio. `.env` está ignorado por Git; usa `.env.example` como plantilla.

Variables críticas de producción:

```env
APP_ENV=production
JWT_SECRET=<aleatorio, mínimo 32 caracteres, rotado y respaldado>
DATABASE_URL=postgres://medilist_app:<password>@db:5432/medilist?sslmode=require
EMAIL_PROVIDER=smtp
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USER=<usuario>
SMTP_PASSWORD=<secreto del gestor>
CORS_ALLOWED_ORIGINS=https://app.example.com
TRUSTED_PROXIES=10.0.0.0/8,172.16.0.0/12
HSTS_ENABLED=true
METRICS_ENABLED=true
RATE_LIMIT_ENABLED=true
PORTAL_PACIENTE_ENABLED=false
DERIVACIONES_ENABLED=false
TRANSFERENCIAS_PACIENTE_ENABLED=false
```

Reglas:

- No uses el usuario `postgres` de superusuario para la aplicación. Crea un rol
  con DML mínimo sobre las tablas y permisos de ejecución de funciones.
- No uses `EMAIL_PROVIDER=outbox` en producción; es un buzón de desarrollo.
- Rota `JWT_SECRET` con un procedimiento que invalide sesiones de forma
  controlada. Un cambio de secreto invalida todos los access/refresh tokens.
- No publiques `SMTP_PASSWORD`, certificados, llaves privadas ni `.env` en
  imágenes Docker, logs o tickets. Añade `*.pem`, `*.key` y `.env*` a los
  controles de escaneo de secretos.
- `TRUSTED_PROXIES` solo debe contener las redes del proxy. Si queda vacío,
  `X-Forwarded-For` se ignora y el rate limit usa `RemoteAddr`; esto es seguro
  pero hará que todos los clientes detrás de un proxy compartan la IP del proxy.
- Las flags de portal, derivaciones y transferencias se activan después de los
  smoke tests; una ruta apagada responde `503 feature_disabled`.

## 3. TLS, HSTS y proxy inverso

Ejemplo mínimo de Nginx:

```nginx
server {
    listen 80;
    server_name api.example.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name api.example.com;

    ssl_certificate     /etc/letsencrypt/live/api.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/api.example.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers off;

    client_max_body_size 1m;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto https;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_read_timeout 60s;
    }
}
```

- Con el proxy local, configura `TRUSTED_PROXIES=127.0.0.1/32`.
- `HSTS_ENABLED=true` añade `Strict-Transport-Security: max-age=31536000;
  includeSubDomains`. Actívalo solo cuando todos los subdominios sean HTTPS.
- La API ya envía `X-Content-Type-Options`, `X-Frame-Options`, CSP,
  `Referrer-Policy`, `Permissions-Policy`, `Cross-Origin-Resource-Policy`,
  `Cache-Control: no-store` y `Pragma: no-cache`.
- No agregues cabeceras CORS manuales en el proxy; la aplicación aplica la
  allowlist y responde los preflight antes de autenticar.

## 4. CORS

`CORS_ALLOWED_ORIGINS` es una lista separada por comas de orígenes exactos:

```env
CORS_ALLOWED_ORIGINS=https://app.example.com,https://admin.example.com
```

- No incluyas rutas, barras finales ni comodines de subdominio.
- `*` se rechaza en producción.
- Si el frontend usa cookies, añade credenciales y CSRF en una fase posterior;
  el contrato actual usa bearer tokens.
- Un origen no listado no recibe `Access-Control-Allow-Origin`; el navegador
  bloquea la lectura aunque el endpoint responda.

## 5. Rate limiting distribuido

La migración `018_rate_limit.sql` crea `rate_limit_buckets`. La clave incluye
el ámbito y el cliente; las ventanas son fijas y atómicas:

| Ámbito | Límite | Ventana |
|---|---:|---:|
| Login | 30 | 1 minuto |
| Refresh | 60 | 1 minuto |
| Registro | 5 | 1 hora |
| Verificación | 30 | 1 minuto |
| Reenvío de verificación | 3 | 15 minutos |
| Solicitud de reset | 5 | 15 minutos |
| Reset | 10 | 15 minutos |
| Reserva de portal | 20 por actor | 1 hora |

- `RATE_LIMIT_ENABLED=false` desactiva solo la capa distribuida; el limitador
  local sigue activo.
- Si PostgreSQL falla, la capa distribuida falla abierto y registra
  `no se pudo aplicar rate limiting distribuido`. El limitador local y el
  bloqueo de cuenta siguen protegiendo.
- La tabla puede crecer; programa una limpieza de filas con
  `updated_at < now() - interval '7 days'` o inclúyela en la retención
  operativa. No borres filas activas de la ventana actual.
- Para probar el límite, repite el login con una IP de prueba y verifica el
  `429` con `Retry-After`.

## 6. Métricas y alertas

`GET /metrics` expone contadores Prometheus sin etiquetas de usuario:

```text
medilist_registros_total
medilist_verificaciones_total
medilist_reservas_total
medilist_conflictos_slot_total
medilist_derivaciones_total
medilist_conversiones_total
medilist_cancelaciones_total
medilist_fallos_email_total
medilist_notificaciones_enviadas_total
```

Alertas provisionadas en Grafana/Loki:

- solicitudes `5xx`;
- fallo al persistir errores;
- logs `level=ERROR`;
- fallo de retención;
- fallos de notificaciones/outbox;
- fallo al registrar auditoría.

Revisa el dashboard `MediList - Observabilidad` y correlaciona con
`trace_id`. Las métricas no incluyen paciente, médico, email ni documento.

## 7. Logs y privacidad

- No se registran cuerpos JSON, contraseñas, tokens, notas clínicas ni correos
  completos.
- Los logs incluyen `trace_id`, método, ruta, estado, latencia, usuario ID y,
  cuando aplica, IP/user agent para investigación.
- La tabla `auditoria` y `errores_aplicacion` se purgan según
  `AUDIT_RETENTION_DAYS` y `ERROR_RETENTION_DAYS`; `0` desactiva la purga.
- El outbox conserva el destinatario para entregar el correo; no lo expongas en
  logs ni en respuestas de API. Si tu marco regulatorio lo exige, anonimiza o
  purga notificaciones enviadas antiguas.
- No incluyas datos clínicos en tickets de soporte; usa IDs y `trace_id`.

## 8. Backups y restauración

Programa backups cifrados y fuera del host de producción:

```powershell
$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
& "C:\Program Files\PostgreSQL\18\bin\pg_dump.exe" `
  -U medilist_app -h db -Fc -f "backups\medilist-$stamp.dump" medilist
```

Restauración de prueba en una base aislada:

```powershell
& "C:\Program Files\PostgreSQL\18\bin\createdb.exe" -U postgres -h localhost medilist_restore
& "C:\Program Files\PostgreSQL\18\bin\pg_restore.exe" -U postgres -h localhost `
  -d medilist_restore --clean --if-exists "backups\medilist-YYYYMMDD-HHmmss.dump"
```

Verifica después de restaurar:

1. `GET /health` devuelve `database: up`.
2. Las migraciones `001`–`018` están aplicadas.
3. Un login de prueba y una consulta de disponibilidad funcionan.
4. El worker de outbox puede reclamar una notificación pendiente.
5. La auditoría conserva eventos recientes y la retención no borró datos fuera
   de política.

Prueba la restauración al menos una vez por trimestre. Un backup que nunca se
restauró no se considera verificado.

## 9. Orden de despliegue y rollback

1. Haz backup verificable y anota la versión actual.
2. Aplica migraciones aditivas en orden (`018_rate_limit.sql` es aditiva).
3. Despliega la API con `PORTAL_PACIENTE_ENABLED=false`,
   `DERIVACIONES_ENABLED=false` y `TRANSFERENCIAS_PACIENTE_ENABLED=false`.
4. Verifica health, `/metrics`, login administrativo y worker de outbox.
5. Activa `PORTAL_PACIENTE_ENABLED` y ejecuta el smoke test del portal.
6. Activa `DERIVACIONES_ENABLED` y ejecuta el E2E de derivaciones.
7. Activa `TRANSFERENCIAS_PACIENTE_ENABLED` cuando recepción esté capacitada.
8. Revisa logs de `level=ERROR`, alertas y outbox pendiente.

Antes de tocar staging, ensaya el flujo completo en local con
`scripts/e2e_fase10.ps1`; usa una base temporal y no requiere datos reales.

Rollback:

- La aplicación puede volver a la versión anterior; las migraciones de Fase 9
  no eliminan columnas ni datos.
- Ante un incidente, apaga primero las flags (`PORTAL_PACIENTE_ENABLED`,
  `DERIVACIONES_ENABLED`, `TRANSFERENCIAS_PACIENTE_ENABLED`) para retirar el
  tráfico sin borrar datos.
- No hagas `DROP TABLE rate_limit_buckets` durante un rollback normal; la tabla
  es compatible y evita perder la ventana compartida.
- Si una migración falla o hay corrupción, restaura el backup en una base nueva
  y cambia la cadena de conexión. No ejecutes `DROP`/`TRUNCATE` sobre datos
  clínicos en producción sin autorización y backup.

## 10. Incidentes frecuentes

### Outbox atascado o fallos de email

1. Revisa la alerta `MediList - fallos de notificaciones/outbox`.
2. Consulta `notificaciones_outbox` por estado, `intentos`,
   `proximo_intento` y `ultimo_error`.
3. Verifica conectividad y credenciales SMTP; prueba un envío controlado.
4. Si el proveedor está caído, deja el worker activo: reintenta con backoff.
5. Si hay una notificación envenenada, corrige el dato y reprograma el intento
   con una operación auditada; no borres filas a ciegas.

### Fallo de auditoría

1. La alerta es crítica: revisa PostgreSQL y permisos de la tabla `auditoria`.
2. No reinicies el servicio como única solución; identifica si es disco lleno,
   lock o falta de permisos.
3. Si la API sigue operando sin auditoría, considera desviar tráfico o detener
   escrituras clínicas hasta restaurar el control.

### Rate limiting con error de base

- La capa distribuida falla abierto para no tumbar el portal. Verifica
  PostgreSQL y los logs; el limitador local y el bloqueo de cuenta siguen
  activos. No desactives el limitador local en producción.

### Picos de `409` por slot

- Son esperados en concurrencia; revisa `medilist_conflictos_slot_total`.
- Si el pico es anómalo, revisa el cliente que reintenta sin releer la
  disponibilidad y aplica backoff.

## 11. Pruebas de carga

El script `scripts/prueba_carga_fase9.ps1` ejecuta una prueba básica de login,
disponibilidad y reserva contra un entorno de staging. Úsalo con datos de
prueba y nunca contra producción.

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\prueba_carga_fase9.ps1 `
  -BaseUrl "http://localhost:8080" `
  -Usuario "paciente.prueba" `
  -Password "Password123!" `
  -Iteraciones 20 `
  -Concurrencia 5 `
  -Reservar
```

Sin `-Reservar` solo se prueba login y disponibilidad; con el switch se
intenta reservar el mismo slot desde varios workers, por lo que los `409`
esperados también forman parte del resultado.

Criterios mínimos:

- sin `500` inesperados;
- `429` solo después del límite configurado;
- conflictos de slot devueltos como `409`, no como `500`;
- latencia de disponibilidad y reserva registrada;
- outbox sin crecimiento anómalo de fallos.

## 12. Checklist de salida de Fase 9

- [x] Rate limiting distribuido para registro, login, refresh, reset y reservas.
- [x] CORS con allowlist.
- [x] HSTS y headers de seguridad; TLS documentado en proxy.
- [x] Secretos fuera del repositorio.
- [x] Revisión de IDOR/RBAC y pruebas de ownership existentes.
- [x] Auditoría de acciones nuevas y alerta de fallos.
- [x] Redacción de PII/datos clínicos en logs.
- [x] Métricas de registros, verificaciones, reservas, conflictos,
  derivaciones, conversión, cancelaciones y fallos de email.
- [x] Alertas de outbox y auditoría.
- [x] Pruebas de carga documentadas y script base.
- [x] Retención y consentimiento documentados.
- [x] Runbook y backups documentados.
