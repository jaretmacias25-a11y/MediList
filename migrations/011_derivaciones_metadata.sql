BEGIN;

-- ---------------------------------------------------------------------------
-- Derivaciones y metadata de citas
--
-- 1. La derivación nace de una cita de Medicina General atendida.
-- 2. La especialidad destino debe exigir derivación.
-- 3. La base refuerza las reglas críticas; el servicio seguirá siendo la
--    fuente principal de la lógica transaccional.
-- 4. Una derivación no puede sostener dos citas activas o atendidas.
-- 5. La nota mínima pertenece a una cita y solo puede escribirla su médico.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- Derivaciones
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS derivaciones (
    id                      BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    paciente_id             BIGINT NOT NULL REFERENCES pacientes (id) ON DELETE RESTRICT,
    medico_origen_id        BIGINT NOT NULL REFERENCES medicos (id) ON DELETE RESTRICT,
    cita_origen_id          BIGINT NOT NULL REFERENCES citas (id) ON DELETE RESTRICT,
    especialidad_destino_id BIGINT NOT NULL REFERENCES especialidades (id) ON DELETE RESTRICT,
    medico_destino_id       BIGINT REFERENCES medicos (id) ON DELETE SET NULL,
    motivo                  TEXT NOT NULL,
    prioridad               VARCHAR(15) NOT NULL DEFAULT 'normal'
        CHECK (prioridad IN ('normal', 'preferente', 'urgente')),
    estado                  VARCHAR(15) NOT NULL DEFAULT 'pendiente'
        CHECK (estado IN ('pendiente', 'agendada', 'atendida', 'vencida', 'cancelada')),
    vigente_desde           TIMESTAMPTZ NOT NULL DEFAULT now(),
    vigente_hasta           TIMESTAMPTZ NOT NULL,
    motivo_cancelacion      TEXT NOT NULL DEFAULT '',
    version                 BIGINT NOT NULL DEFAULT 1,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT derivaciones_vigencia_ck
        CHECK (vigente_hasta > vigente_desde)
);

CREATE INDEX IF NOT EXISTS derivaciones_paciente_idx
    ON derivaciones (paciente_id, estado, vigente_hasta);

CREATE INDEX IF NOT EXISTS derivaciones_origen_idx
    ON derivaciones (cita_origen_id);

CREATE INDEX IF NOT EXISTS derivaciones_destino_idx
    ON derivaciones (especialidad_destino_id, estado);

CREATE INDEX IF NOT EXISTS derivaciones_medico_destino_idx
    ON derivaciones (medico_destino_id)
    WHERE medico_destino_id IS NOT NULL;

-- Defensa en profundidad: valida el origen clínico y el destino antes de
-- aceptar una derivación. El servicio debe seguir validando la regla completa
-- dentro de su transacción.
CREATE OR REPLACE FUNCTION validar_derivacion_consistente()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    v_cita_paciente           BIGINT;
    v_cita_medico             BIGINT;
    v_cita_estado             VARCHAR(20);
    v_origen_codigo           VARCHAR(20);
    v_destino_requiere        BOOLEAN;
    v_destino_medico_esp      BIGINT;
BEGIN
    SELECT c.paciente_id, c.medico_id, c.estado, e.codigo
      INTO v_cita_paciente, v_cita_medico, v_cita_estado, v_origen_codigo
      FROM citas c
      JOIN medicos m ON m.id = c.medico_id
      JOIN especialidades e ON e.id = m.especialidad_id
     WHERE c.id = NEW.cita_origen_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'la cita de origen % no existe', NEW.cita_origen_id
            USING ERRCODE = '23514';
    END IF;

    IF v_cita_paciente <> NEW.paciente_id THEN
        RAISE EXCEPTION 'la cita de origen no pertenece al paciente de la derivación'
            USING ERRCODE = '23514';
    END IF;

    IF v_cita_medico <> NEW.medico_origen_id THEN
        RAISE EXCEPTION 'el médico de origen no es el médico de la cita de origen'
            USING ERRCODE = '23514';
    END IF;

    IF v_cita_estado <> 'atendida' THEN
        RAISE EXCEPTION 'la cita de origen debe estar atendida (estado actual: %)', v_cita_estado
            USING ERRCODE = '23514';
    END IF;

    -- Medicina General es la puerta de entrada clínica. El acceso directo del
    -- catálogo se controla con especialidades.requiere_derivacion; este trigger
    -- solo valida el origen de la derivación.
    IF v_origen_codigo <> 'MED-GEN' THEN
        RAISE EXCEPTION 'la derivación solo puede originarse en Medicina General'
            USING ERRCODE = '23514';
    END IF;

    SELECT requiere_derivacion
      INTO v_destino_requiere
      FROM especialidades
     WHERE id = NEW.especialidad_destino_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'la especialidad destino % no existe', NEW.especialidad_destino_id
            USING ERRCODE = '23514';
    END IF;

    IF NOT v_destino_requiere THEN
        RAISE EXCEPTION 'la especialidad destino no requiere derivación'
            USING ERRCODE = '23514';
    END IF;

    IF NEW.medico_destino_id IS NOT NULL THEN
        SELECT especialidad_id
          INTO v_destino_medico_esp
          FROM medicos
         WHERE id = NEW.medico_destino_id;

        IF NOT FOUND THEN
            RAISE EXCEPTION 'el médico destino % no existe', NEW.medico_destino_id
                USING ERRCODE = '23514';
        END IF;

        IF v_destino_medico_esp <> NEW.especialidad_destino_id THEN
            RAISE EXCEPTION 'el médico destino no pertenece a la especialidad destino'
                USING ERRCODE = '23514';
        END IF;
    END IF;

    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_derivaciones_consistencia ON derivaciones;
