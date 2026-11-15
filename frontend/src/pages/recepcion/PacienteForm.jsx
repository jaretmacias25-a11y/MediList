import { useState } from "react";

const inputClass =
  "w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 shadow-sm outline-none transition focus:border-brand-500 focus:ring-2 focus:ring-brand-100";

const TIPOS_DOCUMENTO = ["CEDULA", "PASAPORTE", "RUC", "OTRO"];
const SEXOS = ["femenino", "masculino", "otro"];
const FACTORES_RH = ["+", "-"];

const FORM_INICIAL = {
  tipo_documento: "CEDULA",
  numero_documento: "",
  nombres: "",
  apellidos: "",
  fecha_nacimiento: "",
  sexo: "femenino",
  telefono: "",
  email: "",
  direccion: "",
  grupo_sanguineo: "O",
  factor_rh: "+",
  contacto_emergencia_nombre: "",
  contacto_emergencia_telefono: "",
};

export default function PacienteForm({ inicial, onCancelar, onSubmit, guardando }) {
  const [form, setForm] = useState(() => {
    if (!inicial) return FORM_INICIAL;
    return {
      tipo_documento: inicial.persona.tipo_documento,
      numero_documento: inicial.persona.numero_documento,
      nombres: inicial.persona.nombres,
      apellidos: inicial.persona.apellidos,
      fecha_nacimiento: inicial.persona.fecha_nacimiento,
      sexo: inicial.persona.sexo,
      telefono: inicial.persona.telefono,
      email: inicial.persona.email,
      direccion: inicial.persona.direccion,
      grupo_sanguineo: inicial.grupo_sanguineo || "O",
      factor_rh: inicial.factor_rh || "+",
      contacto_emergencia_nombre: inicial.contacto_emergencia_nombre,
      contacto_emergencia_telefono: inicial.contacto_emergencia_telefono,
    };
  });

  function actualizar(campo, valor) {
    setForm((previo) => ({ ...previo, [campo]: valor }));
  }

  function handleSubmit(evento) {
    evento.preventDefault();
    onSubmit({
      persona: {
        tipo_documento: form.tipo_documento,
        numero_documento: form.numero_documento.trim(),
        nombres: form.nombres.trim(),
        apellidos: form.apellidos.trim(),
        fecha_nacimiento: form.fecha_nacimiento,
        sexo: form.sexo,
        telefono: form.telefono.trim(),
        email: form.email.trim(),
        direccion: form.direccion.trim(),
      },
      grupo_sanguineo: form.grupo_sanguineo,
      factor_rh: form.factor_rh,
      contacto_emergencia_nombre: form.contacto_emergencia_nombre.trim(),
      contacto_emergencia_telefono: form.contacto_emergencia_telefono.trim(),
      version: inicial?.version ?? 0,
    });
  }

  return (
    <form onSubmit={handleSubmit} className="grid grid-cols-1 gap-4 sm:grid-cols-2">
      <div>
        <label className="mb-1 block text-sm font-medium text-slate-700">Tipo de documento</label>
        <select value={form.tipo_documento} onChange={(e) => actualizar("tipo_documento", e.target.value)} className={inputClass}>
          {TIPOS_DOCUMENTO.map((tipo) => (
            <option key={tipo} value={tipo}>{tipo}</option>
          ))}
        </select>
      </div>
      <div>
        <label className="mb-1 block text-sm font-medium text-slate-700">Número de documento</label>
        <input required value={form.numero_documento} onChange={(e) => actualizar("numero_documento", e.target.value)} className={inputClass} />
      </div>
      <div>
        <label className="mb-1 block text-sm font-medium text-slate-700">Nombres</label>
        <input required value={form.nombres} onChange={(e) => actualizar("nombres", e.target.value)} className={inputClass} />
      </div>
      <div>
        <label className="mb-1 block text-sm font-medium text-slate-700">Apellidos</label>
        <input required value={form.apellidos} onChange={(e) => actualizar("apellidos", e.target.value)} className={inputClass} />
      </div>
      <div>
        <label className="mb-1 block text-sm font-medium text-slate-700">Fecha de nacimiento</label>
        <input type="date" required value={form.fecha_nacimiento} onChange={(e) => actualizar("fecha_nacimiento", e.target.value)} className={inputClass} />
      </div>
      <div>
        <label className="mb-1 block text-sm font-medium text-slate-700">Sexo</label>
        <select value={form.sexo} onChange={(e) => actualizar("sexo", e.target.value)} className={inputClass}>
          {SEXOS.map((sexo) => (
            <option key={sexo} value={sexo}>{sexo}</option>
          ))}
        </select>
      </div>
      <div>
        <label className="mb-1 block text-sm font-medium text-slate-700">Teléfono</label>
        <input required value={form.telefono} onChange={(e) => actualizar("telefono", e.target.value)} className={inputClass} />
      </div>
      <div>
        <label className="mb-1 block text-sm font-medium text-slate-700">Email</label>
        <input type="email" value={form.email} onChange={(e) => actualizar("email", e.target.value)} className={inputClass} />
      </div>
      <div className="sm:col-span-2">
        <label className="mb-1 block text-sm font-medium text-slate-700">Dirección</label>
        <input value={form.direccion} onChange={(e) => actualizar("direccion", e.target.value)} className={inputClass} />
      </div>
      <div>
        <label className="mb-1 block text-sm font-medium text-slate-700">Grupo sanguíneo</label>
        <input value={form.grupo_sanguineo} onChange={(e) => actualizar("grupo_sanguineo", e.target.value)} className={inputClass} />
      </div>
      <div>
        <label className="mb-1 block text-sm font-medium text-slate-700">Factor RH</label>
        <select value={form.factor_rh} onChange={(e) => actualizar("factor_rh", e.target.value)} className={inputClass}>
          {FACTORES_RH.map((factor) => (
            <option key={factor} value={factor}>{factor}</option>
          ))}
        </select>
      </div>
      <div>
        <label className="mb-1 block text-sm font-medium text-slate-700">Contacto de emergencia (nombre)</label>
        <input value={form.contacto_emergencia_nombre} onChange={(e) => actualizar("contacto_emergencia_nombre", e.target.value)} className={inputClass} />
      </div>
      <div>
        <label className="mb-1 block text-sm font-medium text-slate-700">Contacto de emergencia (teléfono)</label>
        <input value={form.contacto_emergencia_telefono} onChange={(e) => actualizar("contacto_emergencia_telefono", e.target.value)} className={inputClass} />
      </div>

      <div className="flex justify-end gap-3 sm:col-span-2">
        <button
          type="button"
          onClick={onCancelar}
          className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
        >
          Cancelar
        </button>
        <button
          type="submit"
          disabled={guardando}
          className="rounded-lg bg-brand-600 px-4 py-2 text-sm font-semibold text-white hover:bg-brand-700 disabled:cursor-not-allowed disabled:opacity-60"
        >
          {guardando ? "Guardando…" : "Guardar"}
        </button>
      </div>
    </form>
  );
}
