BEGIN;

-- ---------------------------------------------------------------------------
-- Portal del paciente: cuentas, catálogo y permisos
--
-- Esta migración es aditiva. No elimina columnas ni cambia el significado de
-- los datos existentes. Los pacientes y médicos históricos quedan sin cuenta
-- vinculada hasta que administración los vincule.
--
-- Regla de acceso:
--   - Medicina General, Fisioterapia, Odontología y Nutrición: directo.
--   - El resto de especialidades: requieren derivación.
--
-- Los permisos globales actuales del rol medico se mantienen por
-- compatibilidad. La autorización por fila y su retiro se harán en la fase de
-- ownership, cuando los endpoints de portal ya existan.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- Catálogo de especialidades
-- ---------------------------------------------------------------------------
ALTER TABLE especialidades
    ADD COLUMN IF NOT EXISTS requiere_derivacion BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS activa BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS orden INTEGER NOT NULL DEFAULT 0
        CHECK (orden >= 0),
    ADD COLUMN IF NOT EXISTS duracion_cita_min INTEGER NOT NULL DEFAULT 30
        CHECK (duracion_cita_min BETWEEN 5 AND 240);

CREATE INDEX IF NOT EXISTS idx_especialidades_activa
    ON especialidades (activa, orden);

-- Insertar solo las especialidades nuevas. No se sobrescriben nombres,
-- descripciones ni configuración existente si la migración se ejecuta de nuevo.
INSERT INTO especialidades (
    codigo, nombre, descripcion, requiere_derivacion, activa, orden, duracion_cita_min
) VALUES
    ('FISIO',    'Fisioterapia',          'Rehabilitación física y terapia del movimiento', FALSE, TRUE, 20, 45),
    ('ODONTO',   'Odontología',           'Atención odontológica general',                   FALSE, TRUE, 30, 30),
    ('NUTRI',    'Nutrición',             'Valoración y acompañamiento nutricional',          FALSE, TRUE, 40, 45),
    ('OTORRINO', 'Otorrinolaringología',  'Evaluación y tratamiento de oído, nariz y garganta', TRUE, TRUE, 50, 30)
ON CONFLICT (codigo) DO NOTHING;

-- Medicina General es la puerta de entrada y no requiere derivación.
-- Las demás especialidades existentes conservan el valor por defecto TRUE.
UPDATE especialidades
SET requiere_derivacion = FALSE
WHERE codigo = 'MED-GEN';

-- Orden inicial solo para registros que aún no tienen uno definido.
UPDATE especialidades
SET orden = CASE codigo
    WHEN 'MED-GEN' THEN 10
    WHEN 'FISIO'   THEN 20
    WHEN 'ODONTO'  THEN 30
    WHEN 'NUTRI'   THEN 40
    WHEN 'OTORRINO' THEN 50
    WHEN 'CARD'    THEN 60
    WHEN 'PED'     THEN 70
    WHEN 'GINE'    THEN 80
    WHEN 'TRAUM'   THEN 90
    WHEN 'DERM'    THEN 100
    WHEN 'NEURO'   THEN 110
    WHEN 'OFT'     THEN 120
    ELSE orden
END
WHERE orden = 0
  AND codigo IN (
      'MED-GEN', 'FISIO', 'ODONTO', 'NUTRI', 'OTORRINO',
      'CARD', 'PED', 'GINE', 'TRAUM', 'DERM', 'NEURO', 'OFT'
  );

-- ---------------------------------------------------------------------------
-- Vínculos cuenta-perfil
-- ---------------------------------------------------------------------------
ALTER TABLE pacientes
    ADD COLUMN IF NOT EXISTS usuario_id BIGINT
        REFERENCES usuarios (id) ON DELETE SET NULL;

CREATE UNIQUE INDEX IF NOT EXISTS pacientes_usuario_id_uk
    ON pacientes (usuario_id)
    WHERE usuario_id IS NOT NULL;

