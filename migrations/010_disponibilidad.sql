BEGIN;

-- ---------------------------------------------------------------------------
-- Disponibilidad de agenda
--
-- 1. horarios_profesional representa la disponibilidad semanal recurrente.
--    Las horas son locales de la clínica; el servicio las convierte a UTC
--    usando CLINIC_TIMEZONE al generar slots.
-- 2. bloqueos_agenda representa ausencias puntuales en UTC.
-- 3. Las restricciones de exclusión cierran la ventana de carrera entre dos
--    solicitudes concurrentes que intentan crear horarios o bloqueos
--    incompatibles. La validación del servicio sigue siendo la primera capa.
-- ---------------------------------------------------------------------------

CREATE EXTENSION IF NOT EXISTS btree_gist;

-- Convierte una hora local a segundos desde medianoche. Se encapsula como
-- IMMUTABLE para poder usarla en el índice de exclusión de horarios.
CREATE OR REPLACE FUNCTION hora_a_segundos(hora TIME)
RETURNS INTEGER
LANGUAGE sql
IMMUTABLE
STRICT
PARALLEL SAFE
AS $$
    SELECT (
        EXTRACT(HOUR FROM hora) * 3600
        + EXTRACT(MINUTE FROM hora) * 60
        + EXTRACT(SECOND FROM hora)
    )::INTEGER
$$;

-- ---------------------------------------------------------------------------
-- Horarios recurrentes
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS horarios_profesional (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    medico_id         BIGINT NOT NULL REFERENCES medicos (id) ON DELETE CASCADE,
    dia_semana        SMALLINT NOT NULL CHECK (dia_semana BETWEEN 1 AND 7),
    hora_inicio       TIME NOT NULL,
    hora_fin          TIME NOT NULL,
    duracion_slot_min INTEGER NOT NULL DEFAULT 30
                      CHECK (duracion_slot_min BETWEEN 5 AND 240),
    vigente_desde     DATE NOT NULL DEFAULT CURRENT_DATE,
    vigente_hasta     DATE,
    activo            BOOLEAN NOT NULL DEFAULT TRUE,
    version           BIGINT NOT NULL DEFAULT 1,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT horarios_profesional_rango_ck
        CHECK (hora_fin > hora_inicio),
    CONSTRAINT horarios_profesional_vigencia_ck
        CHECK (vigente_hasta IS NULL OR vigente_hasta >= vigente_desde)
);

CREATE INDEX IF NOT EXISTS horarios_profesional_medico_idx
    ON horarios_profesional (medico_id, dia_semana, activo);

-- Dos horarios activos del mismo médico, para el mismo día, no pueden
-- solaparse en hora ni en vigencia. Los rangos son [inicio, fin) en horas y
-- [vigente_desde, vigente_hasta] en fechas; vigente_hasta NULL es infinito.
DO $$
DECLARE
    v_horario_a BIGINT;
    v_horario_b BIGINT;
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'horarios_profesional_sin_solape'
          AND conrelid = 'horarios_profesional'::regclass
    ) THEN
        SELECT a.id, b.id
        INTO v_horario_a, v_horario_b
        FROM horarios_profesional a
        JOIN horarios_profesional b
          ON a.id < b.id
         AND a.medico_id = b.medico_id
         AND a.dia_semana = b.dia_semana
         AND a.activo
         AND b.activo
         AND int4range(
                hora_a_segundos(a.hora_inicio),
                hora_a_segundos(a.hora_fin),
                '[)'
             ) && int4range(
                hora_a_segundos(b.hora_inicio),
                hora_a_segundos(b.hora_fin),
                '[)'
             )
         AND daterange(
                a.vigente_desde,
                COALESCE(a.vigente_hasta, 'infinity'::date),
                '[]'
             ) && daterange(
                b.vigente_desde,
                COALESCE(b.vigente_hasta, 'infinity'::date),
                '[]'
             )
        LIMIT 1;

        IF v_horario_a IS NOT NULL THEN
            RAISE EXCEPTION
                'No se puede activar la restricción de horarios: los horarios % y % se solapan para el mismo médico',
                v_horario_a, v_horario_b
                USING HINT = 'Desactiva o ajusta uno de los horarios solapados y vuelve a ejecutar la migración.';
        END IF;

        ALTER TABLE horarios_profesional
            ADD CONSTRAINT horarios_profesional_sin_solape
            EXCLUDE USING gist (
                medico_id WITH =,
                dia_semana WITH =,
                (int4range(
                    hora_a_segundos(hora_inicio),
                    hora_a_segundos(hora_fin),
                    '[)'
                )) WITH &&,
                (daterange(
                    vigente_desde,
                    COALESCE(vigente_hasta, 'infinity'::date),
                    '[]'
                )) WITH &&
            )
            WHERE (activo);
    END IF;
