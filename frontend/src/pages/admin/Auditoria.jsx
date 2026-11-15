import { useEffect, useMemo, useState } from "react";
import { ApiError } from "@/lib/http";
import { listarAuditoria } from "@/services/admin";

const inputClass =
  "w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 shadow-sm outline-none transition focus:border-brand-500 focus:ring-2 focus:ring-brand-100";

const LIMITE = 50;

function formatoFecha(iso) {
  return new Intl.DateTimeFormat("es-EC", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    timeZone: "America/Guayaquil",
  }).format(new Date(iso));
}

export default function Auditoria() {
  const [items, setItems] = useState([]);
  const [cargando, setCargando] = useState(true);
  const [error, setError] = useState(null);

  const [search, setSearch] = useState("");
  const [accion, setAccion] = useState("");
  const [traceID, setTraceID] = useState("");
  const [desde, setDesde] = useState("");
  const [hasta, setHasta] = useState("");

  useEffect(() => {
    let activo = true;
    setCargando(true);
    setError(null);
    // El backend filtra por search (usuario/acción/recurso/ruta/trace),
    // acción exacta y trace_id. El rango de fechas y el filtro por recurso
    // se aplican en el cliente sobre los resultados.
    listarAuditoria(search, accion, traceID, LIMITE, 0)
      .then((resultado) => {
        if (activo) setItems(resultado.items);
      })
      .catch((err) => {
        if (activo) setError(err instanceof ApiError ? err.message : "No se pudo cargar la auditoría.");
      })
      .finally(() => {
        if (activo) setCargando(false);
      });
    return () => {
      activo = false;
    };
  }, [search, accion, traceID]);

  const filtrados = useMemo(() => {
    return items.filter((evento) => {
      const fecha = new Date(evento.created_at).getTime();
      if (desde && fecha < new Date(desde).getTime()) return false;
      if (hasta && fecha > new Date(hasta).getTime() + 24 * 60 * 60 * 1000 - 1) return false;
      return true;
    });
  }, [items, desde, hasta]);

  return (
    <div className="mx-auto max-w-6xl space-y-6">
      <header>
        <h1 className="text-xl font-bold text-slate-900">Auditoría</h1>
        <p className="mt-1 text-sm text-slate-500">
          Eventos de seguridad del sistema. Solo lectura.
        </p>
      </header>

      {error && (
        <p role="alert" className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
          {error}
        </p>
      )}

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-5">
        <div>
          <label className="mb-1 block text-sm font-medium text-slate-700">Usuario / recurso / ruta</label>
          <input type="search" value={search} onChange={(e) => setSearch(e.target.value)} className={inputClass} placeholder="Buscar…" />
        </div>
        <div>
          <label className="mb-1 block text-sm font-medium text-slate-700">Acción</label>
          <input value={accion} onChange={(e) => setAccion(e.target.value)} className={inputClass} placeholder="login, agendar_cita…" />
        </div>
        <div>
          <label className="mb-1 block text-sm font-medium text-slate-700">Trace ID</label>
          <input value={traceID} onChange={(e) => setTraceID(e.target.value)} className={inputClass} placeholder="trace_id" />
        </div>
        <div>
          <label className="mb-1 block text-sm font-medium text-slate-700">Desde</label>
          <input type="date" value={desde} onChange={(e) => setDesde(e.target.value)} className={inputClass} />
        </div>
        <div>
          <label className="mb-1 block text-sm font-medium text-slate-700">Hasta</label>
          <input type="date" value={hasta} onChange={(e) => setHasta(e.target.value)} className={inputClass} />
        </div>
      </div>

      {cargando ? (
        <div className="flex justify-center py-8">
          <div className="h-6 w-6 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
        </div>
      ) : filtrados.length === 0 ? (
        <p className="rounded-lg border border-slate-200 bg-white px-4 py-3 text-sm text-slate-500">
          No hay eventos para los filtros seleccionados.
        </p>
      ) : (
        <div className="overflow-x-auto rounded-2xl border border-slate-200 bg-white shadow-sm">
          <table className="w-full text-left text-sm">
            <thead className="bg-slate-50 text-xs uppercase text-slate-500">
              <tr>
                <th className="px-4 py-3">Fecha</th>
                <th className="px-4 py-3">Usuario</th>
                <th className="px-4 py-3">Acción</th>
                <th className="px-4 py-3">Recurso</th>
                <th className="px-4 py-3">Método</th>
                <th className="px-4 py-3">Ruta</th>
                <th className="px-4 py-3">HTTP</th>
                <th className="px-4 py-3">Trace ID</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {filtrados.map((evento) => (
                <tr key={evento.id} className="align-top">
                  <td className="whitespace-nowrap px-4 py-2 text-slate-600">{formatoFecha(evento.created_at)}</td>
                  <td className="px-4 py-2 text-slate-700">{evento.username || "—"}</td>
                  <td className="px-4 py-2 text-slate-700">{evento.accion}</td>
                  <td className="px-4 py-2 text-slate-700">{evento.recurso}</td>
                  <td className="px-4 py-2 text-slate-600">{evento.metodo}</td>
                  <td className="max-w-xs truncate px-4 py-2 text-slate-500" title={evento.ruta}>
                    {evento.ruta}
                  </td>
                  <td className="px-4 py-2">
                    <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${
                      evento.estado_http >= 400 ? "bg-red-100 text-red-800" : "bg-emerald-100 text-emerald-800"
                    }`}>
                      {evento.estado_http}
                    </span>
                  </td>
                  <td className="max-w-[180px] truncate px-4 py-2 font-mono text-xs text-slate-500" title={evento.trace_id}>
                    {evento.trace_id || "—"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
