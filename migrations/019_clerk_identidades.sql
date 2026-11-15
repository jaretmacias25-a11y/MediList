BEGIN;

-- ---------------------------------------------------------------------------
-- Fase final: identidad externa con Clerk (opcional)
--
-- Permite que usuarios autenticados con Clerk obtengan una sesión local de
-- MediList. Las cuentas de staff conservan su contraseña local; las cuentas
-- que solo usan Clerk no tienen contraseña.
-- ---------------------------------------------------------------------------

ALTER TABLE usuarios ADD COLUMN IF NOT EXISTS clerk_user_id TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS usuarios_clerk_user_id_uk
    ON usuarios (clerk_user_id)
    WHERE clerk_user_id IS NOT NULL;

ALTER TABLE usuarios ALTER COLUMN password_hash DROP NOT NULL;

COMMENT ON COLUMN usuarios.clerk_user_id IS
    'Identificador del usuario en Clerk. Vacío para cuentas locales.';

COMMIT;
