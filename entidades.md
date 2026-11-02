# MediList — Modelo de Entidades

Sistema de gestión para hospital / clínica.

**Convenciones:** `PK` = clave primaria, `FK` = clave foránea, `UK` = valor único. Atributos en `snake_case`; entidades en singular.

---

## Índice

1. [Personas, profesionales y seguridad](#1-personas-profesionales-y-seguridad)
2. [Infraestructura física](#2-infraestructura-física)
3. [Agenda y citas](#3-agenda-y-citas)
4. [Historia clínica y atención](#4-historia-clínica-y-atención)
5. [Hospitalización y cirugía](#5-hospitalización-y-cirugía)
6. [Farmacia e inventario](#6-farmacia-e-inventario)
7. [Facturación y seguros](#7-facturación-y-seguros)
8. [Soporte y auditoría](#8-soporte-y-auditoría)
9. [Diagrama del flujo clínico](#diagrama-del-flujo-clínico-principal)
10. [Reglas de negocio](#reglas-de-negocio-clave)

---

## 1. Personas, profesionales y seguridad

| Entidad | Propósito | Atributos clave |
|---|---|---|
| **Persona** | Datos de identidad comunes a pacientes y empleados | `id_persona` PK, `tipo_documento`, `numero_documento` UK, `nombres`, `apellidos`, `fecha_nacimiento`, `sexo`, `estado_civil`, `telefono`, `email`, `direccion`, `nacionalidad`, auditoría y borrado lógico |
| **Paciente** | Perfil clínico-administrativo | `id_paciente` PK, `id_persona` FK UK, `usuario_id` FK UK (opcional), `medico_responsable_id` FK (opcional), `numero_historia` UK, `grupo_sanguineo`, `factor_rh`, `contacto_emergencia_nombre`, `contacto_emergencia_telefono`, `fecha_registro`, `activo` |
| **TransferenciaPaciente** | Historial append-only del cambio interno de médico tratante | `id` PK, `paciente_id` FK, `medico_origen_id` FK, `medico_destino_id` FK, `motivo`, `creada_por_usuario_id` FK, `created_at` |
| **Empleado** | Vínculo laboral | `id_empleado` PK, `id_persona` FK UK, `codigo_empleado` UK, `id_servicio` FK, `cargo`, `fecha_ingreso`, `fecha_salida`, `tipo_contrato`, `estado` |
| **ProfesionalSalud** | Empleado habilitado para actos clínicos | `id_profesional` PK, `id_empleado` FK UK, `tipo_profesional` (médico, enfermero, etc.), `numero_licencia` UK, `firma_digital`, `activo` |
| **Especialidad** | Catálogo de especialidades | `id_especialidad` PK, `codigo` UK, `nombre`, `descripcion` |
| **ProfesionalEspecialidad** | Relación N:M profesional–especialidad | `id_profesional` FK, `id_especialidad` FK, `principal`, `fecha_certificacion` (PK compuesta) |
| **Usuario** | Credenciales de acceso | `id_usuario` PK, `username` UK, `email` UK, `password_hash`, `email_verificado_en`, `terminos_aceptados_en`, `version_terminos`, `debe_cambiar_password`, `ultimo_acceso`, `activo`, `token_version`, `mfa_habilitado` |
| **Rol / Permiso** | Control de acceso RBAC | `id_rol` PK, `nombre` UK, `descripcion`; `id_permiso` PK, `codigo` UK, `modulo`, `descripcion` |
| **UsuarioRol / RolPermiso** | Asignaciones N:M con vigencia | claves FK compuestas, `vigente_desde`, `vigente_hasta` |

### 1.1 Seguridad implementada en la versión actual

Esta versión implementa el núcleo de autenticación y autorización con las siguientes tablas:

| Tabla | Propósito | Atributos clave |
|---|---|---|
| `usuarios` | Cuenta de acceso | `id` PK, `username` UK, `email` UK, `password_hash`, `nombres`, `apellidos`, `activo`, `email_verificado_en`, `terminos_aceptados_en`, `version_terminos`, `debe_cambiar_password`, `token_version`, `intentos_fallidos`, `bloqueado_hasta`, `ultimo_acceso` |
| `roles` | Agrupación de permisos | `id` PK, `codigo` UK, `nombre`, `descripcion` |
| `permisos` | Acción autorizable | `id` PK, `codigo` UK, `modulo`, `descripcion` |
| `usuario_roles` | Asignación N:M usuario–rol | `usuario_id` FK, `rol_id` FK, PK compuesta |
| `rol_permisos` | Asignación N:M rol–permiso | `rol_id` FK, `permiso_id` FK, PK compuesta |
| `refresh_tokens` | Sesiones de renovación | `id` PK, `usuario_id` FK, `token_hash` UK, `token_version`, `expira_en`, `revocado_en`, `ip`, `user_agent` |
| `tokens_cuenta` | Tokens de verificación de email y reset | `id` PK, `usuario_id` FK, `tipo` (`verificacion_email`, `reset_password`), `token_hash` UK, `expira_en`, `usado_en`, `created_at` |
| `solicitudes_vinculacion` | Vinculación de una cuenta con una historia existente | `id` PK, `usuario_id` FK, `paciente_id` FK, `estado` (`pendiente`, `aprobada`, `rechazada`), `motivo`, `resuelta_por_usuario_id` FK, `resuelta_en`, `created_at`, `updated_at` |
| `auditoria` | Bitácora de seguridad | `id` PK, `usuario_id` FK, `username`, `accion`, `recurso`, `metodo`, `ruta`, `estado_http`, `ip`, `user_agent`, `trace_id`, `created_at` |
| `errores_aplicacion` | Errores agregados para observabilidad | `id` PK, `trace_id`, `servicio`, `instancia`, `componente`, `nivel`, `metodo`, `ruta`, `estado_http`, `codigo`, `mensaje`, `usuario_id` FK, `username`, `ip`, `user_agent`, `stack_trace`, `created_at` |

Reglas de seguridad:

1. `username` y `email` son únicos sin distinguir mayúsculas.
2. La contraseña se almacena únicamente como hash bcrypt (costo 12) y cumple una política mínima de 12 caracteres.
3. Tras 5 intentos fallidos la cuenta se bloquea 15 minutos; un administrador puede desbloquearla.
4. El access token JWT dura 15 minutos; el refresh token dura 7 días, rota en cada uso y se guarda como hash SHA-256.
5. La reutilización de un refresh token revocado invalida todas las sesiones del usuario.
6. Los permisos se validan en el middleware antes de ejecutar el caso de uso.
7. Los cambios de rol, contraseña o estado incrementan `token_version` y revocan los refresh tokens; el access token anterior deja de ser válido de inmediato.
8. Un administrador puede forzar la revocación inmediata de todas las sesiones de un usuario.
9. Se auditan inicios de sesión, bloqueos, refresh, cambios de contraseña, escrituras, accesos denegados y consultas a los paneles de auditoría y errores.
10. Los errores internos y pánicos se registran con `trace_id` y `instancia` en `errores_aplicacion` y en logs JSON; el mismo identificador se devuelve al cliente en `X-Request-ID` y `request_id`.
11. La auditoría y los errores agregados comparten `trace_id`, lo que permite correlacionar un evento de seguridad con su fallo técnico. La retención se aplica automáticamente según la configuración.
12. El registro público valida username, email, documento, edad mínima configurable, contraseña y versión vigente de términos.
13. Una cuenta formada solo por el rol `paciente` requiere `email_verificado_en` para login y refresh; las cuentas internas existentes quedan exentas.
14. Los tokens de `tokens_cuenta` se guardan como SHA-256, vencen y son de un solo uso. Reenviar un token invalida el anterior del mismo tipo.
15. El reset de contraseña verifica el email si faltaba, limpia el bloqueo, incrementa `token_version` y revoca todos los refresh tokens en la misma transacción.
16. Un documento existente sin cuenta no crea otra historia: genera una `solicitud_vinculacion`. La aprobación vincula la cuenta y el rechazo desactiva la cuenta pendiente.
17. Una cuenta y una historia clínica tienen como máximo un vínculo activo. La cuenta de médico se crea por administración y se vincula a un perfil existente.
18. Las cuentas de paciente se vinculan a `pacientes.usuario_id`; los médicos a `medicos.usuario_id`. Los endpoints de portal no aceptan esos IDs desde el cliente.
19. El middleware resuelve un `Actor` por request: usuario, roles, permisos, email verificado y los vínculos `paciente_id`/`medico_id` de perfiles activos. Los servicios de portal usan esos IDs, no parámetros del cliente.
20. Los endpoints administrativos de pacientes y citas exigen un rol `admin`, `recepcion` o `enfermeria`; un médico solo consulta sus citas y los pacientes con relación de atención (`programada`, `confirmada` o `atendida`). Los accesos propios y los intentos denegados se auditan.

---

## 2. Infraestructura física

| Entidad | Propósito | Atributos clave |
|---|---|---|
| **Sede** | Sucursal o establecimiento | `id_sede` PK, `nombre`, `direccion`, `ciudad`, `telefono`, `ruc`, `activa` |
| **Servicio** | Departamento o área (consulta externa, emergencia, laboratorio, hospitalización) | `id_servicio` PK, `id_sede` FK, `nombre`, `tipo`, `piso`, `activo` |
| **Consultorio** | Espacio para consulta ambulatoria | `id_consultorio` PK, `id_servicio` FK, `codigo`, `piso`, `estado` |
| **Habitacion** | Cuarto de hospitalización | `id_habitacion` PK, `id_servicio` FK, `numero`, `tipo` (individual, compartida, UCI), `piso`, `estado` |
| **Cama** | Unidad ocupable dentro de una habitación | `id_cama` PK, `id_habitacion` FK, `codigo`, `tipo`, `estado` (disponible, ocupada, mantenimiento, higienización), UK(`id_habitacion`, `codigo`) |
| **Quirofano** | Sala quirúrgica | `id_quirofano` PK, `id_sede` FK, `nombre`, `estado` |

---

## 3. Agenda y citas

| Entidad | Propósito | Atributos clave |
|---|---|---|
| **HorarioProfesional** | Plantilla de disponibilidad | `id_horario` PK, `id_profesional` FK, `id_consultorio` FK, `dia_semana`, `hora_inicio`, `hora_fin`, `duracion_slot_min`, `vigente_desde`, `vigente_hasta` |
| **Cita** | Reserva de atención | `id_cita` PK, `codigo` UK, `id_paciente` FK, `id_profesional` FK, `id_consultorio` FK, `fecha_hora_inicio`, `duracion_min`, `tipo` (primera vez, control, urgencia), `motivo`, `estado`, `id_cita_origen` FK (reprogramación), `creada_por` FK |
| **ListaEspera** | Pacientes pendientes de cupo | `id_espera` PK, `id_paciente` FK, `id_especialidad` FK, `prioridad`, `fecha_solicitud`, `estado` |
| **BloqueoAgenda** | Ausencias del profesional | `id_bloqueo` PK, `id_profesional` FK, `fecha_inicio`, `fecha_fin`, `motivo` |

**Estados de la cita:** `programada → confirmada → atendida`, con salidas alternas `no_asistio`, `cancelada`, `reprogramada`.

### 3.1 Implementación actual de agenda y derivaciones

Las migraciones 008–011 materializan el modelo operativo de agenda:

| Tabla | Propósito | Atributos clave |
|---|---|---|
| `citas` | Reserva y metadata operativa | `id`, `codigo`, `paciente_id`, `medico_id`, `fecha_hora_inicio`, `duracion_min`, `estado`, `version`, `derivacion_id`, `canal`, `creada_por_usuario_id`, `motivo_cancelacion` |
| `horarios_profesional` | Plantilla semanal de disponibilidad | `medico_id`, `dia_semana`, `hora_inicio`, `hora_fin`, `duracion_slot_min`, vigencia, `activo`, `version` |
| `bloqueos_agenda` | Ausencias y bloqueos puntuales | `medico_id`, `inicio`, `fin`, `motivo`, `creado_por` |
| `derivaciones` | Autorización clínica para especialista | `paciente_id`, `medico_origen_id`, `cita_origen_id`, `especialidad_destino_id`, `medico_destino_id`, `motivo`, `prioridad`, `estado`, vigencia, `version` |
| `excepciones_derivacion` | Autorización administrativa excepcional sin derivación clínica | `cita_id`, `paciente_id`, `especialidad_id`, `medico_id`, `motivo`, `autorizado_por_usuario_id` |
| `notas_cita` | Nota mínima de la consulta | `cita_id` UK, `medico_id`, `motivo`, `evaluacion`, `plan`, `version` |

La entidad conceptual **Derivacion** se implementa como `derivaciones`, con la cita de origen (`cita_origen_id`) como encuentro habilitante. Una derivación solo nace de una cita de Medicina General `atendida`, la especialidad destino debe exigir derivación y una misma derivación no puede sostener dos citas activas o atendidas.

**Fase 4 implementada:** `HorarioProfesional` y `BloqueoAgenda` tienen entidad, store, CRUD administrativo y reglas de solape/versión. `SlotDisponible` y `Disponibilidad` son objetos calculados por `DisponibilidadService`; no se persisten. El servicio convierte las horas locales de `CLINIC_TIMEZONE` a UTC, resta bloqueos y citas `programada`/`confirmada`, y aplica antelación mínima y horizonte máximo.

**Fase 5 implementada:** `Cita` persiste `canal`, `creada_por_usuario_id`, `derivacion_id` y `motivo_cancelacion`. El portal del paciente usa `PortalCitaService`, fuerza `paciente_id` desde el actor y `canal=portal_paciente`, y solo permite especialidades con `requiere_derivacion=false`. La reprogramación usa `version` y conserva paciente/médico; la cancelación respeta `PORTAL_CANCEL_CUTOFF_HOURS`.

**Fase 6 implementada:** `Derivacion`, `NotaCita` y `TransferenciaPaciente` tienen entidad, store y endpoints. `pacientes.medico_responsable_id` representa el médico tratante actual y `transferencias_paciente` conserva el historial inmutable. La relación de acceso del médico incluye sus citas activas/atendidas y la asignación activa; la nota es única por cita y la derivación nace de una cita `MED-GEN` atendida.

**Fase 7 implementada:** la derivación pasa a `agendada` al reservar el especialista, vuelve a `pendiente` si se cancela con antelación, se consume como `cancelada` por cancelación tardía o no-show y se marca `atendida` al atender la cita. `ExcepcionDerivacion` registra una autorización administrativa excepcional sin crear una derivación clínica falsa. La reserva usa bloqueo `FOR UPDATE` y el índice parcial de una cita activa por derivación.

Planificado para la Fase 6: `pacientes.medico_responsable_id` (asignación actual) y `transferencias_paciente` (historial). La transferencia es interna, exige `pacientes:transferir`, motivo y auditoría; el médico nuevo obtiene acceso por la asignación y el anterior conserva solo sus citas vigentes o atendidas.

---

## 4. Historia clínica y atención

| Entidad | Propósito | Atributos clave |
|---|---|---|
| **Antecedente** | Antecedentes patológicos, quirúrgicos, familiares | `id_antecedente` PK, `id_paciente` FK, `tipo`, `descripcion`, `fecha` |
| **Alergia** | Alergias del paciente | `id_alergia` PK, `id_paciente` FK, `alergeno`, `tipo_reaccion`, `severidad`, `activa` |
| **EncuentroAtencion** | Evento clínico central (consulta, emergencia, telemedicina, internación) | `id_encuentro` PK, `id_paciente` FK, `id_cita` FK (opcional), `id_profesional` FK, `id_servicio` FK, `tipo`, `fecha_hora_inicio`, `fecha_hora_fin`, `motivo_consulta`, `enfermedad_actual`, `examen_fisico`, `plan`, `estado` |
| **SignosVitales** | Medición puntual | `id_signos` PK, `id_encuentro` FK, `fecha_hora`, `temperatura`, `fc`, `fr`, `ta_sistolica`, `ta_diastolica`, `spo2`, `peso`, `talla`, `imc`, `escala_dolor`, `registrado_por` FK |
| **CatalogoCIE10** | Catálogo de diagnósticos | `codigo` PK, `descripcion`, `capitulo` |
| **Diagnostico** | Diagnóstico del encuentro | `id_diagnostico` PK, `id_encuentro` FK, `codigo_cie10` FK, `tipo` (presuntivo, definitivo, secundario), `descripcion`, `fecha` |
| **Procedimiento** | Procedimiento realizado | `id_procedimiento` PK, `id_encuentro` FK, `codigo`, `descripcion`, `fecha`, `id_profesional` FK |
| **Receta** | Prescripción emitida | `id_receta` PK, `id_encuentro` FK, `id_profesional` FK, `fecha`, `estado` (emitida, dispensada, anulada), `indicaciones_generales` |
| **DetalleReceta** | Líneas de la receta | `id_detalle` PK, `id_receta` FK, `id_medicamento` FK, `dosis`, `via`, `frecuencia`, `duracion_dias`, `cantidad`, `indicaciones` |
| **OrdenExamen** | Solicitud de estudios | `id_orden` PK, `id_encuentro` FK, `id_profesional` FK, `tipo` (laboratorio, imagen, patología), `prioridad`, `estado`, `fecha` |
| **DetalleOrdenExamen** | Exámenes solicitados | `id_detalle` PK, `id_orden` FK, `id_examen` FK, `indicaciones` |
| **CatalogoExamen** | Catálogo de pruebas | `id_examen` PK, `codigo` UK, `nombre`, `tipo`, `valores_referencia`, `tiempo_entrega_horas` |
| **ResultadoExamen** | Resultado validado | `id_resultado` PK, `id_detalle_orden` FK, `fecha`, `valor`, `unidad`, `rango_referencia`, `interpretacion`, `archivo_url`, `validado_por` FK |
| **Derivacion** | Referencia a otra especialidad | `id_derivacion` PK, `id_encuentro_origen` FK, `id_especialidad_destino` FK, `id_profesional_destino` FK, `motivo`, `prioridad`, `estado`, `fecha` |
| **DocumentoClinico** | Consentimientos, informes, adjuntos | `id_documento` PK, `id_paciente` FK, `id_encuentro` FK, `tipo`, `archivo_url`, `hash`, `fecha` |

---

## 5. Hospitalización y cirugía

| Entidad | Propósito | Atributos clave |
|---|---|---|
| **Hospitalizacion** | Estancia hospitalaria | `id_hospitalizacion` PK, `id_paciente` FK, `id_encuentro` FK (opcional), `id_cama_actual` FK, `id_profesional_tratante` FK, `fecha_hora_ingreso`, `fecha_hora_alta`, `tipo_ingreso`, `motivo`, `estado` |
| **TrasladoCama** | Historial de cambios de cama | `id_traslado` PK, `id_hospitalizacion` FK, `id_cama_origen` FK, `id_cama_destino` FK, `fecha`, `motivo` |
| **NotaEvolucion** | Evolución médica (SOAP) | `id_nota` PK, `id_hospitalizacion` FK, `id_profesional` FK, `fecha`, `subjetivo`, `objetivo`, `analisis`, `plan` |
| **RegistroEnfermeria** | Hoja de enfermería por turno | `id_registro` PK, `id_hospitalizacion` FK, `id_profesional` FK, `turno`, `fecha`, `observaciones` |
| **AdministracionMedicamento** | Registro de dosis administradas | `id_administracion` PK, `id_hospitalizacion` FK, `id_detalle_receta` FK, `fecha_hora`, `dosis`, `via`, `id_enfermero` FK, `estado`, `observacion` |
| **Cirugia** | Intervención quirúrgica | `id_cirugia` PK, `id_hospitalizacion` FK (opcional), `id_quirofano` FK, `id_cirujano_principal` FK, `nombre`, `fecha_inicio`, `fecha_fin`, `tipo`, `estado` |
| **EquipoQuirurgico** | Personal de la cirugía | `id_cirugia` FK, `id_profesional` FK, `rol` (cirujano, anestesiólogo, instrumentador, ayudante), PK compuesta |
| **AltaMedica** | Cierre de la hospitalización | `id_alta` PK, `id_hospitalizacion` FK UK, `fecha`, `tipo` (domicilio, traslado, fallecimiento, voluntario), `diagnostico_egreso`, `recomendaciones`, `id_profesional` FK |

---

## 6. Farmacia e inventario

| Entidad | Propósito | Atributos clave |
|---|---|---|
| **Medicamento** | Catálogo farmacológico | `id_medicamento` PK, `codigo` UK, `nombre_generico`, `nombre_comercial`, `presentacion`, `concentracion`, `forma_farmaceutica`, `via`, `requiere_receta`, `controlado`, `activo` |
| **Lote** | Trazabilidad por lote | `id_lote` PK, `id_medicamento` FK, `numero_lote`, `fecha_vencimiento`, `id_proveedor` FK, `costo_unitario`, UK(`id_medicamento`, `numero_lote`, `fecha_vencimiento`) |
| **Stock** | Existencias por sede | `id_stock` PK, `id_lote` FK, `id_sede` FK, `cantidad`, `stock_minimo`, `ubicacion`, UK(`id_lote`, `id_sede`) |
| **MovimientoInventario** | Kardex de entradas/salidas | `id_movimiento` PK, `id_stock` FK, `tipo` (entrada, salida, ajuste, devolución, baja), `cantidad`, `motivo`, `referencia`, `id_usuario` FK, `fecha` |
| **Dispensacion** | Entrega de medicamentos | `id_dispensacion` PK, `id_receta` FK, `id_farmaceutico` FK, `fecha`, `estado` |
| **DetalleDispensacion** | Ítems entregados | `id_detalle` PK, `id_dispensacion` FK, `id_medicamento` FK, `id_lote` FK, `cantidad` |
| **Proveedor / OrdenCompra / DetalleOrdenCompra** | Abastecimiento (módulo de compras) | proveedor, fechas, cantidades, precios, estado de recepción |

---

## 7. Facturación y seguros

| Entidad | Propósito | Atributos clave |
|---|---|---|
| **Aseguradora** | Entidad de seguro | `id_aseguradora` PK, `nombre`, `ruc`, `contacto` |
| **PlanSeguro** | Plan contratado | `id_plan` PK, `id_aseguradora` FK, `nombre`, `cobertura_pct`, `copago`, `deducible` |
| **CoberturaPaciente** | Póliza vigente del paciente | `id_cobertura` PK, `id_paciente` FK, `id_plan` FK, `numero_poliza`, `titular`, `vigente_desde`, `vigente_hasta`, `estado` |
| **Tarifa** | Precio de servicios | `id_tarifa` PK, `codigo` UK, `descripcion`, `tipo`, `precio`, `vigente_desde`, `vigente_hasta` |
| **Factura** | Documento financiero | `id_factura` PK, `numero` UK, `id_paciente` FK, `id_encuentro` FK, `id_hospitalizacion` FK, `id_cobertura` FK, `fecha_emision`, `subtotal`, `descuento`, `impuesto`, `total`, `estado` |
| **DetalleFactura** | Conceptos facturados | `id_detalle` PK, `id_factura` FK, `id_tarifa` FK, `descripcion`, `cantidad`, `precio_unitario`, `subtotal` |
| **Pago** | Abonos a la factura | `id_pago` PK, `id_factura` FK, `fecha`, `monto`, `metodo`, `referencia`, `id_usuario_caja` FK |
| **AutorizacionSeguro** | Aprobación previa del seguro | `id_autorizacion` PK, `id_cobertura` FK, `id_encuentro` FK, `numero_autorizacion`, `estado`, `monto_aprobado` |

---

## 8. Soporte y auditoría

| Entidad | Propósito | Atributos clave |
|---|---|---|
| **Notificacion** | Avisos a usuarios | `id_notificacion` PK, `id_usuario` FK, `titulo`, `mensaje`, `tipo`, `leida`, `fecha` |
| **Auditoria** | Trazabilidad de cambios | `id_auditoria` PK, `id_usuario` FK, `tabla`, `accion`, `registro_id`, `datos_anteriores` JSON, `datos_nuevos` JSON, `ip`, `fecha` |
| **ErrorAplicacion** | Error correlacionable para agregación de logs | `id_error` PK, `trace_id`, `servicio`, `componente`, `nivel`, `metodo`, `ruta`, `estado_http`, `codigo`, `mensaje`, `id_usuario` FK, `username`, `ip`, `user_agent`, `stack_trace`, `created_at` |
| **Configuracion** | Parámetros del sistema | `clave` PK, `valor`, `descripcion`, `modulo` |

---

## Diagrama del flujo clínico principal

```mermaid
erDiagram
    PERSONA ||--o| PACIENTE : "perfil"
    PERSONA ||--o| EMPLEADO : "perfil"
    EMPLEADO ||--o| PROFESIONAL_SALUD : "es"
    PACIENTE ||--o{ CITA : "agenda"
    PROFESIONAL_SALUD ||--o{ CITA : "atiende"
    PROFESIONAL_SALUD ||--o{ PACIENTE : "responsable"
    CITA ||--o| ENCUENTRO : "genera"
    ENCUENTRO ||--o{ DIAGNOSTICO : "registra"
    ENCUENTRO ||--o{ RECETA : "emite"
    RECETA ||--o{ RECETA_DETALLE : "contiene"
    MEDICAMENTO ||--o{ RECETA_DETALLE : "prescrito"
    ENCUENTRO ||--o{ ORDEN_EXAMEN : "solicita"
    ORDEN_EXAMEN ||--o{ RESULTADO_EXAMEN : "produce"
    PACIENTE ||--o{ HOSPITALIZACION : "ingresa"
    HOSPITALIZACION }o--|| CAMA : "ocupa"
    HOSPITALIZACION ||--o{ NOTA_EVOLUCION : "evoluciona"
    HOSPITALIZACION ||--o| ALTA_MEDICA : "finaliza"
    ENCUENTRO ||--o| FACTURA : "origina"
    PACIENTE ||--o{ FACTURA : "factura"
    FACTURA ||--o{ FACTURA_DETALLE : "detalla"
    FACTURA ||--o{ PAGO : "paga"
```

---

## Reglas de negocio clave

1. **Historia clínica inmutable:** no se actualiza ni elimina; las correcciones se hacen con notas de rectificación auditadas.
2. **Una cama, un paciente:** índice único parcial para garantizar que solo exista una hospitalización activa por cama.
3. **Sin doble reserva:** restricción única sobre profesional + fecha/hora de cita activa.
4. **Cobertura vigente:** la póliza debe estar vigente en la fecha del servicio para aplicar seguro.
5. **Facturas anulables, no borrables:** anulación con motivo, usuario y fecha; el pago genera movimientos inmutables.
6. **Farmacia FEFO:** salida de inventario por lote con fecha de vencimiento más próxima.
7. **Controlados:** medicamentos marcados como controlados exigen receta especial y trazabilidad completa.
8. **Multi-sede:** servicios, camas, stock y caja siempre asociados a una sede.
9. **Auditoría obligatoria** en tablas clínicas, financieras y de seguridad.
10. **Tipos de datos:** montos en `DECIMAL(12,2)`, fechas en UTC, documentos y firmas con hash.
11. **Contraseñas:** nunca se almacenan ni registran en texto plano; solo el hash bcrypt.
12. **Tokens:** el refresh token se almacena hasheado, rota en cada uso y se revoca al cerrar sesión o cambiar credenciales.
13. **Mínimo privilegio:** cada rol recibe solo los permisos necesarios para su función.
14. **Trazabilidad:** toda operación de escritura autenticada, todo intento de acceso denegado y toda consulta a los paneles de auditoría/errores queda en `auditoria` con su `trace_id`.
15. **Revocación inmediata:** el claim `ver` del JWT debe coincidir con `usuarios.token_version`; cualquier cambio de rol, contraseña, estado o revocación explícita invalida los access tokens vigentes.
16. **Observabilidad:** los errores internos se escriben simultáneamente en stdout JSON y en `errores_aplicacion`, correlacionados por `trace_id` e identificados por `instancia`; nunca incluyen contraseñas, tokens ni cuerpos de solicitud.
17. **Una historia, una cuenta:** `pacientes.usuario_id` y `medicos.usuario_id` son únicos; el registro con documento existente no duplica persona ni paciente.
18. **Verificación previa al portal:** una cuenta exclusivamente de paciente no inicia sesión hasta verificar el email. La verificación no sustituye la aprobación de la vinculación cuando la historia ya existía.
19. **Tokens de cuenta de un solo uso:** verificación y reset se consumen con una actualización atómica; un token vencido o reutilizado devuelve `400`.
20. **Reset seguro:** cambiar la contraseña por recuperación revoca refresh tokens e incrementa `token_version`; el access token anterior deja de funcionar de inmediato.
21. **Transferencia interna:** solo `admin` o `recepción` con `pacientes:transferir` cambian el médico tratante. Se registra el origen, destino, motivo, actor y fecha; no se borra la historia ni se transfiere a otra institución.
