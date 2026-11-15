import { useEffect, useState } from "react";
import { ApiError } from "@/lib/http";
import { actualizarCita, eliminarCita, listarCitasGlobales } from "@/services/recepcion";
import { useAccess } from "@/hooks/useAccess";
import {
  esTerminal,
  etiquetaEstado,
  etiquetaTipo,
  formatoFechaGuayaquil,
} from "@/pages/paciente/citas/etiquetas";

const LIMITE = 10;

function aRFC3339(valorLocal) {
  // datetime-local llega sin zona; se interpreta como hora local y se
  // convierte a UTC/RFC3339 para el backend.
  const fecha = new Date(valorLocal);
  if (Number.isNaN(fecha.getTime())) return "";
  return fecha.toISOString();
}

function aValorLocal(iso) {
  const fecha = new Date(iso);
  const local = new Date(fecha.getTime() - fecha.getTimezoneOffset() * 60000);
  return local.toISOString().slice(0, 16);
}

export default function CitasGlobales() {
  const { can } = useAccess();
  const puedeEscribir = can("citas:escribir");

  const [items, setItems] = useState([]);
  const [meta, setMeta] = useState({ count: 0, limit: LIMITE, offset: 0 });
  const [search, setSearch] = useState("");
  const [offset, setOffset] = useState(0);
  const [cargando, setCargando] = useState(true);
  const [error, setError] = useState(null);
  const [alerta, setAlerta] = useState(null);
  const [exito, setExito] = useState(null);

  const [reprogramando, setReprogramando] = useState(null); // cita en edición
  const [nuevaFecha, setNuevaFecha] = useState("");
  const [cancelando, setCancelando] = useState(null);
  const [guardando, setGuardando] = useState(false);

  async function cargar(busqueda, nuevoOffset) {
    setCargando(true);
    setError(null);
    try {
      const resultado = await listarCitasGlobales(busqueda, LIMITE, nuevoOffset);
      setItems(resultado.items);
      setMeta(resultado.meta);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "No se pudieron cargar las citas.");
    } finally {
      setCargando(false);
    }
  }

  useEffect(() => {
    const timer = setTimeout(() => {
      setOffset(0);
      cargar(search, 0);
    }, 300);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [search]);

  useEffect(() => {
    cargar(search, offset);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [offset]);

  async function confirmarReprogramar() {
    if (!reprogramando || !nuevaFecha) return;
    setGuardando(true);
    setAlerta(null);
    try {
      const actualizada = await actualizarCita(reprogramando.id, {
        paciente_id: reprogramando.paciente_id,
        medico_id: reprogramando.medico_id,
        fecha_hora_inicio: aRFC3339(nuevaFecha),
        duracion_min: reprogramando.duracion_min,
        tipo: reprogramando.tipo,
        motivo: reprogramando.motivo,
        estado: reprogramando.estado,
        version: reprogramando.version,
      });
      setItems((previas) => previas.map((c) => (c.id === actualizada.id ? actualizada : c)));
      setReprogramando(null);
      setExito("Cita reprogramada.");
    } catch (err) {
      if (err instanceof ApiError && err.isConflict) {
        setAlerta("La cita cambió en otra sesión. Se recargaron los datos.");
        setReprogramando(null);
        await cargar(search, offset);
      } else {
        setAlerta(err instanceof ApiError ? err.message : "No se pudo reprogramar la cita.");
      }
    } finally {
      setGuardando(false);
    }
  }

  async function confirmarCancelar() {
    if (!cancelando) return;
    setGuardando(true);
    setAlerta(null);
    try {
      await eliminarCita(cancelando.id);
      setCancelando(null);
      setExito("Cita cancelada.");
      await cargar(search, offset);
    } catch (err) {
      if (err instanceof ApiError && err.isConflict) {
        setAlerta("La cita cambió en otra sesión. Se recargaron los datos.");
        setCancelando(null);
        await cargar(search, offset);
      } else {
        setAlerta(err instanceof ApiError ? err.message : "No se pudo cancelar la cita.");
      }
    } finally {
      setGuardando(false);
    }
  }

  const total = meta?.count ?? items.length;
  const paginas = Math.max(1, Math.ceil(total / LIMITE));

  return (
    <div className="mx-auto max-w-5xl space-y-6">
      <header>
        <h1 className="text-xl font-bold text-slate-900">Citas globales</h1>
        <p className="mt-1 text-sm text-slate-500">
          Citas de todos los médicos de la clínica.
        </p>
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

      <div>
        <label htmlFor="busqueda" className="mb-1 block text-sm font-medium text-slate-700">
          Buscar
        </label>
        <input
          id="busqueda"
          type="search"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Código, paciente, médico o especialidad"
          className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm outline-none focus:border-brand-500 focus:ring-2 focus:ring-brand-100"
        />
      </div>

      {cargando ? (
        <div className="flex justify-center py-8">
          <div className="h-6 w-6 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
        </div>
      ) : items.length === 0 ? (
        <p className="rounded-lg border border-slate-200 bg-white px-4 py-3 text-sm text-slate-500">
          No se encontraron citas.
        </p>
      ) : (
        <div className="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
          <ul className="divide-y divide-slate-100">
            {items.map((cita) => {
              const estado = etiquetaEstado(cita.estado);
              return (
                <li key={cita.id} className="flex flex-wrap items-center gap-4 px-4 py-3">
                  <div className="min-w-0 flex-1">
                    <p className="text-xs text-slate-500">Cita {cita.codigo}</p>
                    <p className="font-medium text-slate-900">{cita.paciente_nombre}</p>
                    <p className="text-sm text-slate-500">
                      {cita.medico_nombre} · {cita.especialidad_nombre}
                    </p>
                    <p className="text-sm text-slate-600">
                      {formatoFechaGuayaquil(cita.fecha_hora_inicio)} · {etiquetaTipo(cita.tipo)}
                    </p>
                  </div>
                  <span className={`shrink-0 rounded-full px-2.5 py-1 text-xs font-semibold ${estado.clase}`}>
                    {estado.texto}
                  </span>
                  {puedeEscribir && !esTerminal(cita.estado) && (
                    <div className="flex shrink-0 gap-2">
                      <button
                        type="button"
                        onClick={() => {
                          setReprogramando(cita);
                          setNuevaFecha(aValorLocal(cita.fecha_hora_inicio));
                        }}
                        className="rounded-lg border border-slate-300 px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
                      >
                        Reprogramar
                      </button>
                      <button
                        type="button"
                        onClick={() => setCancelando(cita)}
                        className="rounded-lg border border-red-300 px-3 py-1.5 text-sm font-medium text-red-600 hover:bg-red-50"
                      >
                        Cancelar
                      </button>
                    </div>
                  )}
                </li>
              );
            })}
          </ul>
        </div>
      )}

      <div className="flex items-center justify-between text-sm text-slate-600">
        <button
          type="button"
          disabled={offset === 0 || cargando}
          onClick={() => setOffset(Math.max(0, offset - LIMITE))}
          className="rounded-lg border border-slate-300 px-3 py-1.5 disabled:opacity-50"
        >
          Anterior
        </button>
        <span>
          Página {Math.floor(offset / LIMITE) + 1} de {paginas} · {total} registros
        </span>
        <button
          type="button"
          disabled={offset + LIMITE >= total || cargando}
          onClick={() => setOffset(offset + LIMITE)}
          className="rounded-lg border border-slate-300 px-3 py-1.5 disabled:opacity-50"
        >
          Siguiente
        </button>
      </div>

      {reprogramando && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div className="absolute inset-0 bg-slate-900/40" onClick={() => setReprogramando(null)} />
          <div className="relative w-full max-w-sm rounded-2xl bg-white p-6 shadow-xl">
            <h2 className="text-lg font-bold text-slate-900">Reprogramar cita</h2>
            <p className="mt-1 text-sm text-slate-500">
              Solo cambia la fecha y hora; médico y especialidad se conservan.
            </p>
            <div className="mt-4">
              <label htmlFor="nueva-fecha" className="mb-1 block text-sm font-medium text-slate-700">
                Nueva fecha y hora
              </label>
              <input
                id="nueva-fecha"
                type="datetime-local"
                value={nuevaFecha}
                onChange={(e) => setNuevaFecha(e.target.value)}
                className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none focus:border-brand-500 focus:ring-2 focus:ring-brand-100"
              />
            </div>
            <div className="mt-5 flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setReprogramando(null)}
                className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
              >
                Cancelar
              </button>
              <button
                type="button"
                disabled={guardando || !nuevaFecha}
                onClick={confirmarReprogramar}
                className="rounded-lg bg-brand-600 px-4 py-2 text-sm font-semibold text-white hover:bg-brand-700 disabled:opacity-60"
              >
                {guardando ? "Guardando…" : "Confirmar"}
              </button>
            </div>
          </div>
        </div>
      )}

      {cancelando && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div className="absolute inset-0 bg-slate-900/40" onClick={() => setCancelando(null)} />
          <div className="relative w-full max-w-sm rounded-2xl bg-white p-6 shadow-xl">
            <h2 className="text-lg font-bold text-slate-900">Cancelar cita</h2>
            <p className="mt-2 text-sm text-slate-600">
              La cita {cancelando.codigo} de {cancelando.paciente_nombre} se
              cancelará. ¿Deseas continuar?
            </p>
            <div className="mt-5 flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setCancelando(null)}
                className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
              >
                Volver
              </button>
              <button
                type="button"
                disabled={guardando}
                onClick={confirmarCancelar}
                className="rounded-lg bg-red-600 px-4 py-2 text-sm font-semibold text-white hover:bg-red-700 disabled:opacity-60"
              >
                {guardando ? "Cancelando…" : "Sí, cancelar"}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
