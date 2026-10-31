BEGIN;

-- ---------------------------------------------------------------------------
-- Fase 2: vinculación de cuentas con historias clínicas existentes.
--
-- El registro público nunca duplica una persona/paciente. Cuando el documento
-- ya existe sin cuenta, se crea una cuenta pendiente y una solicitud que
-- recepción/administración debe aprobar después de verificar la identidad.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS solicitudes_vinculacion (
    id                        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    usuario_id                BIGINT NOT NULL REFERENCES usuarios (id) ON DELETE CASCADE,
    paciente_id               BIGINT NOT NULL REFERENCES pacientes (id) ON DELETE CASCADE,
    estado                    VARCHAR(20) NOT NULL DEFAULT 'pendiente'
        CHECK (estado IN ('pendiente', 'aprobada', 'rechazada')),
    motivo                    TEXT NOT NULL DEFAULT '',
    resuelta_por_usuario_id   BIGINT REFERENCES usuarios (id) ON DELETE SET NULL,
    resuelta_en               TIMESTAMPTZ,
    created_at                TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Una historia clínica y una cuenta solo pueden tener una solicitud pendiente.
-- Las solicitudes resueltas quedan como historial y no bloquean una nueva.
CREATE UNIQUE INDEX IF NOT EXISTS solicitudes_vinculacion_paciente_pendiente_uk
    ON solicitudes_vinculacion (paciente_id)
    WHERE estado = 'pendiente';

CREATE UNIQUE INDEX IF NOT EXISTS solicitudes_vinculacion_usuario_pendiente_uk
    ON solicitudes_vinculacion (usuario_id)
    WHERE estado = 'pendiente';

CREATE INDEX IF NOT EXISTS solicitudes_vinculacion_estado_idx
    ON solicitudes_vinculacion (estado, created_at DESC);

DROP TRIGGER IF EXISTS trg_solicitudes_vinculacion_updated_at ON solicitudes_vinculacion;
CREATE TRIGGER trg_solicitudes_vinculacion_updated_at
    BEFORE UPDATE ON solicitudes_vinculacion
    FOR EACH ROW EXECUTE FUNCTION establecer_updated_at();

COMMIT;
