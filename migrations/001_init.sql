BEGIN;

-- ---------------------------------------------------------------------------
-- Catálogos
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS especialidades (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    codigo      VARCHAR(20)  NOT NULL UNIQUE,
    nombre      VARCHAR(120) NOT NULL,
    descripcion TEXT         NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- Personas (datos de identidad compartidos)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS personas (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tipo_documento   VARCHAR(20)  NOT NULL,
    numero_documento VARCHAR(30)  NOT NULL,
    nombres          VARCHAR(120) NOT NULL,
    apellidos        VARCHAR(120) NOT NULL,
    fecha_nacimiento DATE         NOT NULL,
    sexo             VARCHAR(20)  NOT NULL
                     CHECK (sexo IN ('masculino', 'femenino', 'otro', 'no_especificado')),
    telefono         VARCHAR(30)  NOT NULL DEFAULT '',
    email            VARCHAR(160) NOT NULL DEFAULT '',
    direccion        VARCHAR(255) NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT personas_documento_uk UNIQUE (tipo_documento, numero_documento)
);

-- ---------------------------------------------------------------------------
-- Pacientes
-- ---------------------------------------------------------------------------
CREATE SEQUENCE IF NOT EXISTS paciente_historia_seq START WITH 1;

CREATE TABLE IF NOT EXISTS pacientes (
    id                          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    persona_id                  BIGINT      NOT NULL UNIQUE REFERENCES personas (id),
    numero_historia             VARCHAR(30) NOT NULL UNIQUE
                                DEFAULT ('HC-' || lpad(nextval('paciente_historia_seq')::text, 6, '0')),
    grupo_sanguineo             VARCHAR(5)  NOT NULL DEFAULT ''
                                CHECK (grupo_sanguineo IN ('', 'A', 'B', 'AB', 'O')),
    factor_rh                   VARCHAR(5)  NOT NULL DEFAULT ''
                                CHECK (factor_rh IN ('', '+', '-')),
    contacto_emergencia_nombre  VARCHAR(160) NOT NULL DEFAULT '',
    contacto_emergencia_telefono VARCHAR(30) NOT NULL DEFAULT '',
    activo                      BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_pacientes_persona ON pacientes (persona_id);
CREATE INDEX IF NOT EXISTS idx_pacientes_activo  ON pacientes (activo);

-- ---------------------------------------------------------------------------
-- Médicos / profesionales de la salud
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS medicos (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    persona_id       BIGINT      NOT NULL UNIQUE REFERENCES personas (id),
    especialidad_id  BIGINT      NOT NULL REFERENCES especialidades (id),
    numero_licencia  VARCHAR(40) NOT NULL UNIQUE,
    tipo_profesional VARCHAR(40) NOT NULL DEFAULT 'medico',
    activo           BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_medicos_especialidad ON medicos (especialidad_id);
CREATE INDEX IF NOT EXISTS idx_medicos_activo       ON medicos (activo);

-- ---------------------------------------------------------------------------
-- Citas
-- ---------------------------------------------------------------------------
CREATE SEQUENCE IF NOT EXISTS cita_codigo_seq START WITH 1;

CREATE TABLE IF NOT EXISTS citas (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    codigo            VARCHAR(30) NOT NULL UNIQUE
                      DEFAULT ('CIT-' || lpad(nextval('cita_codigo_seq')::text, 6, '0')),
    paciente_id       BIGINT      NOT NULL REFERENCES pacientes (id),
    medico_id         BIGINT      NOT NULL REFERENCES medicos (id),
    fecha_hora_inicio TIMESTAMPTZ NOT NULL,
    duracion_min      INTEGER     NOT NULL DEFAULT 30
                      CHECK (duracion_min BETWEEN 5 AND 480),
    tipo              VARCHAR(30) NOT NULL DEFAULT 'primera_vez'
                      CHECK (tipo IN ('primera_vez', 'control', 'urgencia', 'telemedicina')),
    motivo            TEXT        NOT NULL DEFAULT '',
    estado            VARCHAR(20) NOT NULL DEFAULT 'programada'
                      CHECK (estado IN ('programada', 'confirmada', 'atendida', 'no_asistio', 'cancelada', 'reprogramada')),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_citas_medico_inicio   ON citas (medico_id, fecha_hora_inicio);
CREATE INDEX IF NOT EXISTS idx_citas_paciente_inicio ON citas (paciente_id, fecha_hora_inicio);
CREATE INDEX IF NOT EXISTS idx_citas_estado          ON citas (estado);

-- ---------------------------------------------------------------------------
-- Trigger común para updated_at
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION establecer_updated_at()
RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_especialidades_updated_at ON especialidades;
CREATE TRIGGER trg_especialidades_updated_at
    BEFORE UPDATE ON especialidades
    FOR EACH ROW EXECUTE FUNCTION establecer_updated_at();

DROP TRIGGER IF EXISTS trg_personas_updated_at ON personas;
CREATE TRIGGER trg_personas_updated_at
    BEFORE UPDATE ON personas
    FOR EACH ROW EXECUTE FUNCTION establecer_updated_at();

DROP TRIGGER IF EXISTS trg_pacientes_updated_at ON pacientes;
CREATE TRIGGER trg_pacientes_updated_at
    BEFORE UPDATE ON pacientes
    FOR EACH ROW EXECUTE FUNCTION establecer_updated_at();

DROP TRIGGER IF EXISTS trg_medicos_updated_at ON medicos;
CREATE TRIGGER trg_medicos_updated_at
    BEFORE UPDATE ON medicos
    FOR EACH ROW EXECUTE FUNCTION establecer_updated_at();

DROP TRIGGER IF EXISTS trg_citas_updated_at ON citas;
CREATE TRIGGER trg_citas_updated_at
    BEFORE UPDATE ON citas
    FOR EACH ROW EXECUTE FUNCTION establecer_updated_at();

COMMIT;
