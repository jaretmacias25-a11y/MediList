export const ETIQUETAS_ESTADO = {
  programada: { texto: "Programada", clase: "bg-blue-100 text-blue-800" },
  confirmada: { texto: "Confirmada", clase: "bg-emerald-100 text-emerald-800" },
  atendida: { texto: "Atendida", clase: "bg-slate-200 text-slate-700" },
  no_asistio: { texto: "No asistió", clase: "bg-red-100 text-red-800" },
  cancelada: { texto: "Cancelada", clase: "bg-slate-200 text-slate-600" },
  reprogramada: { texto: "Reprogramada", clase: "bg-amber-100 text-amber-800" },
};

export const ETIQUETAS_TIPO = {
  primera_vez: "Primera vez",
  control: "Control",
  urgencia: "Urgencia",
  telemedicina: "Telemedicina",
};

export function formatoFechaGuayaquil(fechaISO) {
  return new Intl.DateTimeFormat("es-EC", {
    weekday: "long",
    day: "numeric",
    month: "long",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    timeZone: "America/Guayaquil",
  }).format(new Date(fechaISO));
}

export function esTerminal(estado) {
  return ["atendida", "no_asistio", "cancelada", "reprogramada"].includes(estado);
}

export function etiquetaEstado(estado) {
  return ETIQUETAS_ESTADO[estado] ?? {
    texto: estado,
    clase: "bg-slate-100 text-slate-700",
  };
}

export function etiquetaTipo(tipo) {
  return ETIQUETAS_TIPO[tipo] ?? tipo;
}