CREATE TRIGGER trg_derivaciones_consistencia
    BEFORE INSERT OR UPDATE ON derivaciones
    FOR EACH ROW EXECUTE FUNCTION validar_derivacion_consistente();

-- ---------------------------------------------------------------------------
-- Metadata de citas
-- ---------------------------------------------------------------------------

ALTER TABLE citas
    ADD COLUMN IF NOT EXISTS derivacion_id BIGINT
        REFERENCES derivaciones (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS canal VARCHAR(20) NOT NULL DEFAULT 'recepcion'
        CHECK (canal IN ('recepcion', 'portal_paciente', 'medico', 'admin', 'legacy')),
    ADD COLUMN IF NOT EXISTS creada_por_usuario_id BIGINT
        REFERENCES usuarios (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS motivo_cancelacion TEXT NOT NULL DEFAULT '';

COMMENT ON COLUMN citas.canal IS
    'Canal de origen de la cita. Las citas previas a la migración 011 quedan como recepcion.';

-- Una derivación solo puede sostener una cita activa o atendida. Las citas
-- canceladas, no asistidas o reprogramadas liberan la derivación.
CREATE UNIQUE INDEX IF NOT EXISTS citas_derivacion_activa_uk
    ON citas (derivacion_id)
    WHERE derivacion_id IS NOT NULL
      AND estado IN ('programada', 'confirmada', 'atendida');

CREATE INDEX IF NOT EXISTS citas_derivacion_idx
    ON citas (derivacion_id)
    WHERE derivacion_id IS NOT NULL;

-- Valida que la cita y la derivación pertenezcan al mismo paciente y que el
-- médico tenga la especialidad destino. La vigencia y el consumo se validan
-- en el servicio; aquí se cierra la ventana de datos incoherentes.
CREATE OR REPLACE FUNCTION validar_cita_derivacion()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    v_paciente_id      BIGINT;
    v_especialidad_id  BIGINT;
    v_estado           VARCHAR(15);
    v_vigente_hasta    TIMESTAMPTZ;
BEGIN
    IF NEW.derivacion_id IS NULL THEN
        RETURN NEW;
    END IF;

    SELECT paciente_id, especialidad_destino_id, estado, vigente_hasta
      INTO v_paciente_id, v_especialidad_id, v_estado, v_vigente_hasta
      FROM derivaciones
     WHERE id = NEW.derivacion_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'la derivación % no existe', NEW.derivacion_id
            USING ERRCODE = '23514';
    END IF;

    IF v_paciente_id <> NEW.paciente_id THEN
        RAISE EXCEPTION 'la derivación no pertenece al paciente de la cita'
            USING ERRCODE = '23514';
    END IF;

    IF v_estado NOT IN ('pendiente', 'agendada') THEN
        RAISE EXCEPTION 'la derivación no está disponible para agendar (estado: %)', v_estado
            USING ERRCODE = '23514';
    END IF;

    IF v_vigente_hasta <= now() THEN
        RAISE EXCEPTION 'la derivación está vencida'
            USING ERRCODE = '23514';
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM medicos m
        WHERE m.id = NEW.medico_id
          AND m.especialidad_id = v_especialidad_id
    ) THEN
        RAISE EXCEPTION 'el médico de la cita no pertenece a la especialidad de la derivación'
            USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_citas_derivacion_consistencia ON citas;
CREATE TRIGGER trg_citas_derivacion_consistencia
    BEFORE INSERT OR UPDATE OF derivacion_id, paciente_id, medico_id ON citas
    FOR EACH ROW EXECUTE FUNCTION validar_cita_derivacion();

-- ---------------------------------------------------------------------------
-- Nota mínima de consulta
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS notas_cita (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    cita_id     BIGINT NOT NULL UNIQUE REFERENCES citas (id) ON DELETE CASCADE,
    medico_id   BIGINT NOT NULL REFERENCES medicos (id) ON DELETE RESTRICT,
    motivo      TEXT NOT NULL DEFAULT '',
    evaluacion  TEXT NOT NULL DEFAULT '',
    plan        TEXT NOT NULL DEFAULT '',
    version     BIGINT NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS notas_cita_medico_idx
    ON notas_cita (medico_id);

CREATE OR REPLACE FUNCTION validar_nota_cita()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM citas c
        WHERE c.id = NEW.cita_id
          AND c.medico_id = NEW.medico_id
    ) THEN
        RAISE EXCEPTION 'la nota debe pertenecer al médico de la cita'
            USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_notas_cita_medico ON notas_cita;
CREATE TRIGGER trg_notas_cita_medico
    BEFORE INSERT OR UPDATE ON notas_cita
    FOR EACH ROW EXECUTE FUNCTION validar_nota_cita();

-- ---------------------------------------------------------------------------
-- Triggers de updated_at
-- ---------------------------------------------------------------------------

DROP TRIGGER IF EXISTS trg_derivaciones_updated_at ON derivaciones;
CREATE TRIGGER trg_derivaciones_updated_at
    BEFORE UPDATE ON derivaciones
    FOR EACH ROW EXECUTE FUNCTION establecer_updated_at();

DROP TRIGGER IF EXISTS trg_notas_cita_updated_at ON notas_cita;
CREATE TRIGGER trg_notas_cita_updated_at
    BEFORE UPDATE ON notas_cita
    FOR EACH ROW EXECUTE FUNCTION establecer_updated_at();

COMMENT ON TABLE derivaciones IS
    'Derivaciones emitidas desde una cita de Medicina General atendida.';
COMMENT ON TABLE notas_cita IS
    'Nota mínima de consulta; una por cita y escrita por el médico de la cita.';

COMMIT;
