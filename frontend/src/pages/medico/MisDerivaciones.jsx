import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { ApiError } from "@/lib/http";
import { listarDerivaciones } from "@/services/medico";

const ETIQUETAS_ESTADO = {
  pendiente: { texto: "Pendiente", clase: "bg-blue-100 text-blue-800" },
  agendada: { texto: "Agendada", clase: "bg-emerald-100 text-emerald-800" },
  atendida: { texto: "Atendida", clase: "bg-slate-200 text-slate-700" },
  vencida: { texto: "Vencida", clase: "bg-amber-100 text-amber-800" },
  cancelada: { texto: "Cancelada", clase: "bg-red-100 text-red-800" },
};

function etiquetaEstado(estado) {
  return ETIQUETAS_ESTADO[estado] ?? {
    texto: estado,
    clase: "bg-slate-100 text-slate-700",
  };
}

function formatoFecha(fechaISO) {
  return new Intl.DateTimeFormat("es-EC", {
    day: "numeric",
    month: "short",
    year: "numeric",
    timeZone: "America/Guayaquil",
  }).format(new Date(fechaISO));
}

export default function MisDerivaciones() {
  const [derivaciones, setDerivaciones] = useState([]);
  const [cargando, setCargando] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    let activo = true;
    listarDerivaciones()
      .then((lista) => {
        if (activo) setDerivaciones(lista);
      })
      .catch((err) => {
        if (activo) {
          if (err instanceof ApiError && err.isFeatureDisabled) {
            setError("El módulo de derivaciones no está disponible en este momento.");
          } else {
            setError(err instanceof ApiError ? err.message : "No se pudieron cargar las derivaciones.");
          }
        }
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
    <div className="mx-auto max-w-4xl space-y-6">
      <header>
        <h1 className="text-xl font-bold text-slate-900">Mis derivaciones</h1>
        <p className="mt-1 text-sm text-slate-500">
          Derivaciones emitidas desde tus citas de Medicina General.
        </p>
      </header>

      {derivaciones.length === 0 ? (
        <div className="rounded-lg border border-slate-200 bg-white px-4 py-6 text-center text-sm text-slate-500">
          No has emitido derivaciones. Genera una desde una cita de Medicina
          General atendida.
          <Link to="/medico/agenda" className="mt-3 block font-medium text-brand-600 hover:text-brand-700">
            Ir a mi agenda
          </Link>
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-3">
          {derivaciones.map((derivacion) => {
            const estado = etiquetaEstado(derivacion.estado);
            return (
              <div
                key={derivacion.id}
                className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm"
              >
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div className="min-w-0">
                    <p className="text-xs text-slate-500">
                      Derivación #{derivacion.id}
                    </p>
                    <p className="mt-1 font-semibold text-slate-900">
                      {derivacion.especialidad_destino}
                    </p>
                    <p className="text-sm text-slate-500">
                      Paciente: {derivacion.paciente_nombre}
                    </p>
                    {derivacion.medico_destino_nombre && (
                      <p className="text-sm text-slate-500">
                        Médico destino: {derivacion.medico_destino_nombre}
                      </p>
                    )}
                  </div>
                  <div className="flex shrink-0 flex-col items-end gap-1">
                    <span className={`rounded-full px-2.5 py-1 text-xs font-semibold ${estado.clase}`}>
                      {estado.texto}
                    </span>
                    <span className="text-xs text-slate-500">
                      {derivacion.prioridad === "urgente" ? "Urgente" : "Normal"}
                    </span>
                  </div>
                </div>

                <p className="mt-3 text-sm text-slate-600">
                  {derivacion.motivo}
                </p>

                <p className="mt-2 text-xs text-slate-400">
                  Vigente del {formatoFecha(derivacion.vigente_desde)} al{" "}
                  {formatoFecha(derivacion.vigente_hasta)}
                </p>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
