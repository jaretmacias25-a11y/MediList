BEGIN;

-- Versión de sesión del usuario. Se incrementa cuando se revocan sesiones,
-- cambian roles, cambia la contraseña o se desactiva la cuenta.
ALTER TABLE usuarios
    ADD COLUMN IF NOT EXISTS token_version INTEGER NOT NULL DEFAULT 0;

COMMIT;
