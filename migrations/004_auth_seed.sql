BEGIN;

-- ---------------------------------------------------------------------------
-- Permisos
-- ---------------------------------------------------------------------------
INSERT INTO permisos (codigo, modulo, descripcion) VALUES
    ('pacientes:leer',        'pacientes',      'Consultar pacientes'),
    ('pacientes:escribir',    'pacientes',      'Crear, actualizar y desactivar pacientes'),
    ('medicos:leer',          'medicos',        'Consultar médicos'),
    ('medicos:escribir',      'medicos',        'Crear, actualizar y desactivar médicos'),
    ('especialidades:leer',   'especialidades', 'Consultar especialidades'),
    ('especialidades:escribir','especialidades','Gestionar especialidades'),
    ('citas:leer',            'citas',          'Consultar citas'),
    ('citas:escribir',        'citas',          'Agendar, reprogramar y cancelar citas'),
    ('usuarios:leer',         'usuarios',       'Consultar usuarios'),
    ('usuarios:escribir',     'usuarios',       'Crear y actualizar usuarios y roles'),
    ('roles:leer',            'roles',          'Consultar roles y permisos'),
    ('auditoria:leer',        'auditoria',      'Consultar la bitácora de auditoría')
ON CONFLICT (codigo) DO UPDATE
SET modulo = EXCLUDED.modulo,
    descripcion = EXCLUDED.descripcion;

-- ---------------------------------------------------------------------------
-- Roles
-- ---------------------------------------------------------------------------
INSERT INTO roles (codigo, nombre, descripcion) VALUES
    ('admin',      'Administrador', 'Acceso total al sistema'),
    ('medico',     'Médico',        'Atención clínica y agenda propia'),
    ('recepcion',  'Recepción',     'Gestión de pacientes y agenda'),
    ('enfermeria', 'Enfermería',    'Consulta clínica y agenda'),
    ('auditor',    'Auditor',       'Consulta de usuarios y auditoría')
ON CONFLICT (codigo) DO UPDATE
SET nombre = EXCLUDED.nombre,
    descripcion = EXCLUDED.descripcion;

-- Administrador: todos los permisos
INSERT INTO rol_permisos (rol_id, permiso_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permisos p
WHERE r.codigo = 'admin'
ON CONFLICT DO NOTHING;

-- Médico
INSERT INTO rol_permisos (rol_id, permiso_id)
SELECT r.id, p.id
FROM roles r
JOIN permisos p ON p.codigo IN (
    'pacientes:leer', 'medicos:leer', 'especialidades:leer',
    'citas:leer', 'citas:escribir'
)
WHERE r.codigo = 'medico'
ON CONFLICT DO NOTHING;

-- Recepción
INSERT INTO rol_permisos (rol_id, permiso_id)
SELECT r.id, p.id
FROM roles r
JOIN permisos p ON p.codigo IN (
    'pacientes:leer', 'pacientes:escribir',
    'medicos:leer', 'especialidades:leer',
    'citas:leer', 'citas:escribir'
)
WHERE r.codigo = 'recepcion'
ON CONFLICT DO NOTHING;

-- Enfermería
INSERT INTO rol_permisos (rol_id, permiso_id)
SELECT r.id, p.id
FROM roles r
JOIN permisos p ON p.codigo IN (
    'pacientes:leer', 'medicos:leer', 'especialidades:leer', 'citas:leer'
)
WHERE r.codigo = 'enfermeria'
ON CONFLICT DO NOTHING;

-- Auditor
INSERT INTO rol_permisos (rol_id, permiso_id)
SELECT r.id, p.id
FROM roles r
JOIN permisos p ON p.codigo IN (
    'auditoria:leer', 'usuarios:leer', 'roles:leer'
)
WHERE r.codigo = 'auditor'
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------------
-- Usuario administrador inicial
-- Contraseña temporal: Admin12345!2026
-- Cambiar inmediatamente después del primer inicio de sesión.
-- ---------------------------------------------------------------------------
INSERT INTO usuarios (username, email, password_hash, nombres, apellidos, activo)
VALUES (
    'admin',
    'admin@medilist.local',
    '$2a$12$KcWaogmrMOX2tAv07E14u.119Bl.TT8Lj4BiVRzNSiyM2VZHwTF.m',
    'Administrador',
    'MediList',
    TRUE
)
ON CONFLICT DO NOTHING;

INSERT INTO usuario_roles (usuario_id, rol_id)
SELECT u.id, r.id
FROM usuarios u
CROSS JOIN roles r
WHERE u.username = 'admin' AND r.codigo = 'admin'
ON CONFLICT DO NOTHING;

COMMIT;
