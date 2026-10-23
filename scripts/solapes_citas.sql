-- Lista los solapamientos que impiden activar las restricciones de agenda.
-- Úsalo antes de aplicar 008_consistencia_concurrencia.sql o cuando la
-- migración indique que existen citas solapadas.
--
-- Cada fila muestra las dos citas implicadas y el motivo (médico o paciente).
-- Corrige manualmente los horarios/estados y vuelve a ejecutar la migración.

WITH citas_activas AS (
    SELECT
        id,
        codigo,
        paciente_id,
        medico_id,
        fecha_hora_inicio,
        duracion_min,
        estado,
        tstzrange(
            fecha_hora_inicio,
            fecha_hora_inicio + (duracion_min * interval '1 minute'),
            '[)'
        ) AS rango
    FROM citas
    WHERE estado IN ('programada', 'confirmada')
)
SELECT
    'medico' AS tipo_solape,
    a.medico_id AS recurso_id,
    a.id AS cita_a,
    a.codigo AS codigo_a,
    a.fecha_hora_inicio AS inicio_a,
    a.duracion_min AS duracion_a,
    b.id AS cita_b,
    b.codigo AS codigo_b,
    b.fecha_hora_inicio AS inicio_b,
    b.duracion_min AS duracion_b
FROM citas_activas a
JOIN citas_activas b
  ON a.id < b.id
 AND a.medico_id = b.medico_id
 AND a.rango && b.rango

UNION ALL

SELECT
    'paciente' AS tipo_solape,
    a.paciente_id AS recurso_id,
    a.id AS cita_a,
    a.codigo AS codigo_a,
    a.fecha_hora_inicio AS inicio_a,
    a.duracion_min AS duracion_a,
    b.id AS cita_b,
    b.codigo AS codigo_b,
    b.fecha_hora_inicio AS inicio_b,
    b.duracion_min AS duracion_b
FROM citas_activas a
JOIN citas_activas b
  ON a.id < b.id
 AND a.paciente_id = b.paciente_id
 AND a.rango && b.rango

ORDER BY tipo_solape, recurso_id, cita_a, cita_b;
