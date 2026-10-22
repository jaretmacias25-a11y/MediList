BEGIN;

INSERT INTO especialidades (codigo, nombre, descripcion) VALUES
    ('MED-GEN', 'Medicina General', 'Atención médica integral ambulatoria'),
    ('CARD',    'Cardiología',      'Diagnóstico y tratamiento del sistema cardiovascular'),
    ('PED',     'Pediatría',        'Atención médica de niñas, niños y adolescentes'),
    ('GINE',    'Ginecología',      'Salud reproductiva y atención de la mujer'),
    ('TRAUM',   'Traumatología',    'Lesiones del sistema musculoesquelético'),
    ('DERM',    'Dermatología',     'Enfermedades de la piel, cabello y uñas'),
    ('NEURO',   'Neurología',       'Trastornos del sistema nervioso'),
    ('OFT',     'Oftalmología',     'Salud visual y enfermedades oculares')
ON CONFLICT (codigo) DO NOTHING;

COMMIT;
