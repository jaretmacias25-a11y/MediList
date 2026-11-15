import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { ApiError } from "@/lib/http";
import { actualizarPaciente, eliminarPaciente, obtenerPaciente } from "@/services/recepcion";
import { useAccess } from "@/hooks/useAccess";
import PacienteForm from "./PacienteForm";

const readonlyClass =
  "w-full rounded-lg border border-slate-200 bg-slate-100 px-3 py-2 text-sm text-slate-500";

export default function PacienteDetalle() {
  const { id } = useParams();
  const navigate = useNavigate();
  const { can } = useAccess();
  const puedeEscribir = can("pacientes:escribir");

  const [paciente, setPaciente] = useState(null);
  const [cargando, setCargando] = useState(true);
  const [error, setError] = useState(null);
  const [alerta, setAlerta] = useState(null);
  const [exito, setExito] = useState(null);

  const [editando, setEditando] = useState(false);
  const [guardando, setGuardando] = useState(false);
  const [confirmarEliminar, setConfirmarEliminar] = useState(false);

  async function cargar() {
    setCargando(true);
    setError(null);
    try {
      setPaciente(await obtenerPaciente(Number(id)));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "No se pudo cargar el paciente.");
    } finally {
      setCargando(false);
    }
  }

  useEffect(() => {
    cargar();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  async function actualizar(input) {
    setGuardando(true);
    setAlerta(null);
    try {
      const actualizado = await actualizarPaciente(Number(id), input);
      setPaciente(actualizado);
      setEditando(false);
      setExito("Paciente actualizado.");
    } catch (err) {
      if (err instanceof ApiError && err.isConflict) {
        setAlerta("El paciente fue modificado en otra sesión. Se recargaron los datos.");
        await cargar();
        setEditando(false);
      } else {
        setAlerta(err instanceof ApiError ? err.message : "No se pudo actualizar el paciente.");
      }
    } finally {
      setGuardando(false);
    }
  }

  async function eliminar() {
    setGuardando(true);
    setAlerta(null);
    try {
      await eliminarPaciente(Number(id));
      navigate("/recepcion/pacientes");
    } catch (err) {
      setAlerta(err instanceof ApiError ? err.message : "No se pudo eliminar el paciente.");
      setConfirmarEliminar(false);
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

  if (error || !paciente) {
    return (
      <div className="rounded-xl border border-red-200 bg-red-50 p-6 text-sm text-red-700">
        {error ?? "Paciente no encontrado."}
      </div>
    );
  }

  const persona = paciente.persona;

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <Link to="/recepcion/pacientes" className="text-sm font-medium text-brand-600 hover:text-brand-700">
            ← Volver a pacientes
          </Link>
          <h1 className="mt-2 text-xl font-bold text-slate-900">
            {persona.nombres} {persona.apellidos}
          </h1>
          <p className="text-sm text-slate-500">
            Historia {paciente.numero_historia}
          </p>
        </div>
        {puedeEscribir && !editando && (
          <div className="flex gap-3">
            <button
              type="button"
              onClick={() => {
                setEditando(true);
                setExito(null);
              }}
              className="rounded-lg bg-brand-600 px-4 py-2 text-sm font-semibold text-white hover:bg-brand-700"
            >
              Editar
            </button>
            <button
              type="button"
              onClick={() => setConfirmarEliminar(true)}
              className="rounded-lg border border-red-300 px-4 py-2 text-sm font-semibold text-red-600 hover:bg-red-50"
            >
              Eliminar
            </button>
          </div>
        )}
      </header>

      {exito && (
        <p role="status" className="rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-800">
          {exito}
        </p>
      )}

      {alerta && (
        <p role="alert" className="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800">
          {alerta}
        </p>
      )}

      {editando ? (
        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-slate-500">
            Editar paciente
          </h2>
          <PacienteForm
            inicial={paciente}
            onCancelar={() => setEditando(false)}
            onSubmit={actualizar}
            guardando={guardando}
          />
        </section>
      ) : (
        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <dl className="grid grid-cols-1 gap-4 text-sm sm:grid-cols-2">
            <div>
              <dt className="text-slate-500">Documento</dt>
              <dd className="font-medium text-slate-900">
                {persona.tipo_documento} {persona.numero_documento}
              </dd>
            </div>
            <div>
              <dt className="text-slate-500">Fecha de nacimiento</dt>
              <dd className="font-medium text-slate-900">{persona.fecha_nacimiento}</dd>
            </div>
            <div>
              <dt className="text-slate-500">Sexo</dt>
              <dd className="font-medium text-slate-900">{persona.sexo}</dd>
            </div>
            <div>
              <dt className="text-slate-500">Teléfono</dt>
              <dd className="font-medium text-slate-900">{persona.telefono}</dd>
            </div>
            <div>
              <dt className="text-slate-500">Email</dt>
              <dd className="font-medium text-slate-900">{persona.email}</dd>
            </div>
            <div>
              <dt className="text-slate-500">Dirección</dt>
              <dd className="font-medium text-slate-900">{persona.direccion}</dd>
            </div>
            <div>
              <dt className="text-slate-500">Grupo sanguíneo</dt>
              <dd className="font-medium text-slate-900">
                {paciente.grupo_sanguineo} {paciente.factor_rh}
              </dd>
            </div>
            <div>
              <dt className="text-slate-500">Contacto de emergencia</dt>
              <dd className="font-medium text-slate-900">
                {paciente.contacto_emergencia_nombre || "—"} · {paciente.contacto_emergencia_telefono || "—"}
              </dd>
            </div>
          </dl>
        </section>
      )}

      {confirmarEliminar && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div className="absolute inset-0 bg-slate-900/40" onClick={() => setConfirmarEliminar(false)} />
          <div className="relative w-full max-w-sm rounded-2xl bg-white p-6 shadow-xl">
            <h2 className="text-lg font-bold text-slate-900">Eliminar paciente</h2>
            <p className="mt-2 text-sm text-slate-600">
              El paciente se desactivará y no podrá agendarse. ¿Deseas continuar?
            </p>
            <div className="mt-5 flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setConfirmarEliminar(false)}
                className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
              >
                Cancelar
              </button>
              <button
                type="button"
                disabled={guardando}
                onClick={eliminar}
                className="rounded-lg bg-red-600 px-4 py-2 text-sm font-semibold text-white hover:bg-red-700 disabled:opacity-60"
              >
                Eliminar
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
