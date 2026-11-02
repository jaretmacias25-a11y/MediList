BEGIN;

-- ---------------------------------------------------------------------------
-- Fase 0: Pediatría de acceso directo para todos
--
-- La decisión de producto amplía las especialidades de acceso directo a:
--   MED-GEN, FISIO, ODONTO, NUTRI y PED.
--
-- La fuente de verdad sigue siendo especialidades.requiere_derivacion; no se
-- agregan listas hardcodeadas en el código.
-- ---------------------------------------------------------------------------

UPDATE especialidades
SET requiere_derivacion = FALSE,
    version = version + 1
WHERE codigo = 'PED'
  AND requiere_derivacion = TRUE;

COMMIT;
