import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { listarCitas } from "@/services/citas";
import { useAuthStore } from "@/stores/auth";
import { esTerminal, etiquetaEstado, formatoFechaGuayaquil } from "./citas/etiquetas";

export default function InicioPaciente() {
  const user = useAuthStore((state) => state.user);
  const [proximas, setProximas] = useState([]);
  const [cargando, setCargando] = useState(true);

  useEffect(() => {
    let activo = true;
    listarCitas()
      .then((citas) => {
        if (!activo) return;
        const ahora = Date.now();
        const lista = citas
          .filter((c) => !esTerminal(c.estado) && new Date(c.fecha_hora_inicio).getTime() >= ahora)
          .sort((a, b) => new Date(a.fecha_hora_inicio) - new Date(b.fecha_hora_inicio))
          .slice(0, 3);
        setProximas(lista);
      })
      .catch(() => {
        if (activo) setProximas([]);
      })
      .finally(() => {
        if (activo) setCargando(false);
      });
    return () => {
      activo = false;
    };
  }, []);

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <header>
        <h1 className="text-xl font-bold text-slate-900">
          Hola, {user?.nombres ?? "paciente"}
        </h1>
        <p className="mt-1 text-sm text-slate-500">
          Gestiona tus citas y tu información desde aquí.
        </p>
      </header>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <Link
          to="/paciente/agendar"
          className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm transition hover:border-brand-400"
        >
          <p className="font-semibold text-slate-900">Agendar cita</p>
          <p className="mt-1 text-sm text-slate-500">Reserva una cita directa o con derivación.</p>
        </Link>
        <Link
          to="/paciente/citas"
          className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm transition hover:border-brand-400"
        >
          <p className="font-semibold text-slate-900">Mis citas</p>
          <p className="mt-1 text-sm text-slate-500">Consulta, reprograma o cancela tus citas.</p>
        </Link>
        <Link
          to="/paciente/perfil"
          className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm transition hover:border-brand-400"
        >
          <p className="font-semibold text-slate-900">Mi perfil</p>
          <p className="mt-1 text-sm text-slate-500">Actualiza tu información de contacto.</p>
        </Link>
        <Link
          to="/paciente/especialidades"
          className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm transition hover:border-brand-400"
        >
          <p className="font-semibold text-slate-900">Especialidades</p>
          <p className="mt-1 text-sm text-slate-500">Conoce las especialidades y profesionales.</p>
        </Link>
      </div>

      <section>
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-slate-500">
          Próximas citas
        </h2>
        {cargando ? (
          <div className="flex justify-center py-6">
            <div className="h-6 w-6 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
          </div>
        ) : proximas.length === 0 ? (
          <p className="rounded-lg border border-slate-200 bg-white px-4 py-3 text-sm text-slate-500">
            No tienes citas próximas.
          </p>
        ) : (
          <div className="space-y-3">
            {proximas.map((cita) => {
              const estado = etiquetaEstado(cita.estado);
              return (
                <Link
                  key={cita.id}
                  to={`/paciente/citas/${cita.id}`}
                  className="flex flex-wrap items-center gap-3 rounded-2xl border border-slate-200 bg-white p-4 shadow-sm transition hover:border-brand-400"
                >
                  <div className="min-w-0 flex-1">
                    <p className="font-medium text-slate-900">
                      {cita.medico_nombre} · {cita.especialidad_nombre}
                    </p>
                    <p className="text-sm text-slate-500">{formatoFechaGuayaquil(cita.fecha_hora_inicio)}</p>
                  </div>
                  <span className={`rounded-full px-2.5 py-1 text-xs font-semibold ${estado.clase}`}>
                    {estado.texto}
                  </span>
                </Link>
              );
            })}
          </div>
        )}
      </section>
    </div>
  );
}
