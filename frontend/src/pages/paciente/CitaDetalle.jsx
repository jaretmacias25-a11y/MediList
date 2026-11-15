import { useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { ApiError } from "@/lib/http";
import {
  cancelarCita,
  obtenerCita,
  reprogramarCita,
} from "@/services/citas";
import { listarDisponibilidad } from "@/services/agenda";
import SelectorSlot from "./agendar/SelectorSlot";
import {
  esTerminal,
  etiquetaEstado,
  etiquetaTipo,
  formatoFechaGuayaquil,
} from "./citas/etiquetas";

const HORIZONTE_DIAS = 90;
const CUTOFF_HORAS = Number(import.meta.env.VITE_PORTAL_CANCEL_CUTOFF_HOURS ?? 24);

function fechaISO(fecha) {
  const anio = fecha.getFullYear();
  const mes = String(fecha.getMonth() + 1).padStart(2, "0");
  const dia = String(fecha.getDate()).padStart(2, "0");
  return `${anio}-${mes}-${dia}`;
}

export default function CitaDetalle() {
  const { id } = useParams();

  const [cita, setCita] = useState(null);
  const [cargando, setCargando] = useState(true);
  const [error, setError] = useState(null);

  const [mostrarReprogramar, setMostrarReprogramar] = useState(false);
  const [disponibilidad, setDisponibilidad] = useState(null);
  const [slotNuevo, setSlotNuevo] = useState(null);
  const [motivoReprogramar, setMotivoReprogramar] = useState("");
  const [guardandoReprogramar, setGuardandoReprogramar] = useState(false);

  const [mostrarCancelar, setMostrarCancelar] = useState(false);
  const [motivoCancelar, setMotivoCancelar] = useState("");
  const [guardandoCancelar, setGuardandoCancelar] = useState(false);

  const [alerta, setAlerta] = useState(null);
  const [exito, setExito] = useState(null);

  async function cargarCita() {
    setCargando(true);
    setError(null);
    try {
      const datos = await obtenerCita(Number(id));
      setCita(datos);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "No se pudo cargar la cita.");
    } finally {
      setCargando(false);
    }
  }

  useEffect(() => {
    cargarCita();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  async function cargarDisponibilidad() {
    setError(null);
    setSlotNuevo(null);
    try {
      const hoy = new Date();
      const hasta = new Date(hoy.getTime() + HORIZONTE_DIAS * 24 * 60 * 60 * 1000);
      const datos = await listarDisponibilidad(
        cita.medico_id,
        fechaISO(hoy),
        fechaISO(hasta),
      );
      setDisponibilidad(datos);
    } catch (err) {
      setAlerta(err instanceof ApiError ? err.message : "No se pudo cargar la disponibilidad.");
      setDisponibilidad(null);
    }
  }

  async function abrirReprogramar() {
    setAlerta(null);
    setExito(null);
    setMotivoReprogramar("");
    setMostrarReprogramar(true);
    await cargarDisponibilidad();
  }

  async function confirmarReprogramar() {
    if (!slotNuevo) return;
    setGuardandoReprogramar(true);
    setAlerta(null);
    try {
      const actualizada = await reprogramarCita(Number(id), {
        fecha_hora_inicio: slotNuevo.inicio,
        motivo: motivoReprogramar.trim() || undefined,
        version: cita.version,
      });
      setCita(actualizada);
      setMostrarReprogramar(false);
      setSlotNuevo(null);
      setExito("Cita reprogramada correctamente.");
    } catch (err) {
      if (err instanceof ApiError && err.isConflict) {
        setAlerta(
          "La cita cambió en otra sesión o el horario fue ocupado. Se recargaron los datos; elige otro horario.",
        );
        // Recargar cita y disponibilidad automáticamente.
        await cargarCita();
        await cargarDisponibilidad();
      } else {
        setAlerta(err instanceof ApiError ? err.message : "No se pudo reprogramar la cita.");
      }
    } finally {
      setGuardandoReprogramar(false);
    }
  }

  const dentroDelCorte = useMemo(() => {
    if (!cita?.derivacion_id) return false;
    return Date.now() + CUTOFF_HORAS * 60 * 60 * 1000 >
      new Date(cita.fecha_hora_inicio).getTime();
  }, [cita]);

  async function confirmarCancelar() {
    setGuardandoCancelar(true);
    setAlerta(null);
    try {
      const actualizada = await cancelarCita(Number(id), motivoCancelar.trim());
      setCita(actualizada);
      setMostrarCancelar(false);
      setMotivoCancelar("");
      setExito("Cita cancelada.");
    } catch (err) {
      if (err instanceof ApiError && err.isConflict) {
        setAlerta(err.message || "La cita cambió en otra sesión; se recargaron los datos.");
        setMostrarCancelar(false);
        await cargarCita();
      } else {
        setAlerta(err instanceof ApiError ? err.message : "No se pudo cancelar la cita.");
      }
    } finally {
      setGuardandoCancelar(false);
    }
  }

  if (cargando) {
    return (
      <div className="flex h-64 items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
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
  const terminal = esTerminal(cita.estado);

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <header>
        <Link to="/paciente/citas" className="text-sm font-medium text-brand-600 hover:text-brand-700">
          ← Volver a mis citas
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
            <dt className="text-slate-500">Profesional</dt>
            <dd className="font-medium text-slate-900">{cita.medico_nombre}</dd>
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
            <dt className="text-slate-500">Duración</dt>
            <dd className="font-medium text-slate-900">{cita.duracion_min} min</dd>
          </div>
          <div>
            <dt className="text-slate-500">Modalidad</dt>
            <dd className="font-medium text-slate-900">{etiquetaTipo(cita.tipo)}</dd>
          </div>
          <div>
            <dt className="text-slate-500">Derivación</dt>
            <dd className="font-medium text-slate-900">
              {cita.derivacion_id ? "Con derivación" : "Sin derivación"}
            </dd>
          </div>
          <div className="sm:col-span-2">
            <dt className="text-slate-500">Motivo</dt>
            <dd className="font-medium text-slate-900">{cita.motivo || "—"}</dd>
          </div>
          {cita.motivo_cancelacion && (
            <div className="sm:col-span-2">
              <dt className="text-slate-500">Motivo de cancelación</dt>
              <dd className="font-medium text-slate-900">{cita.motivo_cancelacion}</dd>
            </div>
          )}
        </dl>

        {!terminal && (
          <div className="mt-6 flex flex-wrap gap-3">
            <button
              type="button"
              onClick={abrirReprogramar}
              className="rounded-lg bg-brand-600 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-brand-700"
            >
              Reprogramar
            </button>
            <button
              type="button"
              onClick={() => setMostrarCancelar(true)}
              className="rounded-lg border border-red-300 px-4 py-2 text-sm font-semibold text-red-600 hover:bg-red-50"
            >
              Cancelar cita
            </button>
          </div>
        )}
      </section>

      {/* Reprogramación */}
      {mostrarReprogramar && (
        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-slate-500">
            Elegir nuevo horario
          </h2>
          <p className="mb-4 text-sm text-slate-500">
            Solo se cambia el horario; el médico y la especialidad se conservan.
          </p>

          {disponibilidad ? (
            <SelectorSlot
              slots={disponibilidad.slots}
              onSelect={setSlotNuevo}
              selectedInicio={slotNuevo?.inicio ?? null}
            />
          ) : (
            <div className="flex justify-center py-6">
              <div className="h-6 w-6 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
            </div>
          )}

          <div className="mt-4">
            <label htmlFor="motivo-reprogramar" className="mb-1 block text-sm font-medium text-slate-700">
              Motivo del cambio (opcional)
            </label>
            <textarea
              id="motivo-reprogramar"
              value={motivoReprogramar}
              onChange={(e) => setMotivoReprogramar(e.target.value)}
              rows={2}
              className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none focus:border-brand-500 focus:ring-2 focus:ring-brand-100"
            />
          </div>

          <div className="mt-4 flex justify-end gap-3">
            <button
              type="button"
              onClick={() => setMostrarReprogramar(false)}
              className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
            >
              Cancelar
            </button>
            <button
              type="button"
              onClick={confirmarReprogramar}
              disabled={!slotNuevo || guardandoReprogramar}
              className="rounded-lg bg-brand-600 px-4 py-2 text-sm font-semibold text-white hover:bg-brand-700 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {guardandoReprogramar ? "Guardando…" : "Confirmar cambio"}
            </button>
          </div>
        </section>
      )}

      {/* Modal de cancelación */}
      {mostrarCancelar && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div
            className="absolute inset-0 bg-slate-900/40"
            onClick={() => setMostrarCancelar(false)}
          />
          <div className="relative w-full max-w-md rounded-2xl bg-white p-6 shadow-xl">
            <h2 className="text-lg font-bold text-slate-900">Cancelar cita</h2>

            {dentroDelCorte && (
              <p className="mt-3 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800">
                Cancelar con menos de 24 horas de anticipación consumirá tu
                derivación (si aplica). ¿Deseas continuar?
              </p>
            )}

            <div className="mt-4">
              <label htmlFor="motivo-cancelar" className="mb-1 block text-sm font-medium text-slate-700">
                Motivo de cancelación
              </label>
              <textarea
                id="motivo-cancelar"
                value={motivoCancelar}
                onChange={(e) => setMotivoCancelar(e.target.value)}
                rows={3}
                className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none focus:border-brand-500 focus:ring-2 focus:ring-brand-100"
                placeholder="Explica brevemente el motivo"
              />
            </div>

            <div className="mt-5 flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setMostrarCancelar(false)}
                className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
              >
                Volver
              </button>
              <button
                type="button"
                onClick={confirmarCancelar}
                disabled={guardandoCancelar}
                className="rounded-lg bg-red-600 px-4 py-2 text-sm font-semibold text-white hover:bg-red-700 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {guardandoCancelar ? "Cancelando…" : "Sí, cancelar"}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
