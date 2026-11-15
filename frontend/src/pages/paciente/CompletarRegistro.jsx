import { useState } from "react";
import { useAuth, useUser } from "@clerk/react";
import { Link, Navigate, useNavigate } from "react-router-dom";
import { ApiError } from "@/lib/http";
import { clerkEnabled } from "@/lib/clerk";
import { fetchMe, registrarClerk } from "@/services/auth";
import { useAuthStore } from "@/stores/auth";

const inputClass =
  "w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 shadow-sm outline-none transition focus:border-brand-500 focus:ring-2 focus:ring-brand-100";

const TIPOS_DOCUMENTO = ["CEDULA", "PASAPORTE", "RUC", "OTRO"];
const SEXOS = ["femenino", "masculino", "otro", "no_especificado"];

function mensajeDeError(error) {
  if (!(error instanceof ApiError)) {
    return "Ocurrió un error inesperado. Intenta nuevamente.";
  }
  switch (error.status) {
    case 400:
      return error.message || "Revisa los datos ingresados.";
    case 403:
      return "Tu cuenta no puede completar este registro. Contacta a recepción.";
    case 409:
      return (
        error.message ||
        "El documento ya está asociado a una cuenta o tiene una solicitud pendiente."
      );
    case 429:
      return error.retryAfter
        ? `Demasiados intentos. Intenta de nuevo en ${error.retryAfter} segundo(s).`
        : "Demasiados intentos. Espera un momento antes de reintentar.";
    default:
      return error.message || "Ocurrió un error inesperado.";
  }
}

export default function CompletarRegistro() {
  if (!clerkEnabled) {
    return <Navigate to="/sign-in" replace />;
  }
  return <CompletarRegistroInner />;
}

