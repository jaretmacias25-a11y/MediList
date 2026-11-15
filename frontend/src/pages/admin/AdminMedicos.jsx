import { useEffect, useMemo, useState } from "react";
import { ApiError } from "@/lib/http";
import {
  actualizarMedico,
  crearMedico,
  eliminarMedico,
  listarMedicosAdmin,
} from "@/services/catalogo";
import { listarEspecialidadesAdmin } from "@/services/catalogo";
import { convertirPacienteEnMedico } from "@/services/admin";
import { listarPacientes } from "@/services/recepcion";

const inputClass =
  "w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 shadow-sm outline-none transition focus:border-brand-500 focus:ring-2 focus:ring-brand-100";

const FORM_INICIAL = {
  tipo_documento: "CEDULA",
  numero_documento: "",
  nombres: "",
  apellidos: "",
  fecha_nacimiento: "",
  sexo: "masculino",
  telefono: "",
  email: "",
  direccion: "",
  especialidad_id: "",
  numero_licencia: "",
  tipo_profesional: "medico",
  duracion_cita_min: 30,
  acepta_portal: true,
};

export default function AdminMedicos() {
  const [items, setItems] = useState([]);
  const [especialidades, setEspecialidades] = useState([]);
  const [search, setSearch] = useState("");
  const [cargando, setCargando] = useState(true);
  const [error, setError] = useState(null);
  const [exito, setExito] = useState(null);

  const [editando, setEditando] = useState(null); // null = nuevo
  const [mostrar, setMostrar] = useState(false);
  const [form, setForm] = useState(FORM_INICIAL);
  const [guardando, setGuardando] = useState(false);
  const [eliminando, setEliminando] = useState(null);

  const [convirtiendo, setConvirtiendo] = useState(false);
  const [buscarPaciente, setBuscarPaciente] = useState("");
  const [pacientesConv, setPacientesConv] = useState([]);
  const [pacienteConv, setPacienteConv] = useState(null);
  const [buscandoPacientes, setBuscandoPacientes] = useState(false);
  const [guardandoConv, setGuardandoConv] = useState(false);
  const [formConv, setFormConv] = useState({
    especialidad_id: "",
    numero_licencia: "",
    tipo_profesional: "medico",
    duracion_cita_min: 30,
    bio: "",
    acepta_portal: true,
  });

  async function cargar() {
    setCargando(true);
    setError(null);
    try {
      const [medicos, especialidadesResp] = await Promise.all([
        listarMedicosAdmin(),
        listarEspecialidadesAdmin(),
      ]);
      setItems(medicos);
      setEspecialidades(especialidadesResp.filter((e) => e.activa));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "No se pudieron cargar los médicos.");
    } finally {
      setCargando(false);
    }
  }

  useEffect(() => {
    cargar();
  }, []);

  async function buscarPacientesConv() {
    setBuscandoPacientes(true);
    setError(null);
    try {
      const resultado = await listarPacientes(buscarPaciente, 10, 0);
      setPacientesConv(resultado.items.filter((p) => p.usuario_id));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "No se pudieron buscar pacientes.");
    } finally {
      setBuscandoPacientes(false);
    }
  }

  async function convertir() {
    if (!pacienteConv?.usuario_id) {
      setError("Selecciona un paciente con cuenta de usuario.");
      return;
    }
    if (!formConv.especialidad_id || !formConv.numero_licencia.trim()) {
      setError("Selecciona una especialidad e ingresa el número de licencia.");
      return;
    }
    setGuardandoConv(true);
    setError(null);
    try {
      await convertirPacienteEnMedico({
        usuario_id: pacienteConv.usuario_id,
        especialidad_id: Number(formConv.especialidad_id),
        numero_licencia: formConv.numero_licencia.trim(),
        tipo_profesional: formConv.tipo_profesional,
        duracion_cita_min: Number(formConv.duracion_cita_min),
        bio: formConv.bio.trim(),
        foto_url: "",
        acepta_portal: formConv.acepta_portal,
      });
      setExito(
        "Paciente convertido en médico. Sus sesiones fueron revocadas; debe iniciar sesión de nuevo.",
      );
      setConvirtiendo(false);
      setPacienteConv(null);
      setPacientesConv([]);
      await cargar();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "No se pudo convertir al paciente.");
    } finally {
      setGuardandoConv(false);
    }
  }

  const filtrados = useMemo(() => {
    const texto = search.trim().toLowerCase();
    if (!texto) return items;
    return items.filter((medico) =>
      [
        medico.persona.nombres,
        medico.persona.apellidos,
        medico.numero_licencia,
        medico.persona.numero_documento,
      ]
        .join(" ")
        .toLowerCase()
        .includes(texto),
    );
  }, [items, search]);

  async function guardar() {
    setGuardando(true);
    setError(null);
    try {
      const input = {
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
        especialidad_id: Number(form.especialidad_id),
        numero_licencia: form.numero_licencia.trim(),
        tipo_profesional: form.tipo_profesional,
        duracion_cita_min: Number(form.duracion_cita_min),
        bio: "",
        foto_url: "",
        acepta_portal: form.acepta_portal,
        version: editando?.version ?? 0,
      };
      if (editando) {
        await actualizarMedico(editando.id, input);
      } else {
        await crearMedico(input);
      }
      setMostrar(false);
      setExito(editando ? "Médico actualizado." : "Médico creado.");
      await cargar();
    } catch (err) {
      if (err instanceof ApiError && err.isConflict) {
        setError("El médico cambió en otra sesión. Se recargaron los datos.");
        setMostrar(false);
        await cargar();
      } else {
        setError(err instanceof ApiError ? err.message : "No se pudo guardar el médico.");
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
      await eliminarMedico(eliminando.id);
      setEliminando(null);
      setExito("Médico eliminado.");
      await cargar();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "No se pudo eliminar el médico.");
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
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-bold text-slate-900">Médicos</h1>
          <p className="mt-1 text-sm text-slate-500">Profesionales de la clínica.</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <button
            type="button"
            onClick={() => {
              setConvirtiendo(true);
              setPacienteConv(null);
              setPacientesConv([]);
              setBuscarPaciente("");
              setFormConv({
                especialidad_id: especialidades[0]?.id ? String(especialidades[0].id) : "",
                numero_licencia: "",
                tipo_profesional: "medico",
                duracion_cita_min: 30,
                bio: "",
                acepta_portal: true,
              });
            }}
            className="rounded-lg border border-brand-300 px-4 py-2 text-sm font-semibold text-brand-700 hover:bg-brand-50"
          >
            Convertir paciente
          </button>
          <button
            type="button"
            onClick={() => {
              setEditando(null);
              setForm({ ...FORM_INICIAL, especialidad_id: especialidades[0]?.id ? String(especialidades[0].id) : "" });
              setMostrar(true);
            }}
            className="rounded-lg bg-brand-600 px-4 py-2 text-sm font-semibold text-white hover:bg-brand-700"
          >
            Nuevo médico
          </button>
        </div>
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

      {convirtiendo && (
        <section className="space-y-4 rounded-2xl border border-brand-200 bg-brand-50 p-4">
          <div>
            <h2 className="text-base font-semibold text-slate-900">
              Convertir paciente en médico
            </h2>
            <p className="mt-1 text-sm text-slate-600">
              Reutiliza la persona y la historia clínica del paciente. Se crea el
              perfil de médico, se vincula la cuenta y se reemplaza el rol por
              <strong> medico</strong>.
            </p>
          </div>

          <div className="flex flex-wrap gap-2">
            <input
              type="search"
              value={buscarPaciente}
              onChange={(e) => setBuscarPaciente(e.target.value)}
              placeholder="Buscar paciente por nombre o documento"
              className={`${inputClass} max-w-md flex-1`}
            />
            <button
              type="button"
              onClick={buscarPacientesConv}
              disabled={buscandoPacientes}
              className="rounded-lg bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-800 disabled:opacity-60"
            >
              {buscandoPacientes ? "Buscando…" : "Buscar"}
            </button>
          </div>

          {pacientesConv.length > 0 && (
            <ul className="max-h-48 divide-y divide-slate-200 overflow-auto rounded-lg border border-slate-200 bg-white">
              {pacientesConv.map((paciente) => (
                <li key={paciente.id}>
                  <button
                    type="button"
                    onClick={() => setPacienteConv(paciente)}
                    className={`flex w-full items-center justify-between gap-3 px-3 py-2 text-left text-sm hover:bg-slate-50 ${
                      pacienteConv?.id === paciente.id ? "bg-brand-50" : ""
                    }`}
                  >
                    <span>
                      <span className="font-medium text-slate-900">
                        {paciente.persona.nombres} {paciente.persona.apellidos}
                      </span>
                      <span className="ml-2 text-slate-500">
                        {paciente.persona.numero_documento} · {paciente.numero_historia}
                      </span>
                    </span>
                    <span className="text-xs text-brand-700">Seleccionar</span>
                  </button>
                </li>
              ))}
            </ul>
          )}

          {pacienteConv && (
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <p className="sm:col-span-2 text-sm text-slate-700">
                Paciente seleccionado:{" "}
                <strong>
                  {pacienteConv.persona.nombres} {pacienteConv.persona.apellidos}
                </strong>
              </p>
              <div>
                <label htmlFor="conv_especialidad" className="mb-1 block text-sm font-medium text-slate-700">
                  Especialidad
                </label>
                <select
                  id="conv_especialidad"
                  value={formConv.especialidad_id}
                  onChange={(e) => setFormConv((f) => ({ ...f, especialidad_id: e.target.value }))}
                  className={inputClass}
                >
                  {especialidades.map((esp) => (
                    <option key={esp.id} value={esp.id}>
                      {esp.nombre}
                    </option>
                  ))}
                </select>
              </div>
              <div>
                <label htmlFor="conv_licencia" className="mb-1 block text-sm font-medium text-slate-700">
                  Número de licencia
                </label>
                <input
                  id="conv_licencia"
                  value={formConv.numero_licencia}
                  onChange={(e) => setFormConv((f) => ({ ...f, numero_licencia: e.target.value }))}
                  className={inputClass}
                />
              </div>
              <div>
                <label htmlFor="conv_duracion" className="mb-1 block text-sm font-medium text-slate-700">
                  Duración de cita (min)
                </label>
                <input
                  id="conv_duracion"
                  type="number"
                  min="5"
                  max="240"
                  value={formConv.duracion_cita_min}
                  onChange={(e) => setFormConv((f) => ({ ...f, duracion_cita_min: e.target.value }))}
                  className={inputClass}
                />
              </div>
              <div>
                <label htmlFor="conv_bio" className="mb-1 block text-sm font-medium text-slate-700">
                  Bio (opcional)
                </label>
                <input
                  id="conv_bio"
                  value={formConv.bio}
                  onChange={(e) => setFormConv((f) => ({ ...f, bio: e.target.value }))}
                  className={inputClass}
                />
              </div>
              <div className="sm:col-span-2 flex flex-wrap gap-2">
                <button
                  type="button"
                  onClick={convertir}
                  disabled={guardandoConv}
                  className="rounded-lg bg-brand-600 px-4 py-2 text-sm font-semibold text-white hover:bg-brand-700 disabled:opacity-60"
                >
                  {guardandoConv ? "Convirtiendo…" : "Convertir en médico"}
                </button>
                <button
                  type="button"
                  onClick={() => setConvirtiendo(false)}
                  className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
                >
                  Cancelar
                </button>
              </div>
            </div>
          )}
        </section>
      )}

      <div>
        <label htmlFor="busqueda" className="mb-1 block text-sm font-medium text-slate-700">
          Buscar
        </label>
        <input
          id="busqueda"
          type="search"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Nombre, documento o licencia"
          className={inputClass}
        />
      </div>

      <div className="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
        <ul className="divide-y divide-slate-100">
          {filtrados.map((medico) => (
            <li key={medico.id} className="flex flex-wrap items-center gap-3 px-4 py-3">
              <div className="min-w-0 flex-1">
                <p className="font-medium text-slate-900">
                  {medico.persona.nombres} {medico.persona.apellidos}
                </p>
                <p className="text-sm text-slate-500">
                  {medico.especialidad.nombre} · {medico.numero_licencia}
                </p>
              </div>
              <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${
                medico.activo ? "bg-emerald-100 text-emerald-800" : "bg-slate-200 text-slate-600"
              }`}>
                {medico.activo ? "Activo" : "Inactivo"}
              </span>
              <button
                type="button"
                onClick={() => {
                  setEditando(medico);
                  setForm({
                    tipo_documento: medico.persona.tipo_documento,
                    numero_documento: medico.persona.numero_documento,
                    nombres: medico.persona.nombres,
                    apellidos: medico.persona.apellidos,
                    fecha_nacimiento: medico.persona.fecha_nacimiento,
                    sexo: medico.persona.sexo,
                    telefono: medico.persona.telefono,
                    email: medico.persona.email,
                    direccion: medico.persona.direccion,
                    especialidad_id: String(medico.especialidad.id),
                    numero_licencia: medico.numero_licencia,
                    tipo_profesional: medico.tipo_profesional,
                    duracion_cita_min: medico.duracion_cita_min,
                    acepta_portal: medico.acepta_portal,
                  });
                  setMostrar(true);
                }}
                className="rounded-lg border border-slate-300 px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
              >
                Editar
              </button>
              <button
                type="button"
                onClick={() => setEliminando(medico)}
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
          <div className="relative max-h-[90vh] w-full max-w-2xl overflow-y-auto rounded-2xl bg-white p-6 shadow-xl">
            <h2 className="text-lg font-bold text-slate-900">
              {editando ? "Editar médico" : "Nuevo médico"}
            </h2>
            <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div>
                <label className="mb-1 block text-sm font-medium text-slate-700">Tipo de documento</label>
                <select value={form.tipo_documento} onChange={(e) => setForm((f) => ({ ...f, tipo_documento: e.target.value }))} className={inputClass}>
                  {["CEDULA", "PASAPORTE", "RUC", "OTRO"].map((tipo) => (
                    <option key={tipo} value={tipo}>{tipo}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="mb-1 block text-sm font-medium text-slate-700">Número de documento</label>
                <input value={form.numero_documento} onChange={(e) => setForm((f) => ({ ...f, numero_documento: e.target.value }))} className={inputClass} />
              </div>
              <div>
                <label className="mb-1 block text-sm font-medium text-slate-700">Nombres</label>
                <input value={form.nombres} onChange={(e) => setForm((f) => ({ ...f, nombres: e.target.value }))} className={inputClass} />
              </div>
              <div>
                <label className="mb-1 block text-sm font-medium text-slate-700">Apellidos</label>
                <input value={form.apellidos} onChange={(e) => setForm((f) => ({ ...f, apellidos: e.target.value }))} className={inputClass} />
              </div>
              <div>
                <label className="mb-1 block text-sm font-medium text-slate-700">Fecha de nacimiento</label>
                <input type="date" value={form.fecha_nacimiento} onChange={(e) => setForm((f) => ({ ...f, fecha_nacimiento: e.target.value }))} className={inputClass} />
              </div>
              <div>
                <label className="mb-1 block text-sm font-medium text-slate-700">Sexo</label>
                <select value={form.sexo} onChange={(e) => setForm((f) => ({ ...f, sexo: e.target.value }))} className={inputClass}>
                  {["femenino", "masculino", "otro"].map((sexo) => (
                    <option key={sexo} value={sexo}>{sexo}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="mb-1 block text-sm font-medium text-slate-700">Teléfono</label>
                <input value={form.telefono} onChange={(e) => setForm((f) => ({ ...f, telefono: e.target.value }))} className={inputClass} />
              </div>
              <div>
                <label className="mb-1 block text-sm font-medium text-slate-700">Email</label>
                <input type="email" value={form.email} onChange={(e) => setForm((f) => ({ ...f, email: e.target.value }))} className={inputClass} />
              </div>
              <div className="sm:col-span-2">
                <label className="mb-1 block text-sm font-medium text-slate-700">Dirección</label>
                <input value={form.direccion} onChange={(e) => setForm((f) => ({ ...f, direccion: e.target.value }))} className={inputClass} />
              </div>
              <div>
                <label className="mb-1 block text-sm font-medium text-slate-700">Especialidad</label>
                <select value={form.especialidad_id} onChange={(e) => setForm((f) => ({ ...f, especialidad_id: e.target.value }))} className={inputClass}>
                  {especialidades.map((especialidad) => (
                    <option key={especialidad.id} value={String(especialidad.id)}>{especialidad.nombre}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="mb-1 block text-sm font-medium text-slate-700">Número de licencia</label>
                <input value={form.numero_licencia} onChange={(e) => setForm((f) => ({ ...f, numero_licencia: e.target.value }))} className={inputClass} />
              </div>
              <div>
                <label className="mb-1 block text-sm font-medium text-slate-700">Duración de cita (min)</label>
                <input type="number" min={5} max={240} value={form.duracion_cita_min}
                  onChange={(e) => setForm((f) => ({ ...f, duracion_cita_min: Number(e.target.value) }))} className={inputClass} />
              </div>
              <label className="flex items-end gap-2 pb-1 text-sm text-slate-700">
                <input type="checkbox" checked={form.acepta_portal}
                  onChange={(e) => setForm((f) => ({ ...f, acepta_portal: e.target.checked }))}
                  className="h-4 w-4 rounded border-slate-300 text-brand-600 focus:ring-brand-500" />
                Acepta reservas del portal
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
            <h2 className="text-lg font-bold text-slate-900">Eliminar médico</h2>
            <p className="mt-2 text-sm text-slate-600">
              El médico <strong>{eliminando.persona.nombres} {eliminando.persona.apellidos}</strong> se
              desactivará. ¿Deseas continuar?
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
