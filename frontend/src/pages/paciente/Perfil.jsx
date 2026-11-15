import { useEffect, useState } from "react";
import { ApiError } from "@/lib/http";
import { getPerfil, updatePerfil } from "@/services/paciente";

const inputClass =
  "w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 shadow-sm outline-none transition focus:border-brand-500 focus:ring-2 focus:ring-brand-100";

const readonlyClass =
  "w-full rounded-lg border border-slate-200 bg-slate-100 px-3 py-2 text-sm text-slate-500";

function mensajeDeError(error) {
  if (!(error instanceof ApiError)) {
    return "Ocurrió un error inesperado. Intenta nuevamente.";
  }
  switch (error.status) {
    case 400:
      return error.message || "Revisa los datos ingresados.";
    case 409:
      return "El perfil fue modificado en otra sesión. Se recargaron los datos; revisa los cambios y vuelve a intentar.";
    default:
      return error.message || "Ocurrió un error inesperado.";
  }
}

export default function Perfil() {
  const [perfil, setPerfil] = useState(null);
  const [cargando, setCargando] = useState(true);
  const [error, setError] = useState(null);

  const [telefono, setTelefono] = useState("");
  const [direccion, setDireccion] = useState("");
  const [emergenciaNombre, setEmergenciaNombre] = useState("");
  const [emergenciaTelefono, setEmergenciaTelefono] = useState("");

  const [guardando, setGuardando] = useState(false);
  const [exito, setExito] = useState(null);
  const [errorFormulario, setErrorFormulario] = useState(null);

  async function cargarPerfil() {
    setCargando(true);
    setError(null);
    try {
      const datos = await getPerfil();
      setPerfil(datos);
      setTelefono(datos.paciente.persona.telefono);
      setDireccion(datos.paciente.persona.direccion);
      setEmergenciaNombre(datos.paciente.contacto_emergencia_nombre);
      setEmergenciaTelefono(datos.paciente.contacto_emergencia_telefono);
    } catch (err) {
      setError(mensajeDeError(err));
    } finally {
      setCargando(false);
    }
  }

  useEffect(() => {
    cargarPerfil();
  }, []);

  async function handleSubmit(evento) {
    evento.preventDefault();
    setErrorFormulario(null);
    setExito(null);

    if (!perfil) return;

    setGuardando(true);
    try {
      const actualizado = await updatePerfil({
        telefono: telefono.trim(),
        direccion: direccion.trim(),
        contacto_emergencia_nombre: emergenciaNombre.trim(),
        contacto_emergencia_telefono: emergenciaTelefono.trim(),
        version: perfil.paciente.version,
      });

      setPerfil(actualizado);
      setTelefono(actualizado.paciente.persona.telefono);
      setDireccion(actualizado.paciente.persona.direccion);
      setEmergenciaNombre(actualizado.paciente.contacto_emergencia_nombre);
      setEmergenciaTelefono(actualizado.paciente.contacto_emergencia_telefono);
      setExito("Perfil actualizado correctamente.");
    } catch (err) {
      const mensaje = mensajeDeError(err);

      // Conflicto de versión: recargar los datos automáticamente y avisar.
      if (err instanceof ApiError && err.isConflict) {
        setErrorFormulario(mensaje);
        try {
          const datos = await getPerfil();
          setPerfil(datos);
          setTelefono(datos.paciente.persona.telefono);
          setDireccion(datos.paciente.persona.direccion);
          setEmergenciaNombre(datos.paciente.contacto_emergencia_nombre);
          setEmergenciaTelefono(datos.paciente.contacto_emergencia_telefono);
        } catch (errorRecarga) {
          setErrorFormulario(mensajeDeError(errorRecarga));
        }
      } else {
        setErrorFormulario(mensaje);
      }
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

  if (error || !perfil) {
    return (
      <div className="rounded-xl border border-red-200 bg-red-50 p-6 text-sm text-red-700">
        {error ?? "No se pudo cargar el perfil."}
      </div>
    );
  }

  const { paciente, cuenta } = perfil;
  const persona = paciente.persona;

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <header>
        <h1 className="text-xl font-bold text-slate-900">Mi perfil</h1>
        <p className="mt-1 text-sm text-slate-500">
          Actualiza tu información de contacto. Los datos de identidad solo los
          puede cambiar recepción.
        </p>
      </header>

      {exito && (
        <p
          role="status"
          className="rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-800"
        >
          {exito}
        </p>
      )}

      {errorFormulario && (
        <p
          role="alert"
          className="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800"
        >
          {errorFormulario}
        </p>
      )}

      {/* Datos de identidad: solo lectura */}
      <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
        <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-slate-500">
          Datos de identidad
        </h2>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <label className="mb-1 block text-sm font-medium text-slate-700">Tipo de documento</label>
            <input value={persona.tipo_documento} disabled className={readonlyClass} />
          </div>
          <div>
            <label className="mb-1 block text-sm font-medium text-slate-700">Número de documento</label>
            <input value={persona.numero_documento} disabled className={readonlyClass} />
          </div>
          <div>
            <label className="mb-1 block text-sm font-medium text-slate-700">Nombres</label>
            <input value={persona.nombres} disabled className={readonlyClass} />
          </div>
          <div>
            <label className="mb-1 block text-sm font-medium text-slate-700">Apellidos</label>
            <input value={persona.apellidos} disabled className={readonlyClass} />
          </div>
          <div>
            <label className="mb-1 block text-sm font-medium text-slate-700">Fecha de nacimiento</label>
            <input value={persona.fecha_nacimiento} disabled className={readonlyClass} />
          </div>
          <div>
            <label className="mb-1 block text-sm font-medium text-slate-700">Sexo</label>
            <input value={persona.sexo} disabled className={readonlyClass} />
          </div>
          <div>
            <label className="mb-1 block text-sm font-medium text-slate-700">Correo de la cuenta</label>
            <input value={cuenta.email} disabled className={readonlyClass} />
          </div>
          <div>
            <label className="mb-1 block text-sm font-medium text-slate-700">Historia clínica</label>
            <input value={paciente.numero_historia} disabled className={readonlyClass} />
          </div>
        </div>
      </section>

      {/* Contacto editable */}
      <form
        onSubmit={handleSubmit}
        className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm"
      >
        <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-slate-500">
          Información de contacto
        </h2>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <label htmlFor="telefono" className="mb-1 block text-sm font-medium text-slate-700">
              Teléfono
            </label>
            <input
              id="telefono"
              type="tel"
              value={telefono}
              onChange={(e) => setTelefono(e.target.value)}
              className={inputClass}
              placeholder="0999999999"
            />
          </div>
          <div>
            <label htmlFor="direccion" className="mb-1 block text-sm font-medium text-slate-700">
              Dirección
            </label>
            <input
              id="direccion"
              type="text"
              value={direccion}
              onChange={(e) => setDireccion(e.target.value)}
              className={inputClass}
              placeholder="Quito, Ecuador"
            />
          </div>
          <div>
            <label htmlFor="emergencia-nombre" className="mb-1 block text-sm font-medium text-slate-700">
              Contacto de emergencia (nombre)
            </label>
            <input
              id="emergencia-nombre"
              type="text"
              value={emergenciaNombre}
              onChange={(e) => setEmergenciaNombre(e.target.value)}
              className={inputClass}
              placeholder="Nombre del contacto"
            />
          </div>
          <div>
            <label htmlFor="emergencia-telefono" className="mb-1 block text-sm font-medium text-slate-700">
              Contacto de emergencia (teléfono)
            </label>
            <input
              id="emergencia-telefono"
              type="tel"
              value={emergenciaTelefono}
              onChange={(e) => setEmergenciaTelefono(e.target.value)}
              className={inputClass}
              placeholder="0988888888"
            />
          </div>
        </div>

        <div className="mt-6">
          <button
            type="submit"
            disabled={guardando}
            className="rounded-lg bg-brand-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:bg-brand-700 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {guardando ? "Guardando…" : "Guardar cambios"}
          </button>
        </div>
      </form>
    </div>
  );
}
