import { useEffect, useState } from "react";
import { ApiError } from "@/lib/http";
import {
  actualizarEspecialidad,
  crearEspecialidad,
  eliminarEspecialidad,
  listarEspecialidadesAdmin,
} from "@/services/catalogo";

const inputClass =
  "w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 shadow-sm outline-none transition focus:border-brand-500 focus:ring-2 focus:ring-brand-100";

const FORM_INICIAL = {
  codigo: "",
  nombre: "",
  descripcion: "",
  requiere_derivacion: false,
  activa: true,
  orden: 0,
  duracion_cita_min: 30,
};

export default function AdminEspecialidades() {
  const [items, setItems] = useState([]);
  const [cargando, setCargando] = useState(true);
  const [error, setError] = useState(null);
  const [exito, setExito] = useState(null);

  const [editando, setEditando] = useState(null); // null = nueva
  const [mostrar, setMostrar] = useState(false);
  const [form, setForm] = useState(FORM_INICIAL);
  const [guardando, setGuardando] = useState(false);
  const [eliminando, setEliminando] = useState(null);

  async function cargar() {
    setCargando(true);
    setError(null);
    try {
      setItems(await listarEspecialidadesAdmin());
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "No se pudieron cargar las especialidades.");
    } finally {
      setCargando(false);
    }
  }

  useEffect(() => {
    cargar();
  }, []);

  async function guardar() {
    setGuardando(true);
    setError(null);
    try {
      const input = {
        codigo: form.codigo.trim(),
        nombre: form.nombre.trim(),
        descripcion: form.descripcion.trim(),
        requiere_derivacion: form.requiere_derivacion,
        activa: form.activa,
        orden: Number(form.orden),
        duracion_cita_min: Number(form.duracion_cita_min),
        version: editando?.version ?? 0,
      };
      if (editando) {
        await actualizarEspecialidad(editando.id, input);
      } else {
        await crearEspecialidad(input);
      }
      setMostrar(false);
      setExito(editando ? "Especialidad actualizada." : "Especialidad creada.");
      await cargar();
    } catch (err) {
      if (err instanceof ApiError && err.isConflict) {
        setError("La especialidad cambió en otra sesión. Se recargaron los datos.");
        setMostrar(false);
        await cargar();
      } else {
        setError(err instanceof ApiError ? err.message : "No se pudo guardar la especialidad.");
      }
    } finally {
      setGuardando(false);
    }
  }

  async function eliminar() {
    if (!eliminando) return;
    setGuardando(true);
    setError(null);
    try {
      await eliminarEspecialidad(eliminando.id);
      setEliminando(null);
      setExito("Especialidad eliminada.");
      await cargar();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "No se pudo eliminar la especialidad.");
      setEliminando(null);
    } finally {
      setGuardando(false);
    }
  }

  if (cargando) {
    return (
      <div className="flex h-64 items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-4xl space-y-6">
      <header className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-slate-900">Especialidades</h1>
          <p className="mt-1 text-sm text-slate-500">Catálogo de especialidades de la clínica.</p>
        </div>
        <button
          type="button"
          onClick={() => {
            setEditando(null);
            setForm(FORM_INICIAL);
            setMostrar(true);
          }}
          className="rounded-lg bg-brand-600 px-4 py-2 text-sm font-semibold text-white hover:bg-brand-700"
        >
          Nueva especialidad
        </button>
      </header>

      {exito && (
        <p role="status" className="rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-800">
          {exito}
        </p>
      )}
      {error && (
        <p role="alert" className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
          {error}
        </p>
      )}

      <div className="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
        <ul className="divide-y divide-slate-100">
          {items.map((especialidad) => (
            <li key={especialidad.id} className="flex flex-wrap items-center gap-3 px-4 py-3">
              <div className="min-w-0 flex-1">
                <p className="font-medium text-slate-900">
                  {especialidad.nombre}{" "}
                  <span className="text-xs font-normal text-slate-400">({especialidad.codigo})</span>
                </p>
                <p className="text-sm text-slate-500">
                  {especialidad.duracion_cita_min} min
                  {especialidad.requiere_derivacion && " · requiere derivación"}
                </p>
              </div>
              <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${
                especialidad.activa ? "bg-emerald-100 text-emerald-800" : "bg-slate-200 text-slate-600"
              }`}>
                {especialidad.activa ? "Activa" : "Inactiva"}
              </span>
              <button
                type="button"
                onClick={() => {
                  setEditando(especialidad);
                  setForm({
                    codigo: especialidad.codigo,
                    nombre: especialidad.nombre,
                    descripcion: especialidad.descripcion,
                    requiere_derivacion: especialidad.requiere_derivacion,
                    activa: especialidad.activa,
                    orden: especialidad.orden,
                    duracion_cita_min: especialidad.duracion_cita_min,
                  });
                  setMostrar(true);
                }}
                className="rounded-lg border border-slate-300 px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
              >
                Editar
              </button>
              <button
                type="button"
                onClick={() => setEliminando(especialidad)}
                className="rounded-lg border border-red-300 px-3 py-1.5 text-sm font-medium text-red-600 hover:bg-red-50"
              >
                Eliminar
              </button>
            </li>
          ))}
        </ul>
      </div>

      {mostrar && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div className="absolute inset-0 bg-slate-900/40" onClick={() => setMostrar(false)} />
          <div className="relative w-full max-w-md rounded-2xl bg-white p-6 shadow-xl">
            <h2 className="text-lg font-bold text-slate-900">
              {editando ? "Editar especialidad" : "Nueva especialidad"}
            </h2>
            <div className="mt-4 grid grid-cols-1 gap-4">
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="mb-1 block text-sm font-medium text-slate-700">Código</label>
                  <input value={form.codigo} onChange={(e) => setForm((f) => ({ ...f, codigo: e.target.value }))} className={inputClass} />
                </div>
                <div>
                  <label className="mb-1 block text-sm font-medium text-slate-700">Nombre</label>
                  <input value={form.nombre} onChange={(e) => setForm((f) => ({ ...f, nombre: e.target.value }))} className={inputClass} />
                </div>
              </div>
              <div>
                <label className="mb-1 block text-sm font-medium text-slate-700">Descripción</label>
                <textarea rows={2} value={form.descripcion} onChange={(e) => setForm((f) => ({ ...f, descripcion: e.target.value }))} className={inputClass} />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="mb-1 block text-sm font-medium text-slate-700">Duración (min)</label>
                  <input type="number" min={5} max={240} value={form.duracion_cita_min}
                    onChange={(e) => setForm((f) => ({ ...f, duracion_cita_min: Number(e.target.value) }))} className={inputClass} />
                </div>
                <div>
                  <label className="mb-1 block text-sm font-medium text-slate-700">Orden</label>
                  <input type="number" value={form.orden}
                    onChange={(e) => setForm((f) => ({ ...f, orden: Number(e.target.value) }))} className={inputClass} />
                </div>
              </div>
              <label className="flex items-center gap-2 text-sm text-slate-700">
                <input type="checkbox" checked={form.requiere_derivacion}
                  onChange={(e) => setForm((f) => ({ ...f, requiere_derivacion: e.target.checked }))}
                  className="h-4 w-4 rounded border-slate-300 text-brand-600 focus:ring-brand-500" />
                Requiere derivación
              </label>
              <label className="flex items-center gap-2 text-sm text-slate-700">
                <input type="checkbox" checked={form.activa}
                  onChange={(e) => setForm((f) => ({ ...f, activa: e.target.checked }))}
                  className="h-4 w-4 rounded border-slate-300 text-brand-600 focus:ring-brand-500" />
                Activa
              </label>
            </div>
            <div className="mt-5 flex justify-end gap-3">
              <button type="button" onClick={() => setMostrar(false)}
                className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50">
                Cancelar
              </button>
              <button type="button" disabled={guardando} onClick={guardar}
                className="rounded-lg bg-brand-600 px-4 py-2 text-sm font-semibold text-white hover:bg-brand-700 disabled:opacity-60">
                {guardando ? "Guardando…" : "Guardar"}
              </button>
            </div>
          </div>
        </div>
      )}

      {eliminando && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div className="absolute inset-0 bg-slate-900/40" onClick={() => setEliminando(null)} />
          <div className="relative w-full max-w-sm rounded-2xl bg-white p-6 shadow-xl">
            <h2 className="text-lg font-bold text-slate-900">Eliminar especialidad</h2>
            <p className="mt-2 text-sm text-slate-600">
              La especialidad <strong>{eliminando.nombre}</strong> se desactivará. ¿Deseas continuar?
            </p>
            <div className="mt-5 flex justify-end gap-3">
              <button type="button" onClick={() => setEliminando(null)}
                className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50">
                Cancelar
              </button>
              <button type="button" disabled={guardando} onClick={eliminar}
                className="rounded-lg bg-red-600 px-4 py-2 text-sm font-semibold text-white hover:bg-red-700 disabled:opacity-60">
                Eliminar
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
