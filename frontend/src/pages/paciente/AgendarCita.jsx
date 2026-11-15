import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { ApiError } from "@/lib/http";
import {
  crearCita,
  listarDerivaciones,
  listarDisponibilidad,
  listarEspecialidades,
  listarMedicos,
} from "@/services/agenda";
import SelectorSlot from "./agendar/SelectorSlot";

const HORIZONTE_DIAS = 90;

const TIPOS_CITA = [
  { valor: "primera_vez", etiqueta: "Primera vez" },
  { valor: "control", etiqueta: "Control" },
  { valor: "telemedicina", etiqueta: "Telemedicina" },
];

function fechaISO(fecha) {
  const anio = fecha.getFullYear();
  const mes = String(fecha.getMonth() + 1).padStart(2, "0");
  const dia = String(fecha.getDate()).padStart(2, "0");
  return `${anio}-${mes}-${dia}`;
}

function derivacionVigente(derivacion) {
  if (derivacion.estado !== "pendiente") return false;
  return new Date(derivacion.vigente_hasta).getTime() > Date.now();
}

function mensajeDeError(error) {
  if (!(error instanceof ApiError)) {
    return "Ocurrió un error inesperado. Intenta nuevamente.";
  }
  if (error.isFeatureDisabled) {
    return "El agendamiento no está disponible en este momento.";
  }
  return error.message || "Ocurrió un error inesperado.";
}

const PASOS = ["Especialidad", "Médico", "Horario", "Confirmar"];

