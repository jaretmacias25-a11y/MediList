BEGIN;

-- ---------------------------------------------------------------------------
-- Fase 7: excepciones administrativas de derivación
--
-- No se crea una derivación clínica falsa. Se registra una autorización
-- excepcional, quién la emitió y por qué, para una cita de especialista que
-- no tiene derivación. El flujo normal sigue exigiendo una derivación real.
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS excepciones_derivacion (
    id                        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    cita_id                   BIGINT NOT NULL UNIQUE REFERENCES citas (id) ON DELETE CASCADE,
    paciente_id               BIGINT NOT NULL REFERENCES pacientes (id) ON DELETE CASCADE,
    especialidad_id           BIGINT NOT NULL REFERENCES especialidades (id) ON DELETE RESTRICT,
    medico_id                 BIGINT NOT NULL REFERENCES medicos (id) ON DELETE RESTRICT,
    motivo                    TEXT NOT NULL,
    autorizado_por_usuario_id BIGINT REFERENCES usuarios (id) ON DELETE SET NULL,
    created_at                TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT excepciones_derivacion_motivo_ck CHECK (btrim(motivo) <> '')
);

CREATE INDEX IF NOT EXISTS excepciones_derivacion_paciente_idx
    ON excepciones_derivacion (paciente_id, created_at DESC);

COMMENT ON TABLE excepciones_derivacion IS
    'Excepciones administrativas auditadas para citas de especialista sin derivación.';

COMMIT;
