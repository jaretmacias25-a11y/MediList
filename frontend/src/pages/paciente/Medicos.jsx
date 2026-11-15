import { useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { ApiError } from "@/lib/http";
import { listarEspecialidades, listarMedicos } from "@/services/agenda";

export default function Medicos() {
  const [searchParams, setSearchParams] = useSearchParams();
  const especialidadID = searchParams.get("especialidad_id") ?? "";

  const [especialidades, setEspecialidades] = useState([]);
  const [medicos, setMedicos] = useState([]);
  const [cargando, setCargando] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    let activo = true;
    listarEspecialidades()
      .then((lista) => {
        if (activo) setEspecialidades(lista);
      })
      .catch(() => {});
    return () => {
      activo = false;
    };
  }, []);

  useEffect(() => {
    if (!especialidadID) {
      setMedicos([]);
      setCargando(false);
      return;
    }
    let activo = true;
    setCargando(true);
    setError(null);
    listarMedicos(Number(especialidadID))
      .then((lista) => {
        if (activo) setMedicos(lista);
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
  }, [especialidadID]);

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-xl font-bold text-slate-900">Médicos</h1>
          <p className="mt-1 text-sm text-slate-500">
            Profesionales disponibles para agendar.
          </p>
        </div>
        <label className="text-sm">
          <span className="mb-1 block font-medium text-slate-700">Especialidad</span>
          <select
            value={especialidadID}
            onChange={(e) =>
              setSearchParams(e.target.value ? { especialidad_id: e.target.value } : {})
            }
            className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm outline-none focus:border-brand-500 focus:ring-2 focus:ring-brand-100"
          >
            <option value="">Todas</option>
            {especialidades.map((especialidad) => (
              <option key={especialidad.id} value={String(especialidad.id)}>
                {especialidad.nombre}
              </option>
            ))}
          </select>
        </label>
      </header>

      {error && (
        <p role="alert" className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
          {error}
        </p>
      )}

      {cargando ? (
        <div className="flex justify-center py-8">
          <div className="h-6 w-6 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
        </div>
      ) : medicos.length === 0 ? (
        <p className="rounded-lg border border-slate-200 bg-white px-4 py-3 text-sm text-slate-500">
          No hay profesionales para la selección actual.
        </p>
      ) : (
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          {medicos.map((medico) => (
            <div key={medico.id} className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm">
              <p className="font-semibold text-slate-900">
                {medico.nombres} {medico.apellidos}
              </p>
              <p className="text-sm text-slate-500">{medico.especialidad.nombre}</p>
              {medico.bio && <p className="mt-2 text-sm text-slate-600">{medico.bio}</p>}
              <p className="mt-2 text-xs text-slate-400">
                {medico.duracion_cita_min} min por cita
              </p>
              <Link
                to="/paciente/agendar"
                className="mt-4 inline-block rounded-lg bg-brand-600 px-4 py-2 text-sm font-semibold text-white hover:bg-brand-700"
              >
                Agendar cita
              </Link>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
