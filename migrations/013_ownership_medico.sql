BEGIN;

-- ---------------------------------------------------------------------------
-- Fase 3: ownership y retiro de permisos globales del rol médico
--
-- Un médico ya no puede consultar todos los pacientes ni todas las citas.
-- Usa los permisos propios:
--   - medico:agenda:leer / medico:agenda:escribir
--   - pacientes:atendidos:leer
--
-- La API además aplica una guarda por roles en los endpoints administrativos,
-- porque un access token emitido antes de esta migración conserva los permisos
-- antiguos hasta que expire. Retirar permisos en la base no basta por sí solo.
-- ---------------------------------------------------------------------------

DELETE FROM rol_permisos rp
USING roles r, permisos p
WHERE rp.rol_id = r.id
  AND rp.permiso_id = p.id
  AND r.codigo = 'medico'
  AND p.codigo IN ('pacientes:leer', 'citas:leer', 'citas:escribir');

-- Reafirmar los permisos propios del médico. La migración 009 ya los asigna;
-- este bloque mantiene la migración idempotente y repara instalaciones donde
-- el rol existía antes de la fase de portal.
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

COMMIT;
