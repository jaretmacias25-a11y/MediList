BEGIN;

-- ---------------------------------------------------------------------------
-- Fase 8: outbox de notificaciones y preferencias
--
-- El outbox se escribe en la misma transacción que el evento de negocio.
-- El worker reclama filas pendientes con SKIP LOCKED, envía y registra el
-- resultado. La clave única evita duplicados por evento/recordatorio.
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS notificaciones_outbox (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tipo             VARCHAR(40) NOT NULL,
    usuario_id       BIGINT REFERENCES usuarios (id) ON DELETE SET NULL,
    destinatario     VARCHAR(160) NOT NULL,
    nombre           VARCHAR(120) NOT NULL DEFAULT '',
    asunto           VARCHAR(200) NOT NULL,
    cuerpo           TEXT NOT NULL,
    estado           VARCHAR(15) NOT NULL DEFAULT 'pendiente'
        CHECK (estado IN ('pendiente', 'enviando', 'enviada', 'fallida')),
    intentos         INTEGER NOT NULL DEFAULT 0,
    proximo_intento  TIMESTAMPTZ NOT NULL DEFAULT now(),
    ultimo_error     TEXT NOT NULL DEFAULT '',
    dedupe_key       VARCHAR(160) NOT NULL UNIQUE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    enviada_en       TIMESTAMPTZ
);

ALTER TABLE notificaciones_outbox
    ADD COLUMN IF NOT EXISTS usuario_id BIGINT
        REFERENCES usuarios (id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS notificaciones_outbox_pendientes_idx
    ON notificaciones_outbox (estado, proximo_intento, id);

CREATE INDEX IF NOT EXISTS notificaciones_outbox_tipo_idx
    ON notificaciones_outbox (tipo, created_at DESC);

DROP TRIGGER IF EXISTS trg_notificaciones_outbox_updated_at ON notificaciones_outbox;
CREATE TRIGGER trg_notificaciones_outbox_updated_at
    BEFORE UPDATE ON notificaciones_outbox
    FOR EACH ROW EXECUTE FUNCTION establecer_updated_at();

CREATE TABLE IF NOT EXISTS preferencias_notificacion (
    usuario_id                BIGINT PRIMARY KEY REFERENCES usuarios (id) ON DELETE CASCADE,
    email_habilitado          BOOLEAN NOT NULL DEFAULT TRUE,
    recordatorios_habilitados BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT now()
);

DROP TRIGGER IF EXISTS trg_preferencias_notificacion_updated_at ON preferencias_notificacion;
CREATE TRIGGER trg_preferencias_notificacion_updated_at
    BEFORE UPDATE ON preferencias_notificacion
    FOR EACH ROW EXECUTE FUNCTION establecer_updated_at();

COMMENT ON TABLE notificaciones_outbox IS
    'Outbox durable de correos; se escribe junto con el evento de negocio.';
COMMENT ON TABLE preferencias_notificacion IS
    'Preferencias de correo y recordatorios por cuenta.';

COMMIT;
