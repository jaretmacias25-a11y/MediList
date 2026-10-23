BEGIN;

-- ---------------------------------------------------------------------------
-- Consistencia bajo concurrencia
--
-- 1. Evita solapamientos de agenda en la base de datos, no solo en el
--    servicio. La comprobación previa del servicio puede sufrir una carrera
--    entre dos solicitudes simultáneas.
-- 2. Agrega versiones optimistas para detectar actualizaciones concurrentes
--    sobre el mismo registro.
-- 3. Refuerza la consistencia de participantes y fechas de las citas.
-- ---------------------------------------------------------------------------

-- btree_gist permite combinar la igualdad de médico/paciente con el solape
-- de rangos en una restricción de exclusión.
CREATE EXTENSION IF NOT EXISTS btree_gist;

-- La suma timestamptz + interval es STABLE en PostgreSQL, aunque para
-- intervalos de minutos el resultado es determinista. La encapsulamos como
-- IMMUTABLE para poder usarla en el índice de la restricción de exclusión.
CREATE OR REPLACE FUNCTION cita_fin(inicio TIMESTAMPTZ, duracion_min INTEGER)
RETURNS TIMESTAMPTZ
LANGUAGE sql
IMMUTABLE
PARALLEL SAFE
AS $$
    SELECT inicio + (duracion_min * interval '1 minute')
$$;

-- ---------------------------------------------------------------------------
-- Versiones optimistas
-- ---------------------------------------------------------------------------

ALTER TABLE especialidades
    ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 0;

ALTER TABLE pacientes
    ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 0;

ALTER TABLE medicos
    ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 0;

ALTER TABLE citas
    ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 0;

ALTER TABLE usuarios
    ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 0;

-- Versión de sesión con la que fue emitido el refresh token. Permite que la
-- rotación detecte una revocación concurrente sin crear una sesión nueva
-- después de que el administrador haya incrementado token_version.
ALTER TABLE refresh_tokens
    ADD COLUMN IF NOT EXISTS token_version INTEGER NOT NULL DEFAULT 0;

-- ---------------------------------------------------------------------------
-- Restricciones de exclusión para la agenda
--
-- Solo las citas programadas o confirmadas ocupan un horario. Las citas
-- canceladas, atendidas, no asistidas o reprogramadas liberan el rango.
-- ---------------------------------------------------------------------------

DO $$
DECLARE
    v_cita_a BIGINT;
    v_cita_b BIGINT;
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'citas_medico_sin_solape'
          AND conrelid = 'citas'::regclass
    ) THEN
        SELECT a.id, b.id
        INTO v_cita_a, v_cita_b
        FROM citas a
        JOIN citas b
          ON a.id < b.id
         AND a.medico_id = b.medico_id
         AND a.estado IN ('programada', 'confirmada')
         AND b.estado IN ('programada', 'confirmada')
         AND tstzrange(
                a.fecha_hora_inicio,
                cita_fin(a.fecha_hora_inicio, a.duracion_min),
                '[)'
             ) && tstzrange(
                b.fecha_hora_inicio,
                cita_fin(b.fecha_hora_inicio, b.duracion_min),
                '[)'
             )
        LIMIT 1;

        IF v_cita_a IS NOT NULL THEN
            RAISE EXCEPTION
                'No se puede activar la restricción de agenda: las citas % y % se solapan para el mismo médico',
                v_cita_a, v_cita_b
                USING HINT = 'Resuelve los solapamientos existentes con scripts/solapes_citas.sql y vuelve a ejecutar la migración.';
        END IF;

        ALTER TABLE citas
            ADD CONSTRAINT citas_medico_sin_solape
            EXCLUDE USING gist (
                medico_id WITH =,
                tstzrange(
                    fecha_hora_inicio,
                    cita_fin(fecha_hora_inicio, duracion_min),
                    '[)'
                ) WITH &&
            )
            WHERE (estado IN ('programada', 'confirmada'));
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'citas_paciente_sin_solape'
          AND conrelid = 'citas'::regclass
    ) THEN
        SELECT a.id, b.id
        INTO v_cita_a, v_cita_b
        FROM citas a
        JOIN citas b
          ON a.id < b.id
         AND a.paciente_id = b.paciente_id
         AND a.estado IN ('programada', 'confirmada')
         AND b.estado IN ('programada', 'confirmada')
         AND tstzrange(
                a.fecha_hora_inicio,
                cita_fin(a.fecha_hora_inicio, a.duracion_min),
                '[)'
             ) && tstzrange(
                b.fecha_hora_inicio,
                cita_fin(b.fecha_hora_inicio, b.duracion_min),
                '[)'
             )
        LIMIT 1;

        IF v_cita_a IS NOT NULL THEN
            RAISE EXCEPTION
                'No se puede activar la restricción de agenda: las citas % y % se solapan para el mismo paciente',
                v_cita_a, v_cita_b
                USING HINT = 'Resuelve los solapamientos existentes con scripts/solapes_citas.sql y vuelve a ejecutar la migración.';
        END IF;

        ALTER TABLE citas
            ADD CONSTRAINT citas_paciente_sin_solape
            EXCLUDE USING gist (
                paciente_id WITH =,
                tstzrange(
                    fecha_hora_inicio,
                    cita_fin(fecha_hora_inicio, duracion_min),
                    '[)'
                ) WITH &&
            )
            WHERE (estado IN ('programada', 'confirmada'));
    END IF;
