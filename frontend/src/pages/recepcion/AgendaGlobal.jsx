import { useEffect, useState } from "react";
import { ApiError } from "@/lib/http";
import {
  actualizarBloqueo,
  actualizarHorario,
  crearBloqueo,
  crearHorario,
  listarBloqueos,
  listarHorarios,
} from "@/services/recepcion";
import { listarMedicosClinicos } from "@/services/medico";
import { useAccess } from "@/hooks/useAccess";

const inputClass =
  "w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 shadow-sm outline-none transition focus:border-brand-500 focus:ring-2 focus:ring-brand-100";

const DIAS = [
  { valor: 1, etiqueta: "Lunes" },
  { valor: 2, etiqueta: "Martes" },
  { valor: 3, etiqueta: "Miércoles" },
  { valor: 4, etiqueta: "Jueves" },
  { valor: 5, etiqueta: "Viernes" },
  { valor: 6, etiqueta: "Sábado" },
  { valor: 7, etiqueta: "Domingo" },
];

function aRFC3339(valorLocal) {
  const fecha = new Date(valorLocal);
  if (Number.isNaN(fecha.getTime())) return "";
  return fecha.toISOString();
}

function aValorLocal(iso) {
  const fecha = new Date(iso);
  const local = new Date(fecha.getTime() - fecha.getTimezoneOffset() * 60000);
  return local.toISOString().slice(0, 16);
}

function formatoFecha(iso) {
  return new Intl.DateTimeFormat("es-EC", {
    day: "numeric",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    timeZone: "America/Guayaquil",
  }).format(new Date(iso));
}

const HORARIO_INICIAL = {
  dia_semana: 1,
  hora_inicio: "09:00",
  hora_fin: "17:00",
  duracion_slot_min: 30,
  vigente_desde: "",
  vigente_hasta: "",
  activo: true,
};

