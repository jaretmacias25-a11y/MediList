BEGIN;

-- ---------------------------------------------------------------------------
-- Clerk como único proveedor de identidad.
--
-- Los endpoints locales de login, registro, verificación y recuperación de
-- contraseña ya no existen. Se eliminan los hashes y bloqueos heredados para
-- que ninguna credencial local pueda reutilizarse.
-- ---------------------------------------------------------------------------

UPDATE usuarios
SET password_hash = NULL,
    debe_cambiar_password = FALSE,
    intentos_fallidos = 0,
    bloqueado_hasta = NULL;

COMMIT;
