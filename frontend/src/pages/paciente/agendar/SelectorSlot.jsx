import { useMemo, useState } from "react";

function etiquetaFecha(inicio) {
  const fecha = new Date(inicio);
  return new Intl.DateTimeFormat("es-EC", {
    weekday: "long",
    day: "numeric",
    month: "long",
    year: "numeric",
    timeZone: "America/Guayaquil",
  }).format(fecha);
}

function etiquetaHora(inicio) {
  const fecha = new Date(inicio);
  return new Intl.DateTimeFormat("es-EC", {
    hour: "2-digit",
    minute: "2-digit",
    timeZone: "America/Guayaquil",
  }).format(fecha);
}

/**
 * Selector de slot: agrupa los slots por día (zona America/Guayaquil) y
 * permite elegir fecha y luego hora exacta. Recibe los slots ya ordenados
 * por la API.
 */
export default function SelectorSlot({ slots, onSelect, selectedInicio }) {
  const fechas = useMemo(() => {
    const mapa = new Map();
    for (const slot of slots) {
      const clave = slot.inicio.slice(0, 10);
      if (!mapa.has(clave)) {
        mapa.set(clave, { clave, etiqueta: etiquetaFecha(slot.inicio) });
      }
    }
    return Array.from(mapa.values());
  }, [slots]);

  const [fechaSeleccionada, setFechaSeleccionada] = useState(() =>
    fechas.length > 0 ? fechas[0].clave : null,
  );

  const slotsDelDia = useMemo(() => {
    if (!fechaSeleccionada) return [];
    return slots.filter((slot) => slot.inicio.slice(0, 10) === fechaSeleccionada);
  }, [slots, fechaSeleccionada]);

  if (slots.length === 0) {
    return (
      <div className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
        No hay disponibilidad en el horizonte consultado. Prueba con otro
        médico o vuelve más tarde.
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap gap-2">
        {fechas.map((fecha) => (
          <button
            key={fecha.clave}
            type="button"
            onClick={() => setFechaSeleccionada(fecha.clave)}
            className={`rounded-full border px-3 py-1.5 text-sm font-medium transition ${
              fechaSeleccionada === fecha.clave
                ? "border-brand-600 bg-brand-600 text-white"
                : "border-slate-300 bg-white text-slate-700 hover:border-brand-400"
            }`}
          >
            {fecha.etiqueta}
          </button>
        ))}
      </div>

      <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 md:grid-cols-4">
        {slotsDelDia.map((slot) => {
          const activo = selectedInicio === slot.inicio;
          return (
            <button
              key={slot.inicio}
              type="button"
              onClick={() => onSelect(slot)}
              className={`rounded-lg border px-3 py-2 text-center text-sm font-medium transition ${
                activo
                  ? "border-brand-600 bg-brand-600 text-white"
                  : "border-slate-300 bg-white text-slate-700 hover:border-brand-400"
              }`}
            >
              {etiquetaHora(slot.inicio)}
              <span className="block text-xs font-normal opacity-70">
                {slot.duracion_min} min
              </span>
            </button>
          );
        })}
      </div>
    </div>
  );
}
