import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { ApiError } from "@/lib/http";
import { listarCitas } from "@/services/citas";
import {
  esTerminal,
  etiquetaEstado,
  etiquetaTipo,
  formatoFechaGuayaquil,
} from "./citas/etiquetas";

function TarjetaCita({ cita }) {
  const estado = etiquetaEstado(cita.estado);
  const pasada = esTerminal(cita.estado);

  return (
    <Link
      to={`/paciente/citas/${cita.id}`}
      className={`rounded-2xl border bg-white p-4 shadow-sm transition hover:border-brand-400 ${
        pasada ? "border-slate-200" : "border-slate-200"
      }`}
    >
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-xs text-slate-500">Cita {cita.codigo}</p>
          <p className="mt-1 font-semibold text-slate-900">
            {cita.medico_nombre}
          </p>
          <p className="text-sm text-slate-500">{cita.especialidad_nombre}</p>
        </div>
        <span
          className={`shrink-0 rounded-full px-2.5 py-1 text-xs font-semibold ${estado.clase}`}
        >
          {estado.texto}
        </span>
      </div>

      <div className="mt-3 space-y-1 text-sm text-slate-600">
        <p>{formatoFechaGuayaquil(cita.fecha_hora_inicio)}</p>
        <p>
          {etiquetaTipo(cita.tipo)} · {cita.duracion_min} min
        </p>
      </div>

      {cita.derivacion_id && (
        <p className="mt-2 inline-block rounded-full bg-violet-100 px-2.5 py-0.5 text-xs font-medium text-violet-800">
          Con derivación
        </p>
      )}
    </Link>
  );
}

export default function MisCitas() {
  const [citas, setCitas] = useState([]);
  const [cargando, setCargando] = useState(true);
  const [error, setError] = useState(null);

  async function cargar() {
    setCargando(true);
    setError(null);
    try {
      setCitas(await listarCitas());
    } catch (err) {
      if (err instanceof ApiError && err.isFeatureDisabled) {
        setError("El módulo de citas no está disponible en este momento.");
      } else {
        setError(err instanceof ApiError ? err.message : "No se pudieron cargar las citas.");
      }
    } finally {
      setCargando(false);
    }
  }

  useEffect(() => {
    cargar();
  }, []);

  const { proximas, pasadas } = useMemo(() => {
    const ahora = Date.now();
    const proximas = [];
    const pasadas = [];
    for (const cita of citas) {
      const fecha = new Date(cita.fecha_hora_inicio).getTime();
      if (esTerminal(cita.estado) || fecha < ahora) {
        pasadas.push(cita);
      } else {
        proximas.push(cita);
      }
    }
    proximas.sort(
      (a, b) => new Date(a.fecha_hora_inicio) - new Date(b.fecha_hora_inicio),
    );
    pasadas.sort(
      (a, b) => new Date(b.fecha_hora_inicio) - new Date(a.fecha_hora_inicio),
    );
    return { proximas, pasadas };
  }, [citas]);

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
      <header className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-slate-900">Mis citas</h1>
          <p className="mt-1 text-sm text-slate-500">
            Consulta, reprograma o cancela tus citas.
          </p>
        </div>
        <Link
          to="/paciente/agendar"
          className="rounded-lg bg-brand-600 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-brand-700"
        >
          Agendar cita
        </Link>
      </header>

      <section>
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-slate-500">
          Próximas citas
        </h2>
        {proximas.length === 0 ? (
          <p className="rounded-lg border border-slate-200 bg-white px-4 py-3 text-sm text-slate-500">
            No tienes citas próximas.
          </p>
        ) : (
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            {proximas.map((cita) => (
              <TarjetaCita key={cita.id} cita={cita} />
            ))}
          </div>
        )}
      </section>

      <section>
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-slate-500">
          Citas pasadas
        </h2>
        {pasadas.length === 0 ? (
          <p className="rounded-lg border border-slate-200 bg-white px-4 py-3 text-sm text-slate-500">
            No tienes citas pasadas.
          </p>
        ) : (
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            {pasadas.map((cita) => (
              <TarjetaCita key={cita.id} cita={cita} />
            ))}
          </div>
        )}
      </section>
    </div>
  );
}
