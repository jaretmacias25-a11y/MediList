import { useEffect, useState } from "react";
import { ApiError } from "@/lib/http";
import {
  aprobarSolicitud,
  listarSolicitudes,
  rechazarSolicitud,
} from "@/services/recepcion";
import { useAccess } from "@/hooks/useAccess";

const ETIQUETAS_ESTADO = {
  pendiente: { texto: "Pendiente", clase: "bg-blue-100 text-blue-800" },
  aprobada: { texto: "Aprobada", clase: "bg-emerald-100 text-emerald-800" },
  rechazada: { texto: "Rechazada", clase: "bg-red-100 text-red-800" },
};

function etiquetaEstado(estado) {
  return ETIQUETAS_ESTADO[estado] ?? {
    texto: estado,
    clase: "bg-slate-100 text-slate-700",
  };
}

function formatoFecha(iso) {
  if (!iso) return "—";
  return new Intl.DateTimeFormat("es-EC", {
    day: "numeric",
    month: "short",
    year: "numeric",
    timeZone: "America/Guayaquil",
  }).format(new Date(iso));
}

export default function Vinculaciones() {
  const { can } = useAccess();
  const puedeGestionar = can("cuentas:vincular");

  const [solicitudes, setSolicitudes] = useState([]);
  const [cargando, setCargando] = useState(true);
  const [error, setError] = useState(null);
  const [exito, setExito] = useState(null);
  const [procesando, setProcesando] = useState(null);

  const [rechazando, setRechazando] = useState(null);
  const [motivoRechazo, setMotivoRechazo] = useState("");

  async function cargar() {
    setCargando(true);
    setError(null);
    try {
      setSolicitudes(await listarSolicitudes());
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "No se pudieron cargar las solicitudes.");
    } finally {
      setCargando(false);
    }
  }

  useEffect(() => {
    cargar();
  }, []);

  async function aprobar(id) {
    setProcesando(id);
    setExito(null);
    setError(null);
    try {
      await aprobarSolicitud(id);
      setExito("Solicitud aprobada. La cuenta quedó vinculada.");
      await cargar();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "No se pudo aprobar la solicitud.");
    } finally {
      setProcesando(null);
    }
  }

  async function confirmarRechazo() {
    if (!rechazando) return;
    setProcesando(rechazando.id);
    setExito(null);
    setError(null);
    try {
      await rechazarSolicitud(rechazando.id, motivoRechazo.trim());
      setExito("Solicitud rechazada.");
      setRechazando(null);
      setMotivoRechazo("");
      await cargar();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "No se pudo rechazar la solicitud.");
    } finally {
      setProcesando(null);
    }
  }

  const pendientes = solicitudes.filter((s) => s.estado === "pendiente");
  const resueltas = solicitudes.filter((s) => s.estado !== "pendiente");

  if (cargando) {
    return (
      <div className="flex h-64 items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
      </div>
    );
  }

  if (error && solicitudes.length === 0) {
    return (
      <div className="rounded-xl border border-red-200 bg-red-50 p-6 text-sm text-red-700">
        {error}
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-4xl space-y-6">
      <header>
        <h1 className="text-xl font-bold text-slate-900">Vinculaciones de cuentas</h1>
        <p className="mt-1 text-sm text-slate-500">
          Solicitudes generadas cuando un registro coincide con una historia
          clínica existente.
        </p>
      </header>

      {exito && (
        <p role="status" className="rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-800">
          {exito}
        </p>
      )}
      {error && (
        <p role="alert" className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
          {error}
        </p>
      )}

      <section>
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-slate-500">
          Pendientes ({pendientes.length})
        </h2>
        {pendientes.length === 0 ? (
          <p className="rounded-lg border border-slate-200 bg-white px-4 py-3 text-sm text-slate-500">
            No hay solicitudes pendientes.
          </p>
        ) : (
          <div className="grid grid-cols-1 gap-3">
            {pendientes.map((solicitud) => (
              <div key={solicitud.id} className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div className="min-w-0">
                    <p className="font-semibold text-slate-900">
                      {solicitud.nombres} {solicitud.apellidos}
                    </p>
                    <p className="text-sm text-slate-500">
                      {solicitud.tipo_documento} {solicitud.numero_documento} · Historia {solicitud.numero_historia}
                    </p>
                    <p className="text-sm text-slate-500">
                      Cuenta: {solicitud.username} · {solicitud.email}
                    </p>
                    <p className="mt-1 text-xs text-slate-400">
                      Solicitada el {formatoFecha(solicitud.created_at)}
                    </p>
                  </div>
                  <div className="flex shrink-0 gap-2">
                    {puedeGestionar ? (
                      <>
                        <button
                          type="button"
                          disabled={procesando === solicitud.id}
                          onClick={() => aprobar(solicitud.id)}
                          className="rounded-lg bg-emerald-600 px-3 py-1.5 text-sm font-semibold text-white hover:bg-emerald-700 disabled:opacity-60"
                        >
                          Aprobar
                        </button>
                        <button
                          type="button"
                          disabled={procesando === solicitud.id}
                          onClick={() => {
                            setRechazando(solicitud);
                            setMotivoRechazo("");
                          }}
                          className="rounded-lg border border-red-300 px-3 py-1.5 text-sm font-semibold text-red-600 hover:bg-red-50 disabled:opacity-60"
                        >
                          Rechazar
                        </button>
                      </>
                    ) : (
                      <span className="text-sm text-slate-500">Solo lectura</span>
                    )}
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </section>

      {resueltas.length > 0 && (
        <section>
          <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-slate-500">
            Resueltas ({resueltas.length})
          </h2>
          <div className="grid grid-cols-1 gap-3">
            {resueltas.map((solicitud) => {
              const estado = etiquetaEstado(solicitud.estado);
              return (
                <div key={solicitud.id} className="flex flex-wrap items-center gap-3 rounded-2xl border border-slate-200 bg-white p-4 shadow-sm">
                  <div className="min-w-0 flex-1">
                    <p className="font-medium text-slate-900">
                      {solicitud.nombres} {solicitud.apellidos}
                    </p>
                    <p className="text-sm text-slate-500">
                      {solicitud.numero_historia} · {solicitud.email}
                    </p>
                    {solicitud.motivo && (
                      <p className="mt-1 text-xs text-slate-400">Motivo: {solicitud.motivo}</p>
                    )}
                  </div>
                  <span className={`shrink-0 rounded-full px-2.5 py-1 text-xs font-semibold ${estado.clase}`}>
                    {estado.texto}
                  </span>
                </div>
              );
            })}
          </div>
        </section>
      )}

      {rechazando && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div className="absolute inset-0 bg-slate-900/40" onClick={() => setRechazando(null)} />
          <div className="relative w-full max-w-sm rounded-2xl bg-white p-6 shadow-xl">
            <h2 className="text-lg font-bold text-slate-900">Rechazar solicitud</h2>
            <div className="mt-4">
              <label htmlFor="motivo-rechazo" className="mb-1 block text-sm font-medium text-slate-700">
                Motivo
              </label>
              <textarea
                id="motivo-rechazo"
                rows={3}
                value={motivoRechazo}
                onChange={(e) => setMotivoRechazo(e.target.value)}
                className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none focus:border-brand-500 focus:ring-2 focus:ring-brand-100"
              />
            </div>
            <div className="mt-5 flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setRechazando(null)}
                className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
              >
                Cancelar
              </button>
              <button
                type="button"
                disabled={procesando !== null || !motivoRechazo.trim()}
                onClick={confirmarRechazo}
                className="rounded-lg bg-red-600 px-4 py-2 text-sm font-semibold text-white hover:bg-red-700 disabled:opacity-60"
              >
                Rechazar
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
