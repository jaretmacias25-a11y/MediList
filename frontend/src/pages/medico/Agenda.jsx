import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { ApiError } from "@/lib/http";
import { listarAgenda } from "@/services/medico";
import { useAuthStore } from "@/stores/auth";
import {
  etiquetaEstado,
  etiquetaTipo,
  formatoFechaGuayaquil,
} from "@/pages/paciente/citas/etiquetas";

function fechaGuayaquilISO(fechaISO) {
  const fecha = new Date(fechaISO);
  const partes = new Intl.DateTimeFormat("en-CA", {
    timeZone: "America/Guayaquil",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts(fecha);
  const valores = {};
  for (const parte of partes) valores[parte.type] = parte.value;
  return `${valores.year}-${valores.month}-${valores.day}`;
}

export default function Agenda() {
  const user = useAuthStore((state) => state.user);
  const [citas, setCitas] = useState([]);
  const [filtroFecha, setFiltroFecha] = useState("");
  const [cargando, setCargando] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    let activo = true;
    listarAgenda()
      .then((lista) => {
        if (activo) setCitas(lista);
      })
      .catch((err) => {
        if (activo) {
          setError(err instanceof ApiError ? err.message : "No se pudo cargar la agenda.");
        }
      })
      .finally(() => {
        if (activo) setCargando(false);
      });
    return () => {
      activo = false;
    };
  }, []);

  const citasFiltradas = useMemo(() => {
    const ordenadas = [...citas].sort(
      (a, b) => new Date(a.fecha_hora_inicio) - new Date(b.fecha_hora_inicio),
    );
    if (!filtroFecha) return ordenadas;
    return ordenadas.filter(
      (cita) => fechaGuayaquilISO(cita.fecha_hora_inicio) === filtroFecha,
    );
  }, [citas, filtroFecha]);

  if (!user?.medico_id) {
    return (
      <div className="rounded-xl border border-red-200 bg-red-50 p-6 text-sm text-red-700">
        Acceso denegado: tu cuenta no está vinculada a un perfil médico.
      </div>
    );
  }

  if (cargando) {
    return (
      <div className="flex h-64 items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-xl border border-red-200 bg-red-50 p-6 text-sm text-red-700">
        {error}
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-4xl space-y-6">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-xl font-bold text-slate-900">Mi agenda</h1>
          <p className="mt-1 text-sm text-slate-500">
            Citas asignadas a tu perfil profesional.
          </p>
        </div>
        <label className="text-sm">
          <span className="mb-1 block font-medium text-slate-700">Filtrar por fecha</span>
          <input
            type="date"
            value={filtroFecha}
            onChange={(e) => setFiltroFecha(e.target.value)}
            className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm outline-none focus:border-brand-500 focus:ring-2 focus:ring-brand-100"
          />
        </label>
      </header>

      {citasFiltradas.length === 0 ? (
        <p className="rounded-lg border border-slate-200 bg-white px-4 py-3 text-sm text-slate-500">
          No hay citas {filtroFecha ? "para la fecha seleccionada" : "en tu agenda"}.
        </p>
      ) : (
        <div className="grid grid-cols-1 gap-3">
          {citasFiltradas.map((cita) => {
            const estado = etiquetaEstado(cita.estado);
            return (
              <Link
                key={cita.id}
                to={`/medico/citas/${cita.id}`}
                className="flex flex-wrap items-center gap-4 rounded-2xl border border-slate-200 bg-white p-4 shadow-sm transition hover:border-brand-400"
              >
                <div className="min-w-0 flex-1">
                  <p className="text-xs text-slate-500">Cita {cita.codigo}</p>
                  <p className="mt-1 font-semibold text-slate-900">{cita.paciente_nombre}</p>
                  <p className="text-sm text-slate-500">
                    {cita.especialidad_nombre} · {etiquetaTipo(cita.tipo)}
                  </p>
                  <p className="mt-1 text-sm text-slate-600">
                    {formatoFechaGuayaquil(cita.fecha_hora_inicio)}
                  </p>
                </div>
                <span className={`shrink-0 rounded-full px-2.5 py-1 text-xs font-semibold ${estado.clase}`}>
                  {estado.texto}
                </span>
              </Link>
            );
          })}
        </div>
      )}
    </div>
  );
}
