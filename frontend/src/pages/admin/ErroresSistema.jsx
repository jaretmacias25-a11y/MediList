import { useEffect, useState } from "react";
import { ApiError } from "@/lib/http";
import { listarErrores } from "@/services/admin";

const inputClass =
  "w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 shadow-sm outline-none transition focus:border-brand-500 focus:ring-2 focus:ring-brand-100";

const LIMITE = 50;

const NIVELES = ["", "error", "warning", "info"];

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

export default function ErroresSistema() {
  const [items, setItems] = useState([]);
  const [cargando, setCargando] = useState(true);
  const [error, setError] = useState(null);

  const [nivel, setNivel] = useState("");
  const [servicio, setServicio] = useState("");
  const [traceID, setTraceID] = useState("");

  useEffect(() => {
    let activo = true;
    setCargando(true);
    setError(null);
    listarErrores(nivel, servicio, traceID, LIMITE, 0)
      .then((resultado) => {
        if (activo) setItems(resultado.items);
      })
      .catch((err) => {
        if (activo) setError(err instanceof ApiError ? err.message : "No se pudieron cargar los errores.");
      })
      .finally(() => {
        if (activo) setCargando(false);
      });
    return () => {
      activo = false;
    };
  }, [nivel, servicio, traceID]);

  return (
    <div className="mx-auto max-w-6xl space-y-6">
      <header>
        <h1 className="text-xl font-bold text-slate-900">Errores del sistema</h1>
        <p className="mt-1 text-sm text-slate-500">
          Errores agregados con su <code className="rounded bg-slate-100 px-1">trace_id</code> para
          correlacionar con la auditoría y los logs.
        </p>
      </header>

      {error && (
        <p role="alert" className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
          {error}
        </p>
      )}

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <div>
          <label className="mb-1 block text-sm font-medium text-slate-700">Nivel</label>
          <select value={nivel} onChange={(e) => setNivel(e.target.value)} className={inputClass}>
            {NIVELES.map((n) => (
              <option key={n} value={n}>{n || "Todos"}</option>
            ))}
          </select>
        </div>
        <div>
          <label className="mb-1 block text-sm font-medium text-slate-700">Servicio</label>
          <input value={servicio} onChange={(e) => setServicio(e.target.value)} className={inputClass} placeholder="medilist-api" />
        </div>
        <div>
          <label className="mb-1 block text-sm font-medium text-slate-700">Trace ID</label>
          <input value={traceID} onChange={(e) => setTraceID(e.target.value)} className={inputClass} placeholder="trace_id" />
        </div>
      </div>

      {cargando ? (
        <div className="flex justify-center py-8">
          <div className="h-6 w-6 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
        </div>
      ) : items.length === 0 ? (
        <p className="rounded-lg border border-slate-200 bg-white px-4 py-3 text-sm text-slate-500">
          No hay errores para los filtros seleccionados.
        </p>
      ) : (
        <div className="overflow-x-auto rounded-2xl border border-slate-200 bg-white shadow-sm">
          <table className="w-full text-left text-sm">
            <thead className="bg-slate-50 text-xs uppercase text-slate-500">
              <tr>
                <th className="px-4 py-3">Fecha</th>
                <th className="px-4 py-3">Nivel</th>
                <th className="px-4 py-3">Servicio</th>
                <th className="px-4 py-3">Ruta</th>
                <th className="px-4 py-3">Código</th>
                <th className="px-4 py-3">Mensaje</th>
                <th className="px-4 py-3">Trace ID</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {items.map((item) => (
                <tr key={item.id} className="align-top">
                  <td className="whitespace-nowrap px-4 py-2 text-slate-600">{formatoFecha(item.created_at)}</td>
                  <td className="px-4 py-2">
                    <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${
                      item.nivel === "error"
                        ? "bg-red-100 text-red-800"
                        : item.nivel === "warning"
                          ? "bg-amber-100 text-amber-800"
                          : "bg-blue-100 text-blue-800"
                    }`}>
                      {item.nivel}
                    </span>
                  </td>
                  <td className="px-4 py-2 text-slate-700">{item.servicio}</td>
                  <td className="max-w-xs truncate px-4 py-2 text-slate-500" title={item.ruta}>
                    {item.metodo} {item.ruta}
                  </td>
                  <td className="px-4 py-2 text-slate-700">{item.codigo}</td>
                  <td className="max-w-md truncate px-4 py-2 text-slate-600" title={item.mensaje}>
                    {item.mensaje}
                  </td>
                  <td className="max-w-[180px] truncate px-4 py-2 font-mono text-xs text-slate-500" title={item.trace_id}>
                    {item.trace_id}
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