export default function AgendaGlobal() {
  const { can } = useAccess();
  const puedeEscribir = can("agenda:gestionar");

  const [medicos, setMedicos] = useState([]);
  const [medicoID, setMedicoID] = useState("");
  const [horarios, setHorarios] = useState([]);
  const [bloqueos, setBloqueos] = useState([]);
  const [cargando, setCargando] = useState(true);
  const [error, setError] = useState(null);
  const [alerta, setAlerta] = useState(null);
  const [exito, setExito] = useState(null);

  const [mostrarHorario, setMostrarHorario] = useState(false);
  const [horarioEditando, setHorarioEditando] = useState(null); // null = nuevo
  const [horarioForm, setHorarioForm] = useState(HORARIO_INICIAL);

  const [mostrarBloqueo, setMostrarBloqueo] = useState(false);
  const [bloqueoEditando, setBloqueoEditando] = useState(null); // null = nuevo
  const [bloqueoForm, setBloqueoForm] = useState({ inicio: "", fin: "", motivo: "" });

  const [guardando, setGuardando] = useState(false);

  useEffect(() => {
    let activo = true;
    listarMedicosClinicos()
      .then((lista) => {
        if (activo) {
          const disponibles = lista.filter((m) => m.activo);
          setMedicos(disponibles);
          if (disponibles.length > 0) setMedicoID(String(disponibles[0].id));
        }
      })
      .catch((err) => {
        if (activo) setError(err instanceof ApiError ? err.message : "No se pudieron cargar los médicos.");
      })
      .finally(() => {
        if (activo) setCargando(false);
      });
    return () => {
      activo = false;
    };
  }, []);

  useEffect(() => {
    if (!medicoID) return;
    let activo = true;
    setCargando(true);
    setError(null);
    Promise.all([listarHorarios(Number(medicoID)), listarBloqueos(Number(medicoID))])
      .then(([horariosResp, bloqueosResp]) => {
        if (activo) {
          setHorarios(horariosResp);
          setBloqueos(bloqueosResp);
        }
      })
      .catch((err) => {
        if (activo) setError(err instanceof ApiError ? err.message : "No se pudo cargar la agenda.");
      })
      .finally(() => {
        if (activo) setCargando(false);
      });
    return () => {
      activo = false;
    };
  }, [medicoID]);

  async function guardarHorario() {
    if (!medicoID) return;
    setGuardando(true);
    setAlerta(null);
    try {
      const input = {
        dia_semana: Number(horarioForm.dia_semana),
        hora_inicio: horarioForm.hora_inicio,
        hora_fin: horarioForm.hora_fin,
        duracion_slot_min: Number(horarioForm.duracion_slot_min),
        vigente_desde: horarioForm.vigente_desde || undefined,
        vigente_hasta: horarioForm.vigente_hasta || null,
        activo: horarioForm.activo,
        version: horarioEditando?.version ?? 0,
      };
      if (horarioEditando) {
        await actualizarHorario(Number(medicoID), horarioEditando.id, input);
      } else {
        await crearHorario(Number(medicoID), input);
      }
      setMostrarHorario(false);
      setHorarioForm(HORARIO_INICIAL);
      setExito(horarioEditando ? "Horario actualizado." : "Horario creado.");
      setHorarios(await listarHorarios(Number(medicoID)));
    } catch (err) {
      if (err instanceof ApiError && err.isConflict) {
        setAlerta("La agenda cambió en otra sesión. Se recargaron los datos.");
        setMostrarHorario(false);
        setHorarios(await listarHorarios(Number(medicoID)));
      } else {
        setAlerta(err instanceof ApiError ? err.message : "No se pudo guardar el horario.");
      }
    } finally {
      setGuardando(false);
    }
  }

  async function guardarBloqueo() {
    if (!medicoID) return;
    setGuardando(true);
    setAlerta(null);
    try {
      const input = {
        inicio: aRFC3339(bloqueoForm.inicio),
        fin: aRFC3339(bloqueoForm.fin),
        motivo: bloqueoForm.motivo.trim(),
      };
      if (bloqueoEditando) {
        await actualizarBloqueo(Number(medicoID), bloqueoEditando.id, input);
      } else {
        await crearBloqueo(Number(medicoID), input);
      }
      setMostrarBloqueo(false);
      setBloqueoForm({ inicio: "", fin: "", motivo: "" });
      setExito(bloqueoEditando ? "Bloqueo actualizado." : "Bloqueo creado.");
      setBloqueos(await listarBloqueos(Number(medicoID)));
    } catch (err) {
      if (err instanceof ApiError && err.isConflict) {
        setAlerta("La agenda cambió en otra sesión. Se recargaron los datos.");
        setMostrarBloqueo(false);
        setBloqueos(await listarBloqueos(Number(medicoID)));
      } else {
        setAlerta(err instanceof ApiError ? err.message : "No se pudo guardar el bloqueo.");
      }
    } finally {
      setGuardando(false);
    }
  }

  if (cargando && medicos.length === 0) {
    return (
      <div className="flex h-64 items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
      </div>
    );
  }

  if (error && !medicoID) {
    return (
      <div className="rounded-xl border border-red-200 bg-red-50 p-6 text-sm text-red-700">
        {error}
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-5xl space-y-6">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-xl font-bold text-slate-900">Agenda médica</h1>
          <p className="mt-1 text-sm text-slate-500">
            Horarios recurrentes y bloqueos por profesional.
          </p>
        </div>
        <label className="text-sm">
          <span className="mb-1 block font-medium text-slate-700">Médico</span>
          <select value={medicoID} onChange={(e) => setMedicoID(e.target.value)} className={inputClass}>
            {medicos.map((medico) => (
              <option key={medico.id} value={String(medico.id)}>
                {medico.persona.nombres} {medico.persona.apellidos} — {medico.especialidad.nombre}
              </option>
            ))}
          </select>
        </label>
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
      {error && (
        <p role="alert" className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
          {error}
        </p>
      )}

      <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-sm font-semibold uppercase tracking-wide text-slate-500">Horarios</h2>
          {puedeEscribir && (
            <button
              type="button"
              onClick={() => {
                setHorarioEditando(null);
                setHorarioForm(HORARIO_INICIAL);
                setMostrarHorario(true);
              }}
              className="rounded-lg bg-brand-600 px-3 py-1.5 text-sm font-semibold text-white hover:bg-brand-700"
            >
              Nuevo horario
            </button>
          )}
        </div>

        {cargando ? (
          <div className="flex justify-center py-6">
            <div className="h-6 w-6 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
          </div>
        ) : horarios.length === 0 ? (
          <p className="text-sm text-slate-500">Sin horarios registrados.</p>
        ) : (
          <ul className="divide-y divide-slate-100">
            {horarios.map((horario) => (
              <li key={horario.id} className="flex flex-wrap items-center gap-3 py-3">
                <div className="min-w-0 flex-1">
                  <p className="font-medium text-slate-900">
                    {DIAS.find((d) => d.valor === horario.dia_semana)?.etiqueta ?? horario.dia_semana}{" "}
                    · {horario.hora_inicio}–{horario.hora_fin}
                  </p>
                  <p className="text-sm text-slate-500">
                    Slots de {horario.duracion_slot_min} min
                    {horario.vigente_hasta ? ` · hasta ${horario.vigente_hasta}` : " · vigencia indefinida"}
                  </p>
                </div>
                <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${
                  horario.activo ? "bg-emerald-100 text-emerald-800" : "bg-slate-200 text-slate-600"
                }`}>
                  {horario.activo ? "Activo" : "Inactivo"}
                </span>
                {puedeEscribir && (
                  <button
                    type="button"
                    onClick={() => {
                      setHorarioEditando(horario);
                      setHorarioForm({
                        dia_semana: horario.dia_semana,
                        hora_inicio: horario.hora_inicio,
                        hora_fin: horario.hora_fin,
                        duracion_slot_min: horario.duracion_slot_min,
                        vigente_desde: horario.vigente_desde,
                        vigente_hasta: horario.vigente_hasta ?? "",
                        activo: horario.activo,
                      });
                      setMostrarHorario(true);
                    }}
                    className="rounded-lg border border-slate-300 px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
                  >
                    Editar
                  </button>
                )}
              </li>
            ))}
          </ul>
        )}
      </section>

      <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-sm font-semibold uppercase tracking-wide text-slate-500">Bloqueos</h2>
          {puedeEscribir && (
            <button
              type="button"
              onClick={() => {
                setBloqueoEditando(null);
                setBloqueoForm({ inicio: "", fin: "", motivo: "" });
                setMostrarBloqueo(true);
              }}
              className="rounded-lg bg-brand-600 px-3 py-1.5 text-sm font-semibold text-white hover:bg-brand-700"
            >
              Nuevo bloqueo
            </button>
          )}
        </div>

        {cargando ? (
          <div className="flex justify-center py-6">
            <div className="h-6 w-6 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
          </div>
        ) : bloqueos.length === 0 ? (
          <p className="text-sm text-slate-500">Sin bloqueos registrados.</p>
        ) : (
          <ul className="divide-y divide-slate-100">
            {bloqueos.map((bloqueo) => (
              <li key={bloqueo.id} className="flex flex-wrap items-center gap-3 py-3">
                <div className="min-w-0 flex-1">
                  <p className="font-medium text-slate-900">
                    {formatoFecha(bloqueo.inicio)} → {formatoFecha(bloqueo.fin)}
                  </p>
                  <p className="text-sm text-slate-500">{bloqueo.motivo}</p>
                </div>
                {puedeEscribir && (
                  <button
                    type="button"
                    onClick={() => {
                      setBloqueoEditando(bloqueo);
                      setBloqueoForm({
                        inicio: aValorLocal(bloqueo.inicio),
                        fin: aValorLocal(bloqueo.fin),
                        motivo: bloqueo.motivo,
                      });
                      setMostrarBloqueo(true);
                    }}
                    className="rounded-lg border border-slate-300 px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
                  >
                    Editar
                  </button>
                )}
              </li>
            ))}
          </ul>
        )}
      </section>

      {mostrarHorario && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div className="absolute inset-0 bg-slate-900/40" onClick={() => setMostrarHorario(false)} />
          <div className="relative w-full max-w-md rounded-2xl bg-white p-6 shadow-xl">
            <h2 className="text-lg font-bold text-slate-900">
              {horarioEditando ? "Editar horario" : "Nuevo horario"}
            </h2>
            <div className="mt-4 grid grid-cols-1 gap-4">
              <div>
                <label className="mb-1 block text-sm font-medium text-slate-700">Día de la semana</label>
                <select
                  value={horarioForm.dia_semana}
                  onChange={(e) => setHorarioForm((f) => ({ ...f, dia_semana: Number(e.target.value) }))}
                  className={inputClass}
                >
                  {DIAS.map((dia) => (
                    <option key={dia.valor} value={dia.valor}>{dia.etiqueta}</option>
                  ))}
                </select>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="mb-1 block text-sm font-medium text-slate-700">Inicio</label>
                  <input type="time" value={horarioForm.hora_inicio}
                    onChange={(e) => setHorarioForm((f) => ({ ...f, hora_inicio: e.target.value }))} className={inputClass} />
                </div>
                <div>
                  <label className="mb-1 block text-sm font-medium text-slate-700">Fin</label>
                  <input type="time" value={horarioForm.hora_fin}
                    onChange={(e) => setHorarioForm((f) => ({ ...f, hora_fin: e.target.value }))} className={inputClass} />
                </div>
              </div>
              <div>
                <label className="mb-1 block text-sm font-medium text-slate-700">Duración del slot (min)</label>
                <input type="number" min={5} max={240} value={horarioForm.duracion_slot_min}
                  onChange={(e) => setHorarioForm((f) => ({ ...f, duracion_slot_min: Number(e.target.value) }))} className={inputClass} />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="mb-1 block text-sm font-medium text-slate-700">Vigente desde</label>
                  <input type="date" value={horarioForm.vigente_desde}
                    onChange={(e) => setHorarioForm((f) => ({ ...f, vigente_desde: e.target.value }))} className={inputClass} />
                </div>
                <div>
                  <label className="mb-1 block text-sm font-medium text-slate-700">Vigente hasta</label>
                  <input type="date" value={horarioForm.vigente_hasta}
                    onChange={(e) => setHorarioForm((f) => ({ ...f, vigente_hasta: e.target.value }))} className={inputClass} />
                </div>
              </div>
              <label className="flex items-center gap-2 text-sm text-slate-700">
                <input type="checkbox" checked={horarioForm.activo}
                  onChange={(e) => setHorarioForm((f) => ({ ...f, activo: e.target.checked }))}
                  className="h-4 w-4 rounded border-slate-300 text-brand-600 focus:ring-brand-500" />
                Horario activo
              </label>
            </div>
            <div className="mt-5 flex justify-end gap-3">
              <button type="button" onClick={() => setMostrarHorario(false)}
                className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50">
                Cancelar
              </button>
              <button type="button" disabled={guardando} onClick={guardarHorario}
                className="rounded-lg bg-brand-600 px-4 py-2 text-sm font-semibold text-white hover:bg-brand-700 disabled:opacity-60">
                {guardando ? "Guardando…" : "Guardar"}
              </button>
            </div>
          </div>
        </div>
      )}

      {mostrarBloqueo && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div className="absolute inset-0 bg-slate-900/40" onClick={() => setMostrarBloqueo(false)} />
          <div className="relative w-full max-w-md rounded-2xl bg-white p-6 shadow-xl">
            <h2 className="text-lg font-bold text-slate-900">
              {bloqueoEditando ? "Editar bloqueo" : "Nuevo bloqueo"}
            </h2>
            <div className="mt-4 grid grid-cols-1 gap-4">
              <div>
                <label className="mb-1 block text-sm font-medium text-slate-700">Inicio</label>
                <input type="datetime-local" value={bloqueoForm.inicio}
                  onChange={(e) => setBloqueoForm((f) => ({ ...f, inicio: e.target.value }))} className={inputClass} />
              </div>
              <div>
                <label className="mb-1 block text-sm font-medium text-slate-700">Fin</label>
                <input type="datetime-local" value={bloqueoForm.fin}
                  onChange={(e) => setBloqueoForm((f) => ({ ...f, fin: e.target.value }))} className={inputClass} />
              </div>
              <div>
                <label className="mb-1 block text-sm font-medium text-slate-700">Motivo</label>
                <textarea rows={2} value={bloqueoForm.motivo}
                  onChange={(e) => setBloqueoForm((f) => ({ ...f, motivo: e.target.value }))} className={inputClass} />
              </div>
            </div>
            <div className="mt-5 flex justify-end gap-3">
              <button type="button" onClick={() => setMostrarBloqueo(false)}
                className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50">
                Cancelar
              </button>
              <button type="button" disabled={guardando} onClick={guardarBloqueo}
                className="rounded-lg bg-brand-600 px-4 py-2 text-sm font-semibold text-white hover:bg-brand-700 disabled:opacity-60">
                {guardando ? "Guardando…" : "Guardar"}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