export default function AgendarCita() {
  const navigate = useNavigate();

  const [modo, setModo] = useState(null); // "directa" | "derivacion"

  const [especialidades, setEspecialidades] = useState([]);
  const [derivaciones, setDerivaciones] = useState([]);
  const [cargandoCatalogo, setCargandoCatalogo] = useState(true);
  const [errorCatalogo, setErrorCatalogo] = useState(null);

  const [especialidad, setEspecialidad] = useState(null);
  const [derivacion, setDerivacion] = useState(null);
  const [medico, setMedico] = useState(null);
  const [slot, setSlot] = useState(null);

  const [medicos, setMedicos] = useState([]);
  const [disponibilidad, setDisponibilidad] = useState(null);
  const [cargandoOpciones, setCargandoOpciones] = useState(false);
  const [errorPaso, setErrorPaso] = useState(null);

  const [tipo, setTipo] = useState("primera_vez");
  const [motivo, setMotivo] = useState("");
  const [guardando, setGuardando] = useState(false);
  const [alertaSlot, setAlertaSlot] = useState(null);

  useEffect(() => {
    let activo = true;

    Promise.all([listarEspecialidades(), listarDerivaciones()])
      .then(([especialidadesResp, derivacionesResp]) => {
        if (!activo) return;
        setEspecialidades(especialidadesResp);
        setDerivaciones(derivacionesResp);
      })
      .catch((error) => {
        if (activo) setErrorCatalogo(mensajeDeError(error));
      })
      .finally(() => {
        if (activo) setCargandoCatalogo(false);
      });

    return () => {
      activo = false;
    };
  }, []);

  const especialidadesDirectas = useMemo(
    () => especialidades.filter((e) => !e.requiere_derivacion),
    [especialidades],
  );

  const derivacionesPendientes = useMemo(
    () => derivaciones.filter((d) => d.estado === "pendiente"),
    [derivaciones],
  );

  async function cargarDisponibilidad(medicoID) {
    setCargandoOpciones(true);
    setErrorPaso(null);
    setSlot(null);
    setAlertaSlot(null);
    try {
      const hoy = new Date();
      const hasta = new Date(hoy.getTime() + HORIZONTE_DIAS * 24 * 60 * 60 * 1000);
      const datos = await listarDisponibilidad(
        medicoID,
        fechaISO(hoy),
        fechaISO(hasta),
      );
      setDisponibilidad(datos);
    } catch (error) {
      setErrorPaso(mensajeDeError(error));
      setDisponibilidad(null);
    } finally {
      setCargandoOpciones(false);
    }
  }

  async function seleccionarEspecialidad(especialidadElegida) {
    setEspecialidad(especialidadElegida);
    setMedico(null);
    setSlot(null);
    setDisponibilidad(null);
    setErrorPaso(null);
    setAlertaSlot(null);
    setCargandoOpciones(true);
    try {
      const medicosResp = await listarMedicos(especialidadElegida.id);
      setMedicos(medicosResp);
    } catch (error) {
      setErrorPaso(mensajeDeError(error));
      setMedicos([]);
    } finally {
      setCargandoOpciones(false);
    }
  }

  async function seleccionarDerivacion(derivacionElegida) {
    setDerivacion(derivacionElegida);
    setEspecialidad(null);
    setMedico(null);
    setSlot(null);
    setDisponibilidad(null);
    setErrorPaso(null);
    setAlertaSlot(null);
    setCargandoOpciones(true);
    try {
      const medicosResp = await listarMedicos(derivacionElegida.especialidad_destino_id);
      setMedicos(medicosResp);
    } catch (error) {
      setErrorPaso(mensajeDeError(error));
      setMedicos([]);
    } finally {
      setCargandoOpciones(false);
    }
  }

  async function seleccionarMedico(medicoElegido) {
    setMedico(medicoElegido);
    await cargarDisponibilidad(medicoElegido.id);
  }

  async function confirmar() {
    if (!medico || !slot || !motivo.trim()) return;

    setGuardando(true);
    setAlertaSlot(null);
    setErrorPaso(null);

    const payload = {
      medico_id: medico.id,
      fecha_hora_inicio: slot.inicio,
      tipo,
      motivo: motivo.trim(),
    };
    if (derivacion) {
      payload.derivacion_id = derivacion.id;
    }

    try {
      const resultado = await crearCita(payload);
      if (resultado.status === 201) {
        navigate("/paciente/citas");
        return;
      }
      setErrorPaso("El servidor devolvió una respuesta inesperada.");
    } catch (error) {
      if (error instanceof ApiError && error.isConflict) {
        setAlertaSlot(
          "El horario seleccionado fue ocupado por otro paciente mientras reservabas. Se refrescó la disponibilidad; elige otro horario.",
        );
        setSlot(null);
        // Refrescar automáticamente la disponibilidad del médico elegido.
        await cargarDisponibilidad(medico.id);
      } else {
        setErrorPaso(mensajeDeError(error));
      }
    } finally {
      setGuardando(false);
    }
  }

  function reiniciar() {
    setModo(null);
    setEspecialidad(null);
    setDerivacion(null);
    setMedico(null);
    setSlot(null);
    setMedicos([]);
    setDisponibilidad(null);
    setErrorPaso(null);
    setAlertaSlot(null);
    setMotivo("");
  }

  if (cargandoCatalogo) {
    return (
      <div className="flex h-64 items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
      </div>
    );
  }

  if (errorCatalogo) {
    return (
      <div className="rounded-xl border border-red-200 bg-red-50 p-6 text-sm text-red-700">
        {errorCatalogo}
      </div>
    );
  }

  const paso = modo === null ? 0 : !medico ? 2 : !slot ? 3 : 4;

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <header className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-slate-900">Agendar cita</h1>
          <p className="mt-1 text-sm text-slate-500">
            Elige el origen de tu cita y sigue los pasos.
          </p>
        </div>
        {modo && (
          <button
            type="button"
            onClick={reiniciar}
            className="text-sm font-medium text-brand-600 hover:text-brand-700"
          >
            Reiniciar
          </button>
        )}
      </header>

      {alertaSlot && (
        <p
          role="alert"
          className="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800"
        >
          {alertaSlot}
        </p>
      )}

      {errorPaso && (
        <p
          role="alert"
          className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700"
        >
          {errorPaso}
        </p>
      )}

      {modo === null ? (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <button
            type="button"
            onClick={() => setModo("directa")}
            className="rounded-2xl border border-slate-200 bg-white p-6 text-left shadow-sm transition hover:border-brand-400"
          >
            <p className="text-lg font-semibold text-slate-900">Agendamiento directo</p>
            <p className="mt-2 text-sm text-slate-500">
              Medicina General, Fisioterapia, Odontología, Nutrición y Pediatría.
            </p>
          </button>
          <button
            type="button"
            onClick={() => setModo("derivacion")}
            className="rounded-2xl border border-slate-200 bg-white p-6 text-left shadow-sm transition hover:border-brand-400"
          >
            <p className="text-lg font-semibold text-slate-900">Con derivación</p>
            <p className="mt-2 text-sm text-slate-500">
              Reserva la cita con el especialista usando una derivación vigente.
            </p>
          </button>
        </div>
      ) : (
        <>
          <ol className="flex flex-wrap items-center gap-2 text-xs font-medium text-slate-500">
            {PASOS.map((nombre, indice) => (
              <li key={nombre} className="flex items-center gap-2">
                <span
                  className={`flex h-6 w-6 items-center justify-center rounded-full ${
                    indice + 1 <= paso
                      ? "bg-brand-600 text-white"
                      : "bg-slate-200 text-slate-600"
                  }`}
                >
                  {indice + 1}
                </span>
                {nombre}
                {indice < PASOS.length - 1 && (
                  <span className="text-slate-300">→</span>
                )}
              </li>
            ))}
          </ol>

          {/* Paso 1: especialidad directa o derivación */}
          {modo === "directa" && !especialidad && (
            <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
              <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-slate-500">
                Elige una especialidad
              </h2>
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                {especialidadesDirectas.map((item) => (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => seleccionarEspecialidad(item)}
                    className="rounded-xl border border-slate-200 bg-white p-4 text-left shadow-sm transition hover:border-brand-400"
                  >
                    <p className="font-semibold text-slate-900">{item.nombre}</p>
                    <p className="mt-1 text-xs text-slate-500">
                      {item.duracion_cita_min} min por cita
                    </p>
                  </button>
                ))}
              </div>
            </section>
          )}

          {modo === "derivacion" && !derivacion && (
            <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
              <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-slate-500">
                Elige una derivación vigente
              </h2>

              {derivacionesPendientes.length === 0 && (
                <p className="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800">
                  No tienes derivaciones pendientes. Puedes usar el agendamiento
                  directo o consultar tus derivaciones.
                </p>
              )}

              <div className="grid grid-cols-1 gap-3">
                {derivaciones.map((item) => {
                  const vigente = derivacionVigente(item);
                  const motivoInvalida = item.estado === "vencida"
                    ? "Derivación vencida"
                    : item.estado === "agendada"
                      ? "Derivación ya utilizada"
                      : item.estado === "cancelada"
                        ? "Derivación cancelada"
                        : null;

                  return (
                    <div
                      key={item.id}
                      className={`rounded-xl border p-4 ${
                        vigente
                          ? "border-slate-200 bg-white"
                          : "border-red-200 bg-red-50"
                      }`}
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div>
                          <p className="font-semibold text-slate-900">
                            {item.especialidad_destino}
                          </p>
                          <p className="mt-1 text-sm text-slate-500">
                            Origen: {item.medico_origen_nombre}
                          </p>
                        </div>
                        {vigente ? (
                          <button
                            type="button"
                            onClick={() => seleccionarDerivacion(item)}
                            className="shrink-0 rounded-lg bg-brand-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-brand-700"
                          >
                            Seleccionar
                          </button>
                        ) : (
                          <span className="shrink-0 rounded-lg bg-red-100 px-3 py-1.5 text-xs font-semibold text-red-700">
                            {motivoInvalida ?? `Estado: ${item.estado}`}
                          </span>
                        )}
                      </div>
                      {!vigente && (
                        <p className="mt-2 text-xs text-red-600">
                          Esta derivación no puede usarse para reservar.
                        </p>
                      )}
                    </div>
                  );
                })}
              </div>
            </section>
          )}

          {/* Paso 2: médico */}
          {(especialidad || derivacion) && !medico && (
            <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
              <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-slate-500">
                Elige un profesional
              </h2>

              {cargandoOpciones && (
                <div className="flex justify-center py-6">
                  <div className="h-6 w-6 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
                </div>
              )}

              {!cargandoOpciones && medicos.length === 0 && (
                <p className="text-sm text-slate-500">
                  No hay profesionales disponibles para esta especialidad.
                </p>
              )}

              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                {medicos.map((item) => (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => seleccionarMedico(item)}
                    className="rounded-xl border border-slate-200 bg-white p-4 text-left shadow-sm transition hover:border-brand-400"
                  >
                    <p className="font-semibold text-slate-900">
                      {item.nombres} {item.apellidos}
                    </p>
                    <p className="mt-1 text-xs text-slate-500">
                      {item.especialidad.nombre} · {item.duracion_cita_min} min
                    </p>
                  </button>
                ))}
              </div>
            </section>
          )}

          {/* Paso 3: horario */}
          {medico && !slot && (
            <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
              <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-slate-500">
                Elige un horario
              </h2>

              {cargandoOpciones && (
                <div className="flex justify-center py-6">
                  <div className="h-6 w-6 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
                </div>
              )}

              {!cargandoOpciones && disponibilidad && (
                <SelectorSlot
                  slots={disponibilidad.slots}
                  onSelect={setSlot}
                  selectedInicio={slot?.inicio ?? null}
                />
              )}
            </section>
          )}

          {/* Paso 4: confirmación */}
          {slot && (
            <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
              <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-slate-500">
                Confirma tu cita
              </h2>

              <dl className="grid grid-cols-1 gap-3 rounded-xl bg-slate-50 p-4 text-sm sm:grid-cols-2">
                <div>
                  <dt className="text-slate-500">Especialidad</dt>
                  <dd className="font-medium text-slate-900">
                    {medico.especialidad.nombre}
                  </dd>
                </div>
                <div>
                  <dt className="text-slate-500">Profesional</dt>
                  <dd className="font-medium text-slate-900">
                    {medico.nombres} {medico.apellidos}
                  </dd>
                </div>
                <div>
                  <dt className="text-slate-500">Horario</dt>
                  <dd className="font-medium text-slate-900">
                    {new Date(slot.inicio).toLocaleString("es-EC", {
                      timeZone: "America/Guayaquil",
                      dateStyle: "full",
                      timeStyle: "short",
                    })}
                  </dd>
                </div>
                {derivacion && (
                  <div>
                    <dt className="text-slate-500">Derivación</dt>
                    <dd className="font-medium text-slate-900">
                      {derivacion.especialidad_destino}
                    </dd>
                  </div>
                )}
              </dl>

              <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div>
                  <label htmlFor="tipo" className="mb-1 block text-sm font-medium text-slate-700">
                    Tipo de cita
                  </label>
                  <select
                    id="tipo"
                    value={tipo}
                    onChange={(e) => setTipo(e.target.value)}
                    className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 outline-none focus:border-brand-500 focus:ring-2 focus:ring-brand-100"
                  >
                    {TIPOS_CITA.map((item) => (
                      <option key={item.valor} value={item.valor}>
                        {item.etiqueta}
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <label htmlFor="motivo" className="mb-1 block text-sm font-medium text-slate-700">
                    Motivo
                  </label>
                  <input
                    id="motivo"
                    type="text"
                    value={motivo}
                    onChange={(e) => setMotivo(e.target.value)}
                    className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 outline-none focus:border-brand-500 focus:ring-2 focus:ring-brand-100"
                    placeholder="Motivo de la consulta"
                  />
                </div>
              </div>

              <div className="mt-6 flex items-center justify-end gap-3">
                <button
                  type="button"
                  onClick={() => setSlot(null)}
                  className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
                >
                  Cambiar horario
                </button>
                <button
                  type="button"
                  onClick={confirmar}
                  disabled={guardando || !motivo.trim()}
                  className="rounded-lg bg-brand-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:bg-brand-700 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {guardando ? "Reservando…" : "Confirmar cita"}
                </button>
              </div>
            </section>
          )}
        </>
      )}
    </div>
  );
}
