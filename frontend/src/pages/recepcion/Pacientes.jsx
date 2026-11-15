import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { ApiError } from "@/lib/http";
import { crearPaciente, listarPacientes } from "@/services/recepcion";
import { useAccess } from "@/hooks/useAccess";
import PacienteForm from "./PacienteForm";

const LIMITE = 10;

export default function Pacientes() {
  const { can } = useAccess();
  const puedeEscribir = can("pacientes:escribir");

  const [items, setItems] = useState([]);
  const [meta, setMeta] = useState({ count: 0, limit: LIMITE, offset: 0 });
  const [search, setSearch] = useState("");
  const [offset, setOffset] = useState(0);
  const [cargando, setCargando] = useState(true);
  const [error, setError] = useState(null);

  const [mostrarNuevo, setMostrarNuevo] = useState(false);
  const [guardando, setGuardando] = useState(false);

  async function cargar(busqueda, nuevoOffset) {
    setCargando(true);
    setError(null);
    try {
      const resultado = await listarPacientes(busqueda, LIMITE, nuevoOffset);
      setItems(resultado.items);
      setMeta(resultado.meta);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "No se pudieron cargar los pacientes.");
    } finally {
      setCargando(false);
    }
  }

  useEffect(() => {
    const timer = setTimeout(() => {
      setOffset(0);
      cargar(search, 0);
    }, 300);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [search]);

  useEffect(() => {
    cargar(search, offset);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [offset]);

  async function crear(input) {
    setGuardando(true);
    setError(null);
    try {
      await crearPaciente(input);
      setMostrarNuevo(false);
      await cargar(search, 0);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "No se pudo crear el paciente.");
    } finally {
      setGuardando(false);
    }
  }

  const total = meta?.count ?? items.length;
  const paginas = Math.max(1, Math.ceil(total / LIMITE));

  return (
    <div className="mx-auto max-w-4xl space-y-6">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-bold text-slate-900">Pacientes</h1>
          <p className="mt-1 text-sm text-slate-500">
            Historias clínicas registradas en la clínica.
          </p>
        </div>
        {puedeEscribir && (
          <button
            type="button"
            onClick={() => setMostrarNuevo(true)}
            className="rounded-lg bg-brand-600 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-brand-700"
          >
            Nuevo Paciente
          </button>
        )}
      </header>

      {error && (
        <p role="alert" className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
          {error}
        </p>
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
          placeholder="Nombre, documento, historia o correo"
          className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm outline-none focus:border-brand-500 focus:ring-2 focus:ring-brand-100"
        />
      </div>

      {cargando ? (
        <div className="flex justify-center py-8">
          <div className="h-6 w-6 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
        </div>
      ) : items.length === 0 ? (
        <p className="rounded-lg border border-slate-200 bg-white px-4 py-3 text-sm text-slate-500">
          No se encontraron pacientes.
        </p>
      ) : (
        <div className="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
          <ul className="divide-y divide-slate-100">
            {items.map((paciente) => (
              <li key={paciente.id}>
                <Link
                  to={`/recepcion/pacientes/${paciente.id}`}
                  className="flex items-center justify-between gap-3 px-4 py-3 transition hover:bg-brand-50"
                >
                  <div className="min-w-0">
                    <p className="font-medium text-slate-900">
                      {paciente.persona.nombres} {paciente.persona.apellidos}
                    </p>
                    <p className="text-sm text-slate-500">
                      {paciente.persona.tipo_documento} {paciente.persona.numero_documento} · {paciente.numero_historia}
                    </p>
                  </div>
                  <span
                    className={`shrink-0 rounded-full px-2 py-0.5 text-xs font-medium ${
                      paciente.activo
                        ? "bg-emerald-100 text-emerald-800"
                        : "bg-slate-200 text-slate-600"
                    }`}
                  >
                    {paciente.activo ? "Activo" : "Inactivo"}
                  </span>
                </Link>
              </li>
            ))}
          </ul>
        </div>
      )}

      <div className="flex items-center justify-between text-sm text-slate-600">
        <button
          type="button"
          disabled={offset === 0 || cargando}
          onClick={() => setOffset(Math.max(0, offset - LIMITE))}
          className="rounded-lg border border-slate-300 px-3 py-1.5 disabled:opacity-50"
        >
          Anterior
        </button>
        <span>
          Página {Math.floor(offset / LIMITE) + 1} de {paginas} · {total} registros
        </span>
        <button
          type="button"
          disabled={offset + LIMITE >= total || cargando}
          onClick={() => setOffset(offset + LIMITE)}
          className="rounded-lg border border-slate-300 px-3 py-1.5 disabled:opacity-50"
        >
          Siguiente
        </button>
      </div>

      {mostrarNuevo && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div className="absolute inset-0 bg-slate-900/40" onClick={() => setMostrarNuevo(false)} />
          <div className="relative max-h-[90vh] w-full max-w-2xl overflow-y-auto rounded-2xl bg-white p-6 shadow-xl">
            <h2 className="mb-4 text-lg font-bold text-slate-900">Nuevo paciente</h2>
            <PacienteForm
              onCancelar={() => setMostrarNuevo(false)}
              onSubmit={crear}
              guardando={guardando}
            />
          </div>
        </div>
      )}
    </div>
  );
}