ALTER TABLE medicos
    ADD COLUMN IF NOT EXISTS usuario_id BIGINT
        REFERENCES usuarios (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS duracion_cita_min INTEGER NOT NULL DEFAULT 30
        CHECK (duracion_cita_min BETWEEN 5 AND 240),
    ADD COLUMN IF NOT EXISTS bio TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS foto_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS acepta_portal BOOLEAN NOT NULL DEFAULT TRUE;

CREATE UNIQUE INDEX IF NOT EXISTS medicos_usuario_id_uk
    ON medicos (usuario_id)
    WHERE usuario_id IS NOT NULL;

-- ---------------------------------------------------------------------------
-- Campos de cuenta
-- ---------------------------------------------------------------------------
ALTER TABLE usuarios
    ADD COLUMN IF NOT EXISTS email_verificado_en TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS terminos_aceptados_en TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS version_terminos VARCHAR(20) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS debe_cambiar_password BOOLEAN NOT NULL DEFAULT FALSE;

-- Los usuarios existentes quedan con email_verificado_en = NULL. La fase de
-- registro decidirá qué cuentas internas están exentas de verificación; no se
-- marcan automáticamente para no alterar cuentas nuevas si la migración se
-- vuelve a ejecutar.

-- ---------------------------------------------------------------------------
-- Tokens de verificación de email y recuperación de contraseña
-- Solo se guarda el hash SHA-256 en hexadecimal.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tokens_cuenta (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    usuario_id  BIGINT NOT NULL REFERENCES usuarios (id) ON DELETE CASCADE,
    tipo        VARCHAR(30) NOT NULL
        CHECK (tipo IN ('verificacion_email', 'reset_password')),
    token_hash  CHAR(64) NOT NULL UNIQUE,
    expira_en   TIMESTAMPTZ NOT NULL,
    usado_en    TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS tokens_cuenta_usuario_idx
    ON tokens_cuenta (usuario_id, tipo, expira_en);

CREATE INDEX IF NOT EXISTS tokens_cuenta_limpieza_idx
    ON tokens_cuenta (expira_en)
    WHERE usado_en IS NULL;

-- ---------------------------------------------------------------------------
-- Permisos nuevos
-- ---------------------------------------------------------------------------
INSERT INTO permisos (codigo, modulo, descripcion) VALUES
    ('catalogo:leer',              'catalogo',       'Consultar especialidades y médicos para el portal'),
    ('portal:perfil:leer',         'portal',         'Consultar el perfil propio del paciente'),
    ('portal:perfil:escribir',     'portal',         'Actualizar los datos de contacto propios'),
    ('portal:citas:leer',          'portal',         'Consultar las citas propias del paciente'),
    ('portal:citas:escribir',      'portal',         'Agendar, reprogramar y cancelar citas propias'),
    ('portal:derivaciones:leer',   'portal',         'Consultar las derivaciones propias'),
    ('medico:agenda:leer',         'medico',         'Consultar la agenda propia del médico'),
    ('medico:agenda:escribir',     'medico',         'Actualizar el estado de las citas propias'),
    ('medico:nota:leer',           'medico',         'Consultar la nota de sus propias citas'),
    ('medico:nota:escribir',       'medico',         'Registrar la nota de sus propias citas'),
    ('pacientes:atendidos:leer',   'pacientes',      'Consultar pacientes con relación de atención'),
    ('derivaciones:leer',          'derivaciones',   'Consultar derivaciones'),
    ('derivaciones:escribir',      'derivaciones',   'Emitir derivaciones desde una cita atendida'),
    ('derivaciones:sobrescribir',  'derivaciones',   'Autorizar excepciones a la regla de derivación'),
    ('agenda:gestionar',           'agenda',         'Gestionar horarios y bloqueos de agenda'),
    ('cuentas:vincular',           'cuentas',        'Vincular cuentas con pacientes o médicos existentes')
ON CONFLICT (codigo) DO UPDATE
SET modulo = EXCLUDED.modulo,
    descripcion = EXCLUDED.descripcion;

-- ---------------------------------------------------------------------------
-- Rol de paciente
-- ---------------------------------------------------------------------------
INSERT INTO roles (codigo, nombre, descripcion) VALUES
    ('paciente', 'Paciente', 'Acceso al portal propio de citas y derivaciones')
ON CONFLICT (codigo) DO UPDATE
SET nombre = EXCLUDED.nombre,
    descripcion = EXCLUDED.descripcion;

-- Paciente: solo su portal y catálogo de lectura.
INSERT INTO rol_permisos (rol_id, permiso_id)
SELECT r.id, p.id
FROM roles r
JOIN permisos p ON p.codigo IN (
    'catalogo:leer',
    'portal:perfil:leer', 'portal:perfil:escribir',
    'portal:citas:leer', 'portal:citas:escribir',
    'portal:derivaciones:leer'
)
WHERE r.codigo = 'paciente'
ON CONFLICT DO NOTHING;

-- Médico: permisos propios y de derivación. Los permisos globales actuales
-- se conservan hasta la fase de ownership.
INSERT INTO rol_permisos (rol_id, permiso_id)
SELECT r.id, p.id
FROM roles r
JOIN permisos p ON p.codigo IN (
    'medico:agenda:leer', 'medico:agenda:escribir',
    'medico:nota:leer', 'medico:nota:escribir',
    'pacientes:atendidos:leer',
    'derivaciones:leer', 'derivaciones:escribir'
)
WHERE r.codigo = 'medico'
ON CONFLICT DO NOTHING;

-- Recepción: gestión de agenda y vinculación de cuentas.
INSERT INTO rol_permisos (rol_id, permiso_id)
SELECT r.id, p.id
FROM roles r
JOIN permisos p ON p.codigo IN ('agenda:gestionar', 'cuentas:vincular')
WHERE r.codigo = 'recepcion'
ON CONFLICT DO NOTHING;

-- Administrador: todos los permisos, incluidos los nuevos.
INSERT INTO rol_permisos (rol_id, permiso_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permisos p
WHERE r.codigo = 'admin'
ON CONFLICT DO NOTHING;

COMMIT;