function CompletarRegistroInner() {
  const { isLoaded, isSignedIn, getToken } = useAuth();
  const { user: clerkUser } = useUser();
  const localUser = useAuthStore((state) => state.user);
  const navigate = useNavigate();
  const email =
    localUser?.email ||
    clerkUser?.primaryEmailAddress?.emailAddress ||
    "";
  const [form, setForm] = useState(() => ({
    nombres: localUser?.nombres || clerkUser?.firstName || "",
    apellidos: localUser?.apellidos || clerkUser?.lastName || "",
    tipo_documento: "CEDULA",
    numero_documento: "",
    fecha_nacimiento: "",
    sexo: "femenino",
    telefono: "",
    direccion: "",
  }));
  const [errores, setErrores] = useState({});
  const [errorGeneral, setErrorGeneral] = useState(null);
  const [pendiente, setPendiente] = useState(null);
  const [loading, setLoading] = useState(false);

  if (!isLoaded) {
    return (
      <main className="flex min-h-screen items-center justify-center bg-slate-50">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
      </main>
    );
  }
  if (!isSignedIn) {
    return <Navigate to="/sign-in" replace />;
  }
  if (localUser?.paciente_id) {
    return <Navigate to="/paciente" replace />;
  }
  if (localUser && !localUser.roles.includes("paciente")) {
    return <Navigate to="/forbidden" replace />;
  }

  function actualizarCampo(campo, valor) {
    setForm((previo) => ({ ...previo, [campo]: valor }));
  }

  function validar() {
    const siguientes = {};
    if (!form.nombres.trim()) siguientes.nombres = "El nombre es obligatorio.";
    if (!form.apellidos.trim())
      siguientes.apellidos = "Los apellidos son obligatorios.";
    if (!form.numero_documento.trim())
      siguientes.numero_documento = "El documento es obligatorio.";
    if (!form.fecha_nacimiento)
      siguientes.fecha_nacimiento = "La fecha de nacimiento es obligatoria.";
    if (!form.telefono.trim())
      siguientes.telefono = "El teléfono es obligatorio.";
    if (!form.direccion.trim())
      siguientes.direccion = "La dirección es obligatoria.";
    setErrores(siguientes);
    return Object.keys(siguientes).length === 0;
  }

  async function handleSubmit(evento) {
    evento.preventDefault();
    setErrorGeneral(null);
    if (!validar()) return;

    setLoading(true);
    try {
      const token = await getToken();
      if (!token) {
        throw new Error("Clerk no devolvió un token de sesión.");
      }

      await registrarClerk({
        token,
        nombres: form.nombres.trim(),
        apellidos: form.apellidos.trim(),
        tipo_documento: form.tipo_documento,
        numero_documento: form.numero_documento.trim(),
        fecha_nacimiento: form.fecha_nacimiento,
        sexo: form.sexo,
        telefono: form.telefono.trim(),
        direccion: form.direccion.trim(),
      });

      const me = await fetchMe();
      if (me.paciente_id) {
        navigate("/paciente", { replace: true });
        return;
      }
      setPendiente({
        mensaje:
          "Tu cuenta fue creada. Recepción debe aprobar la vinculación con tu historia clínica.",
      });
    } catch (err) {
      setErrorGeneral(mensajeDeError(err));
    } finally {
      setLoading(false);
    }
  }

  if (pendiente) {
    return (
      <main className="flex min-h-screen items-center justify-center bg-slate-50 p-4">
        <div className="w-full max-w-lg rounded-2xl border border-amber-200 bg-white p-8 text-center shadow-sm">
          <h1 className="text-2xl font-bold text-slate-900">
            Solicitud enviada
          </h1>
          <p className="mt-3 text-sm text-slate-600">
            {pendiente.mensaje}
          </p>
          <p className="mt-2 text-sm text-slate-500">
            Recepción revisará que el documento coincida con tu historia clínica.
          </p>
          <Link
            to="/cuenta-pendiente"
            className="mt-6 inline-block rounded-lg bg-brand-600 px-4 py-2 text-sm font-semibold text-white hover:bg-brand-700"
          >
            Ver estado de mi cuenta
          </Link>
        </div>
      </main>
    );
  }

  return (
    <main className="flex min-h-screen items-center justify-center bg-slate-50 p-4">
      <div className="w-full max-w-2xl rounded-2xl border border-slate-200 bg-white p-8 shadow-sm">
        <div className="mb-6">
          <h1 className="text-2xl font-bold text-slate-900">
            Completa tu registro clínico
          </h1>
          <p className="mt-1 text-sm text-slate-500">
            Tu cuenta de Clerk ya está creada y verificada. Estos datos se usan
            para crear tu historia clínica en MediList.
          </p>
        </div>

        <div className="mb-6 rounded-lg border border-slate-200 bg-slate-50 px-4 py-3 text-sm">
          <span className="font-medium text-slate-700">Correo: </span>
          <span className="text-slate-600">{email}</span>
        </div>

        {errorGeneral && (
          <p
            role="alert"
            className="mb-4 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700"
          >
            {errorGeneral}
          </p>
        )}

        <form
          onSubmit={handleSubmit}
          className="grid grid-cols-1 gap-4 sm:grid-cols-2"
          noValidate
        >
          <div>
            <label htmlFor="nombres" className="mb-1 block text-sm font-medium text-slate-700">
              Nombres
            </label>
            <input
              id="nombres"
              type="text"
              autoComplete="given-name"
              value={form.nombres}
              onChange={(e) => actualizarCampo("nombres", e.target.value)}
              className={inputClass}
            />
            {errores.nombres && (
              <p className="mt-1 text-xs text-red-600">{errores.nombres}</p>
            )}
          </div>

          <div>
            <label htmlFor="apellidos" className="mb-1 block text-sm font-medium text-slate-700">
              Apellidos
            </label>
            <input
              id="apellidos"
              type="text"
              autoComplete="family-name"
              value={form.apellidos}
              onChange={(e) => actualizarCampo("apellidos", e.target.value)}
              className={inputClass}
            />
            {errores.apellidos && (
              <p className="mt-1 text-xs text-red-600">{errores.apellidos}</p>
            )}
          </div>

          <div>
            <label htmlFor="tipo_documento" className="mb-1 block text-sm font-medium text-slate-700">
              Tipo de documento
            </label>
            <select
              id="tipo_documento"
              value={form.tipo_documento}
              onChange={(e) => actualizarCampo("tipo_documento", e.target.value)}
              className={inputClass}
            >
              {TIPOS_DOCUMENTO.map((tipo) => (
                <option key={tipo} value={tipo}>
                  {tipo}
                </option>
              ))}
            </select>
          </div>

          <div>
            <label htmlFor="numero_documento" className="mb-1 block text-sm font-medium text-slate-700">
              Número de documento
            </label>
            <input
              id="numero_documento"
              type="text"
              value={form.numero_documento}
              onChange={(e) => actualizarCampo("numero_documento", e.target.value)}
              className={inputClass}
            />
            {errores.numero_documento && (
              <p className="mt-1 text-xs text-red-600">{errores.numero_documento}</p>
            )}
          </div>

          <div>
            <label htmlFor="fecha_nacimiento" className="mb-1 block text-sm font-medium text-slate-700">
              Fecha de nacimiento
            </label>
            <input
              id="fecha_nacimiento"
              type="date"
              value={form.fecha_nacimiento}
              onChange={(e) => actualizarCampo("fecha_nacimiento", e.target.value)}
              className={inputClass}
            />
            {errores.fecha_nacimiento && (
              <p className="mt-1 text-xs text-red-600">{errores.fecha_nacimiento}</p>
            )}
          </div>

          <div>
            <label htmlFor="sexo" className="mb-1 block text-sm font-medium text-slate-700">
              Sexo
            </label>
            <select
              id="sexo"
              value={form.sexo}
              onChange={(e) => actualizarCampo("sexo", e.target.value)}
              className={inputClass}
            >
              {SEXOS.map((sexo) => (
                <option key={sexo} value={sexo}>
                  {sexo}
                </option>
              ))}
            </select>
          </div>

          <div>
            <label htmlFor="telefono" className="mb-1 block text-sm font-medium text-slate-700">
              Teléfono
            </label>
            <input
              id="telefono"
              type="tel"
              autoComplete="tel"
              value={form.telefono}
              onChange={(e) => actualizarCampo("telefono", e.target.value)}
              className={inputClass}
            />
            {errores.telefono && (
              <p className="mt-1 text-xs text-red-600">{errores.telefono}</p>
            )}
          </div>

          <div>
            <label htmlFor="direccion" className="mb-1 block text-sm font-medium text-slate-700">
              Dirección
            </label>
            <input
              id="direccion"
              type="text"
              autoComplete="street-address"
              value={form.direccion}
              onChange={(e) => actualizarCampo("direccion", e.target.value)}
              className={inputClass}
            />
            {errores.direccion && (
              <p className="mt-1 text-xs text-red-600">{errores.direccion}</p>
            )}
          </div>

          <div className="sm:col-span-2">
            <button
              type="submit"
              disabled={loading}
              className="w-full rounded-lg bg-brand-600 px-4 py-2.5 text-sm font-semibold text-white shadow-sm transition hover:bg-brand-700 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {loading ? "Guardando…" : "Completar registro y entrar al portal"}
            </button>
          </div>
        </form>

        <p className="mt-4 text-center text-xs text-slate-400">
          ¿Los datos no coinciden con tu historia? Contacta a recepción.
        </p>
      </div>
    </main>
  );
}