END
$$;

-- ---------------------------------------------------------------------------
-- Validaciones de integridad de la cita
--
-- La comprobación se hace también en la base de datos para cerrar la ventana
-- entre la validación del servicio y el INSERT/UPDATE. En actualizaciones se
-- valida la fecha solo si cambia y los participantes solo si cambian, para
-- permitir cancelar una cita histórica aunque el paciente esté inactivo.
-- ---------------------------------------------------------------------------

CREATE OR REPLACE FUNCTION validar_cita_consistente()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.fecha_hora_inicio < now() THEN
            RAISE EXCEPTION 'fecha_hora_inicio no puede estar en el pasado'
                USING ERRCODE = '23514';
        END IF;

        IF NOT EXISTS (
            SELECT 1 FROM pacientes
            WHERE id = NEW.paciente_id AND activo
        ) THEN
            RAISE EXCEPTION 'el paciente % no existe o está inactivo', NEW.paciente_id
                USING ERRCODE = '23514';
        END IF;

        IF NOT EXISTS (
            SELECT 1 FROM medicos
            WHERE id = NEW.medico_id AND activo
        ) THEN
            RAISE EXCEPTION 'el médico % no existe o está inactivo', NEW.medico_id
                USING ERRCODE = '23514';
        END IF;
    ELSE
        IF NEW.fecha_hora_inicio IS DISTINCT FROM OLD.fecha_hora_inicio
           AND NEW.fecha_hora_inicio < now() THEN
            RAISE EXCEPTION 'fecha_hora_inicio no puede estar en el pasado'
                USING ERRCODE = '23514';
        END IF;

        IF NEW.paciente_id IS DISTINCT FROM OLD.paciente_id
           AND NOT EXISTS (
               SELECT 1 FROM pacientes
               WHERE id = NEW.paciente_id AND activo
           ) THEN
            RAISE EXCEPTION 'el paciente % no existe o está inactivo', NEW.paciente_id
                USING ERRCODE = '23514';
        END IF;

        IF NEW.medico_id IS DISTINCT FROM OLD.medico_id
           AND NOT EXISTS (
               SELECT 1 FROM medicos
               WHERE id = NEW.medico_id AND activo
           ) THEN
            RAISE EXCEPTION 'el médico % no existe o está inactivo', NEW.medico_id
                USING ERRCODE = '23514';
        END IF;
    END IF;

    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_citas_consistencia ON citas;
CREATE TRIGGER trg_citas_consistencia
    BEFORE INSERT OR UPDATE ON citas
    FOR EACH ROW EXECUTE FUNCTION validar_cita_consistente();

COMMIT;
