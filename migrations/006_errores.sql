BEGIN;

-- ---------------------------------------------------------------------------
-- Errores agregados de la aplicación
-- Centraliza errores para correlacionarlos con logs estructurados por trace_id.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS errores_aplicacion (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    trace_id     VARCHAR(64)  NOT NULL,
    servicio     VARCHAR(80)  NOT NULL,
    componente   VARCHAR(40)  NOT NULL DEFAULT '',
    nivel        VARCHAR(20)  NOT NULL DEFAULT 'error',
    metodo       VARCHAR(10)  NOT NULL DEFAULT '',
    ruta         TEXT         NOT NULL DEFAULT '',
    estado_http  INTEGER      NOT NULL DEFAULT 0,
    codigo       VARCHAR(80)  NOT NULL DEFAULT '',
    mensaje      TEXT         NOT NULL DEFAULT '',
    usuario_id   BIGINT REFERENCES usuarios (id) ON DELETE SET NULL,
    username     VARCHAR(50)  NOT NULL DEFAULT '',
    ip           VARCHAR(45)  NOT NULL DEFAULT '',
    user_agent   TEXT         NOT NULL DEFAULT '',
    stack_trace  TEXT         NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_errores_fecha    ON errores_aplicacion (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_errores_trace    ON errores_aplicacion (trace_id);
CREATE INDEX IF NOT EXISTS idx_errores_nivel    ON errores_aplicacion (nivel, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_errores_servicio ON errores_aplicacion (servicio, created_at DESC);

-- Permiso de consulta para administradores y auditores.
INSERT INTO permisos (codigo, modulo, descripcion) VALUES
    ('errores:leer', 'observabilidad', 'Consultar errores agregados de la aplicación')
ON CONFLICT (codigo) DO UPDATE
SET modulo = EXCLUDED.modulo,
    descripcion = EXCLUDED.descripcion;

INSERT INTO rol_permisos (rol_id, permiso_id)
SELECT r.id, p.id
FROM roles r
JOIN permisos p ON p.codigo = 'errores:leer'
WHERE r.codigo IN ('admin', 'auditor')
ON CONFLICT DO NOTHING;

COMMIT;
