import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { ApiError } from "@/lib/http";
import { listarEspecialidades } from "@/services/agenda";

export default function Especialidades() {
  const [especialidades, setEspecialidades] = useState([]);
  const [cargando, setCargando] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    let activo = true;
    listarEspecialidades()
      .then((lista) => {
        if (activo) setEspecialidades(lista);
      })
      .catch((err) => {
        if (activo) setError(err instanceof ApiError ? err.message : "No se pudieron cargar las especialidades.");
      })
      .finally(() => {
        if (activo) setCargando(false);
      });
    return () => {
      activo = false;
    };
  }, []);

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
    <div className="mx-auto max-w-3xl space-y-6">
      <header>
        <h1 className="text-xl font-bold text-slate-900">Especialidades</h1>
        <p className="mt-1 text-sm text-slate-500">
          Elige una especialidad para ver los profesionales disponibles.
        </p>
      </header>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        {especialidades.map((especialidad) => (
          <Link
            key={especialidad.id}
            to={`/paciente/medicos?especialidad_id=${especialidad.id}`}
            className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm transition hover:border-brand-400"
          >
            <div className="flex items-start justify-between gap-3">
              <div>
                <p className="font-semibold text-slate-900">{especialidad.nombre}</p>
                <p className="mt-1 text-sm text-slate-500">{especialidad.descripcion}</p>
                <p className="mt-1 text-xs text-slate-400">
                  {especialidad.duracion_cita_min} min por cita
                </p>
              </div>
              {especialidad.requiere_derivacion && (
                <span className="shrink-0 rounded-full bg-violet-100 px-2.5 py-1 text-xs font-medium text-violet-800">
                  Requiere derivación
                </span>
              )}
            </div>
          </Link>
        ))}
      </div>
    </div>
  );
}
