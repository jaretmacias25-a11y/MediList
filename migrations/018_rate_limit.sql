BEGIN;

-- ---------------------------------------------------------------------------
-- Fase 9: rate limiting distribuido
--
-- La ventana fija se comparte entre instancias mediante PostgreSQL. La clave
-- incluye el ámbito (login, registro, reserva, etc.) y el identificador de
-- cliente. No se guardan IPs en claro más allá de la ventana operativa.
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS rate_limit_buckets (
    clave          VARCHAR(220) PRIMARY KEY,
    contador       INTEGER NOT NULL DEFAULT 0,
    ventana_inicio TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS rate_limit_buckets_ventana_idx
    ON rate_limit_buckets (ventana_inicio);

CREATE INDEX IF NOT EXISTS rate_limit_buckets_updated_idx
    ON rate_limit_buckets (updated_at);

COMMENT ON TABLE rate_limit_buckets IS
    'Ventanas fijas de rate limiting compartidas entre réplicas de la API.';

COMMIT;