END
$$;

-- ---------------------------------------------------------------------------
-- Bloqueos puntuales
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS bloqueos_agenda (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    medico_id    BIGINT NOT NULL REFERENCES medicos (id) ON DELETE CASCADE,
    inicio       TIMESTAMPTZ NOT NULL,
    fin          TIMESTAMPTZ NOT NULL,
    motivo       TEXT NOT NULL DEFAULT '',
    creado_por   BIGINT REFERENCES usuarios (id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT bloqueos_agenda_rango_ck
        CHECK (fin > inicio)
);

CREATE INDEX IF NOT EXISTS bloqueos_agenda_medico_idx
    ON bloqueos_agenda (medico_id, inicio, fin);

-- Un médico no puede tener dos bloqueos solapados. No se permite el solape
-- aunque los motivos sean distintos, para que la agenda sea determinista.
DO $$
DECLARE
    v_bloqueo_a BIGINT;
    v_bloqueo_b BIGINT;
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'bloqueos_agenda_medico_sin_solape'
          AND conrelid = 'bloqueos_agenda'::regclass
    ) THEN
        SELECT a.id, b.id
        INTO v_bloqueo_a, v_bloqueo_b
        FROM bloqueos_agenda a
        JOIN bloqueos_agenda b
          ON a.id < b.id
         AND a.medico_id = b.medico_id
         AND tstzrange(a.inicio, a.fin, '[)') && tstzrange(b.inicio, b.fin, '[)')
        LIMIT 1;

        IF v_bloqueo_a IS NOT NULL THEN
            RAISE EXCEPTION
                'No se puede activar la restricción de bloqueos: los bloqueos % y % se solapan para el mismo médico',
                v_bloqueo_a, v_bloqueo_b
                USING HINT = 'Ajusta o elimina uno de los bloqueos solapados y vuelve a ejecutar la migración.';
        END IF;

        ALTER TABLE bloqueos_agenda
            ADD CONSTRAINT bloqueos_agenda_medico_sin_solape
            EXCLUDE USING gist (
                medico_id WITH =,
                tstzrange(inicio, fin, '[)') WITH &&
            );
    END IF;
END
$$;

-- ---------------------------------------------------------------------------
-- Triggers de updated_at
-- ---------------------------------------------------------------------------

DROP TRIGGER IF EXISTS trg_horarios_profesional_updated_at ON horarios_profesional;
CREATE TRIGGER trg_horarios_profesional_updated_at
    BEFORE UPDATE ON horarios_profesional
    FOR EACH ROW EXECUTE FUNCTION establecer_updated_at();

DROP TRIGGER IF EXISTS trg_bloqueos_agenda_updated_at ON bloqueos_agenda;
CREATE TRIGGER trg_bloqueos_agenda_updated_at
    BEFORE UPDATE ON bloqueos_agenda
    FOR EACH ROW EXECUTE FUNCTION establecer_updated_at();

COMMENT ON TABLE horarios_profesional IS
    'Disponibilidad semanal recurrente por médico. Las horas son locales de la clínica.';
COMMENT ON TABLE bloqueos_agenda IS
    'Bloqueos puntuales de agenda en UTC. Se restan de los slots disponibles.';

COMMIT;
