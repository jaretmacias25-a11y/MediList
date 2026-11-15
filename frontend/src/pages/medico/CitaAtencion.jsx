import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { ApiError } from "@/lib/http";
import {
  cambiarEstado,
  crearDerivacion,
  guardarNota,
  listarEspecialidadesClinicas,
  listarMedicosClinicos,
  obtenerCita,
  obtenerNota,
} from "@/services/medico";
import {
  esTerminal,
  etiquetaEstado,
  etiquetaTipo,
  formatoFechaGuayaquil,
} from "@/pages/paciente/citas/etiquetas";

// Transiciones clínicas permitidas al médico (coinciden con el backend).
const ACCIONES_POR_ESTADO = {
  programada: [
    { estado: "confirmada", etiqueta: "Confirmar" },
    { estado: "atendida", etiqueta: "Marcar atendida" },
    { estado: "no_asistio", etiqueta: "Marcar no asistió" },
  ],
  confirmada: [
    { estado: "atendida", etiqueta: "Marcar atendida" },
    { estado: "no_asistio", etiqueta: "Marcar no asistió" },
  ],
};

const inputClass =
  "w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 shadow-sm outline-none transition focus:border-brand-500 focus:ring-2 focus:ring-brand-100";

export default function CitaAtencion() {
  const { id } = useParams();

  const [cita, setCita] = useState(null);
  const [cargando, setCargando] = useState(true);
  const [denegada, setDenegada] = useState(false);
  const [error, setError] = useState(null);
  const [alerta, setAlerta] = useState(null);
  const [exito, setExito] = useState(null);
  const [guardando, setGuardando] = useState(false);

  // Nota clínica
  const [nota, setNota] = useState(null);
  const [notaCargando, setNotaCargando] = useState(false);
  const [notaForm, setNotaForm] = useState({ motivo: "", evaluacion: "", plan: "" });
  const [guardandoNota, setGuardandoNota] = useState(false);

  // Derivación
  const [mostrarDerivacion, setMostrarDerivacion] = useState(false);
  const [especialidadesDestino, setEspecialidadesDestino] = useState([]);
  const [medicosDestino, setMedicosDestino] = useState([]);
  const [especialidadDestinoID, setEspecialidadDestinoID] = useState("");
  const [medicoDestinoID, setMedicoDestinoID] = useState("");
  const [motivoDerivacion, setMotivoDerivacion] = useState("");
  const [prioridad, setPrioridad] = useState("normal");
  const [guardandoDerivacion, setGuardandoDerivacion] = useState(false);

  async function cargarCita() {
    setCargando(true);
    setError(null);
    setDenegada(false);
    try {
      const datos = await obtenerCita(Number(id));
      setCita(datos);
    } catch (err) {
      if (err instanceof ApiError && err.isForbidden) {
        setDenegada(true);
      } else {
        setError(err instanceof ApiError ? err.message : "No se pudo cargar la cita.");
      }
    } finally {
      setCargando(false);
    }
  }

  async function cargarNota() {
    setNotaCargando(true);
    try {
      const datos = await obtenerNota(Number(id));
      setNota(datos);
      if (datos) {
        setNotaForm({
          motivo: datos.motivo,
          evaluacion: datos.evaluacion,
          plan: datos.plan,
        });
      }
    } catch (err) {
      setAlerta(err instanceof ApiError ? err.message : "No se pudo cargar la nota.");
    } finally {
      setNotaCargando(false);
    }
  }

  useEffect(() => {
    cargarCita();
    cargarNota();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  async function cambiarEstadoCita(nuevoEstado) {
    setGuardando(true);
    setAlerta(null);
    setExito(null);
    try {
      const actualizada = await cambiarEstado(Number(id), nuevoEstado);
      setCita(actualizada);
      setExito("Estado de la cita actualizado.");
    } catch (err) {
      if (err instanceof ApiError && err.isConflict) {
        setAlerta("La cita fue modificada en otra sesión. Se recargaron los datos.");
        await cargarCita();
      } else {
        setAlerta(err instanceof ApiError ? err.message : "No se pudo actualizar el estado.");
      }
    } finally {
      setGuardando(false);
    }
  }

  async function guardarNotaClinica() {
    setGuardandoNota(true);
    setAlerta(null);
    setExito(null);
    const esActualizacion = nota !== null;
    try {
      const guardada = await guardarNota(
        Number(id),
        {
          motivo: notaForm.motivo.trim(),
          evaluacion: notaForm.evaluacion.trim(),
          plan: notaForm.plan.trim(),
          version: nota?.version ?? 0,
        },
        esActualizacion,
      );
      setNota(guardada);
      setNotaForm({
        motivo: guardada.motivo,
        evaluacion: guardada.evaluacion,
        plan: guardada.plan,
      });
      setExito(esActualizacion ? "Nota actualizada." : "Nota registrada.");
    } catch (err) {
      if (err instanceof ApiError && err.isConflict) {
        setAlerta(
          "La nota fue modificada en otra sesión. Se recargó la versión actual.",
        );
        await cargarNota();
      } else {
        setAlerta(err instanceof ApiError ? err.message : "No se pudo guardar la nota.");
      }
    } finally {
      setGuardandoNota(false);
    }
  }

  async function abrirDerivacion() {
    setMostrarDerivacion(true);
    setMotivoDerivacion("");
    setMedicoDestinoID("");
    setPrioridad("normal");
    setAlerta(null);
    try {
      const especialidades = await listarEspecialidadesClinicas();
      const destinos = especialidades.filter(
        (e) => e.requiere_derivacion && e.activa && e.codigo !== "MED-GEN",
      );
      setEspecialidadesDestino(destinos);
      setEspecialidadDestinoID(destinos[0]?.id ? String(destinos[0].id) : "");
      const medicos = await listarMedicosClinicos();
      setMedicosDestino(medicos.filter((m) => m.activo));
    } catch (err) {
      setAlerta(err instanceof ApiError ? err.message : "No se pudo preparar la derivación.");
    }
  }

  const medicosDeEspecialidad = especialidadDestinoID
    ? medicosDestino.filter(
        (m) => String(m.especialidad.id) === especialidadDestinoID,
      )
    : [];

  async function confirmarDerivacion() {
    if (!especialidadDestinoID || !motivoDerivacion.trim()) return;
    setGuardandoDerivacion(true);
    setAlerta(null);
    try {
      await crearDerivacion(Number(id), {
        especialidad_destino_id: Number(especialidadDestinoID),
        medico_destino_id: medicoDestinoID ? Number(medicoDestinoID) : null,
        motivo: motivoDerivacion.trim(),
        prioridad,
      });
      setMostrarDerivacion(false);
      setExito("Derivación generada correctamente.");
    } catch (err) {
      setAlerta(err instanceof ApiError ? err.message : "No se pudo generar la derivación.");
    } finally {
      setGuardandoDerivacion(false);
    }
  }

  if (cargando) {
    return (
      <div className="flex h-64 items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
      </div>
    );
  }

  // 403: la cita pertenece a otro médico. No se muestra información.
  if (denegada) {
    return (
      <div className="rounded-xl border border-red-200 bg-red-50 p-6 text-sm text-red-700">
        <p className="font-semibold">Acceso Denegado / Cita Ajena</p>
        <p className="mt-1">
          Esta cita no pertenece a tu agenda. No se muestra información clínica.
        </p>
        <Link to="/medico/agenda" className="mt-3 inline-block font-medium text-brand-600 hover:text-brand-700">
          Volver a mi agenda
        </Link>
      </div>
    );
  }

  if (error || !cita) {
    return (
      <div className="rounded-xl border border-red-200 bg-red-50 p-6 text-sm text-red-700">
        {error ?? "Cita no encontrada."}
      </div>
    );
  }

  const estado = etiquetaEstado(cita.estado);
  const acciones = ACCIONES_POR_ESTADO[cita.estado] ?? [];
  const atendida = cita.estado === "atendida";
  const esMedicinaGeneral =
    (cita.especialidad_nombre ?? "").trim().toLowerCase() === "medicina general";

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <header>
        <Link to="/medico/agenda" className="text-sm font-medium text-brand-600 hover:text-brand-700">
          ← Volver a mi agenda
        </Link>
        <div className="mt-2 flex flex-wrap items-center gap-3">
          <h1 className="text-xl font-bold text-slate-900">Cita {cita.codigo}</h1>
          <span className={`rounded-full px-2.5 py-1 text-xs font-semibold ${estado.clase}`}>
            {estado.texto}
          </span>
        </div>
      </header>

      {exito && (
        <p role="status" className="rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-800">
          {exito}
        </p>
      )}

      {alerta && (
        <p role="alert" className="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800">
          {alerta}
        </p>
      )}

      <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
        <dl className="grid grid-cols-1 gap-4 text-sm sm:grid-cols-2">
          <div>
            <dt className="text-slate-500">Paciente</dt>
            <dd className="font-medium text-slate-900">{cita.paciente_nombre}</dd>
          </div>
          <div>
            <dt className="text-slate-500">Especialidad</dt>
            <dd className="font-medium text-slate-900">{cita.especialidad_nombre}</dd>
          </div>
          <div>
            <dt className="text-slate-500">Fecha y hora</dt>
            <dd className="font-medium text-slate-900">
              {formatoFechaGuayaquil(cita.fecha_hora_inicio)}
            </dd>
          </div>
          <div>
            <dt className="text-slate-500">Modalidad</dt>
            <dd className="font-medium text-slate-900">{etiquetaTipo(cita.tipo)}</dd>
          </div>
          <div className="sm:col-span-2">
            <dt className="text-slate-500">Motivo</dt>
            <dd className="font-medium text-slate-900">{cita.motivo || "—"}</dd>
          </div>
          {cita.derivacion_id && (
            <div className="sm:col-span-2">
              <span className="rounded-full bg-violet-100 px-2.5 py-0.5 text-xs font-medium text-violet-800">
                Cita con derivación
              </span>
            </div>
          )}
        </dl>

        {acciones.length > 0 && (
          <div className="mt-6 flex flex-wrap gap-3">
            {acciones.map((accion) => (
              <button
                key={accion.estado}
                type="button"
                disabled={guardando}
                onClick={() => cambiarEstadoCita(accion.estado)}
                className={`rounded-lg px-4 py-2 text-sm font-semibold text-white shadow-sm transition disabled:cursor-not-allowed disabled:opacity-60 ${
                  accion.estado === "atendida"
                    ? "bg-emerald-600 hover:bg-emerald-700"
                    : accion.estado === "no_asistio"
                      ? "bg-red-600 hover:bg-red-700"
                      : "bg-brand-600 hover:bg-brand-700"
                }`}
              >
                {accion.etiqueta}
              </button>
            ))}
          </div>
        )}

        {esTerminal(cita.estado) && (
          <p className="mt-4 text-sm text-slate-500">
            Esta cita no admite más cambios de estado.
          </p>
        )}
      </section>

      {/* Nota clínica */}
      <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
        <h2 className="mb-1 text-sm font-semibold uppercase tracking-wide text-slate-500">
          Nota clínica
        </h2>

        {!atendida && (
          <p className="mt-2 rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-sm text-slate-500">
            La nota solo se puede registrar cuando la cita está en estado
            "atendida".
          </p>
        )}

        {notaCargando ? (
          <div className="flex justify-center py-6">
            <div className="h-6 w-6 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
          </div>
        ) : (
          <div className="mt-4 grid grid-cols-1 gap-4">
            <div>
              <label htmlFor="nota-motivo" className="mb-1 block text-sm font-medium text-slate-700">
                Motivo
              </label>
              <textarea
                id="nota-motivo"
                rows={2}
                disabled={!atendida || guardandoNota}
                value={notaForm.motivo}
                onChange={(e) => setNotaForm((f) => ({ ...f, motivo: e.target.value }))}
                className={`${inputClass} disabled:bg-slate-100 disabled:text-slate-500`}
              />
            </div>
            <div>
              <label htmlFor="nota-evaluacion" className="mb-1 block text-sm font-medium text-slate-700">
                Evaluación
              </label>
              <textarea
                id="nota-evaluacion"
                rows={3}
                disabled={!atendida || guardandoNota}
                value={notaForm.evaluacion}
                onChange={(e) => setNotaForm((f) => ({ ...f, evaluacion: e.target.value }))}
                className={`${inputClass} disabled:bg-slate-100 disabled:text-slate-500`}
              />
            </div>
            <div>
              <label htmlFor="nota-plan" className="mb-1 block text-sm font-medium text-slate-700">
                Plan
              </label>
              <textarea
                id="nota-plan"
                rows={3}
                disabled={!atendida || guardandoNota}
                value={notaForm.plan}
                onChange={(e) => setNotaForm((f) => ({ ...f, plan: e.target.value }))}
                className={`${inputClass} disabled:bg-slate-100 disabled:text-slate-500`}
              />
            </div>

            {atendida && (
              <div className="flex justify-end">
                <button
                  type="button"
                  disabled={guardandoNota}
                  onClick={guardarNotaClinica}
                  className="rounded-lg bg-brand-600 px-4 py-2 text-sm font-semibold text-white hover:bg-brand-700 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {guardandoNota
                    ? "Guardando…"
                    : nota
                      ? "Actualizar nota"
                      : "Registrar nota"}
                </button>
              </div>
            )}
          </div>
        )}
      </section>

      {/* Derivación */}
      {atendida && esMedicinaGeneral && (
        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <h2 className="text-sm font-semibold uppercase tracking-wide text-slate-500">
            Derivación
          </h2>
          <p className="mt-2 text-sm text-slate-500">
            Genera una derivación para esta cita de Medicina General atendida.
          </p>
          <div className="mt-4 flex flex-wrap items-center gap-3">
            <button
              type="button"
              onClick={abrirDerivacion}
              className="rounded-lg bg-violet-600 px-4 py-2 text-sm font-semibold text-white hover:bg-violet-700"
            >
              Generar Derivación
            </button>
            <Link
              to="/medico/derivaciones"
              className="text-sm font-medium text-brand-600 hover:text-brand-700"
            >
              Ver derivaciones emitidas
            </Link>
          </div>
        </section>
      )}

      {/* Modal de derivación */}
      {mostrarDerivacion && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div
            className="absolute inset-0 bg-slate-900/40"
            onClick={() => setMostrarDerivacion(false)}
          />
          <div className="relative max-h-[90vh] w-full max-w-lg overflow-y-auto rounded-2xl bg-white p-6 shadow-xl">
            <h2 className="text-lg font-bold text-slate-900">Generar derivación</h2>

            <div className="mt-4 grid grid-cols-1 gap-4">
              <div>
                <label htmlFor="deriv-especialidad" className="mb-1 block text-sm font-medium text-slate-700">
                  Especialidad destino
                </label>
                <select
                  id="deriv-especialidad"
                  value={especialidadDestinoID}
                  onChange={(e) => {
                    setEspecialidadDestinoID(e.target.value);
                    setMedicoDestinoID("");
                  }}
                  className={inputClass}
                >
                  {especialidadesDestino.map((especialidad) => (
                    <option key={especialidad.id} value={String(especialidad.id)}>
                      {especialidad.nombre}
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label htmlFor="deriv-medico" className="mb-1 block text-sm font-medium text-slate-700">
                  Médico destino (opcional)
                </label>
                <select
                  id="deriv-medico"
                  value={medicoDestinoID}
                  onChange={(e) => setMedicoDestinoID(e.target.value)}
                  className={inputClass}
                >
                  <option value="">Sin médico específico</option>
                  {medicosDeEspecialidad.map((medico) => (
                    <option key={medico.id} value={String(medico.id)}>
                      {medico.persona.nombres} {medico.persona.apellidos}
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label htmlFor="deriv-motivo" className="mb-1 block text-sm font-medium text-slate-700">
                  Motivo
                </label>
                <textarea
                  id="deriv-motivo"
                  rows={3}
                  value={motivoDerivacion}
                  onChange={(e) => setMotivoDerivacion(e.target.value)}
                  className={inputClass}
                  placeholder="Motivo de la derivación"
                />
              </div>

              <div>
                <span className="mb-1 block text-sm font-medium text-slate-700">Prioridad</span>
                <div className="flex gap-3">
                  {["normal", "urgente"].map((opcion) => (
                    <label key={opcion} className="flex items-center gap-2 text-sm text-slate-700">
                      <input
                        type="radio"
                        name="prioridad"
                        checked={prioridad === opcion}
                        onChange={() => setPrioridad(opcion)}
                        className="h-4 w-4 text-brand-600 focus:ring-brand-500"
                      />
                      {opcion === "normal" ? "Normal" : "Urgente"}
                    </label>
                  ))}
                </div>
              </div>
            </div>

            <div className="mt-6 flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setMostrarDerivacion(false)}
                className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
              >
                Cancelar
              </button>
              <button
                type="button"
                disabled={guardandoDerivacion || !especialidadDestinoID || !motivoDerivacion.trim()}
                onClick={confirmarDerivacion}
                className="rounded-lg bg-violet-600 px-4 py-2 text-sm font-semibold text-white hover:bg-violet-700 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {guardandoDerivacion ? "Generando…" : "Generar derivación"}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
