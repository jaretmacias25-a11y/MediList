BEGIN;

-- ---------------------------------------------------------------------------
-- Fase 6: transferencia interna de pacientes
--
-- El médico responsable es una asignación operativa distinta de las citas.
-- El historial permite auditar quién reasignó, desde qué médico y por qué.
-- La transferencia es interna: no crea usuarios ni médicos externos.
-- ---------------------------------------------------------------------------

ALTER TABLE pacientes
    ADD COLUMN IF NOT EXISTS medico_responsable_id BIGINT
        REFERENCES medicos (id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS pacientes_medico_responsable_idx
    ON pacientes (medico_responsable_id)
    WHERE medico_responsable_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS transferencias_paciente (
    id                     BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    paciente_id            BIGINT NOT NULL REFERENCES pacientes (id) ON DELETE CASCADE,
    medico_anterior_id     BIGINT REFERENCES medicos (id) ON DELETE SET NULL,
    medico_nuevo_id        BIGINT NOT NULL REFERENCES medicos (id) ON DELETE RESTRICT,
    motivo                 TEXT NOT NULL,
    creada_por_usuario_id  BIGINT REFERENCES usuarios (id) ON DELETE SET NULL,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT transferencias_paciente_medico_distinto_ck
        CHECK (medico_anterior_id IS NULL OR medico_anterior_id <> medico_nuevo_id)
);

CREATE INDEX IF NOT EXISTS transferencias_paciente_paciente_idx
    ON transferencias_paciente (paciente_id, created_at DESC);

COMMENT ON COLUMN pacientes.medico_responsable_id IS
    'Médico tratante actual asignado por administración/recepción.';
COMMENT ON TABLE transferencias_paciente IS
    'Historial inmutable de transferencias internas de médico tratante.';

-- Permiso administrativo para reasignar el médico responsable.
INSERT INTO permisos (codigo, modulo, descripcion) VALUES
    ('pacientes:transferir', 'pacientes', 'Transferir un paciente a otro médico tratante')
ON CONFLICT (codigo) DO UPDATE
SET modulo = EXCLUDED.modulo,
    descripcion = EXCLUDED.descripcion;

INSERT INTO rol_permisos (rol_id, permiso_id)
SELECT r.id, p.id
FROM roles r
JOIN permisos p ON p.codigo = 'pacientes:transferir'
WHERE r.codigo IN ('admin', 'recepcion')
ON CONFLICT DO NOTHING;

COMMIT;
