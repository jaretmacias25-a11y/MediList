BEGIN;

-- ---------------------------------------------------------------------------
-- RBAC: permisos, roles y asignaciones
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS permisos (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    codigo      VARCHAR(80) NOT NULL UNIQUE,
    modulo      VARCHAR(40) NOT NULL,
    descripcion TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS roles (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    codigo      VARCHAR(40) NOT NULL UNIQUE,
    nombre      VARCHAR(100) NOT NULL,
    descripcion TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS rol_permisos (
    rol_id     BIGINT NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    permiso_id BIGINT NOT NULL REFERENCES permisos (id) ON DELETE CASCADE,
    PRIMARY KEY (rol_id, permiso_id)
);

-- ---------------------------------------------------------------------------
-- Usuarios
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS usuarios (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username          VARCHAR(50)  NOT NULL,
    email             VARCHAR(160) NOT NULL,
    password_hash     TEXT         NOT NULL,
    nombres           VARCHAR(120) NOT NULL,
    apellidos         VARCHAR(120) NOT NULL,
    activo            BOOLEAN      NOT NULL DEFAULT TRUE,
    intentos_fallidos INTEGER      NOT NULL DEFAULT 0 CHECK (intentos_fallidos >= 0),
    bloqueado_hasta   TIMESTAMPTZ,
    ultimo_acceso     TIMESTAMPTZ,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS usuarios_username_lower_uk ON usuarios (LOWER(username));
CREATE UNIQUE INDEX IF NOT EXISTS usuarios_email_lower_uk    ON usuarios (LOWER(email));
CREATE INDEX IF NOT EXISTS idx_usuarios_activo ON usuarios (activo);

CREATE TABLE IF NOT EXISTS usuario_roles (
    usuario_id BIGINT NOT NULL REFERENCES usuarios (id) ON DELETE CASCADE,
    rol_id     BIGINT NOT NULL REFERENCES roles (id) ON DELETE RESTRICT,
    PRIMARY KEY (usuario_id, rol_id)
);

CREATE INDEX IF NOT EXISTS idx_usuario_roles_rol ON usuario_roles (rol_id);

-- ---------------------------------------------------------------------------
-- Refresh tokens (solo se guarda el hash SHA-256)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    usuario_id  BIGINT      NOT NULL REFERENCES usuarios (id) ON DELETE CASCADE,
    token_hash  CHAR(64)    NOT NULL UNIQUE,
    expira_en   TIMESTAMPTZ NOT NULL,
    revocado_en TIMESTAMPTZ,
    ip          VARCHAR(45) NOT NULL DEFAULT '',
    user_agent  TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_usuario ON refresh_tokens (usuario_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expira  ON refresh_tokens (expira_en);

-- ---------------------------------------------------------------------------
-- Auditoría
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS auditoria (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    usuario_id  BIGINT REFERENCES usuarios (id) ON DELETE SET NULL,
    username    VARCHAR(50)  NOT NULL DEFAULT '',
    accion      VARCHAR(40)  NOT NULL,
    recurso     VARCHAR(80)  NOT NULL DEFAULT '',
    metodo      VARCHAR(10)  NOT NULL DEFAULT '',
    ruta        TEXT         NOT NULL DEFAULT '',
    estado_http INTEGER      NOT NULL DEFAULT 0,
    ip          VARCHAR(45)  NOT NULL DEFAULT '',
    user_agent  TEXT         NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_auditoria_fecha   ON auditoria (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_auditoria_usuario ON auditoria (usuario_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_auditoria_accion  ON auditoria (accion, created_at DESC);

-- ---------------------------------------------------------------------------
-- Triggers de updated_at
-- ---------------------------------------------------------------------------
DROP TRIGGER IF EXISTS trg_roles_updated_at ON roles;
CREATE TRIGGER trg_roles_updated_at
    BEFORE UPDATE ON roles
    FOR EACH ROW EXECUTE FUNCTION establecer_updated_at();

DROP TRIGGER IF EXISTS trg_usuarios_updated_at ON usuarios;
CREATE TRIGGER trg_usuarios_updated_at
    BEFORE UPDATE ON usuarios
    FOR EACH ROW EXECUTE FUNCTION establecer_updated_at();

COMMIT;
