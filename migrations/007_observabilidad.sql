BEGIN;

-- ---------------------------------------------------------------------------
-- Observabilidad: instancia del error y correlación de auditoría
-- ---------------------------------------------------------------------------

-- Permite distinguir réplicas cuando el servicio se ejecuta varias veces.
ALTER TABLE errores_aplicacion
    ADD COLUMN IF NOT EXISTS instancia VARCHAR(120) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_errores_instancia
    ON errores_aplicacion (instancia, created_at DESC);

-- Permite cruzar un evento de seguridad con los logs y errores de la misma
-- solicitud usando el mismo trace_id.
ALTER TABLE auditoria
    ADD COLUMN IF NOT EXISTS trace_id VARCHAR(64) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_auditoria_trace
    ON auditoria (trace_id);

COMMIT;
