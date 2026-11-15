import { useEffect, useState } from "react";
import { ApiError } from "@/lib/http";
import {
  actualizarUsuario,
  desbloquearUsuario,
  listarRoles,
  listarUsuarios,
  revocarSesionesUsuario,
} from "@/services/admin";
import { useAccess } from "@/hooks/useAccess";
import { useAuthStore } from "@/stores/auth";

const inputClass =
  "w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 shadow-sm outline-none transition focus:border-brand-500 focus:ring-2 focus:ring-brand-100";

const LIMITE = 10;

export default function UsuariosRoles() {
  const { can } = useAccess();
  const puedeEscribir = can("usuarios:escribir");
  const usuarioActual = useAuthStore((state) => state.user);

  const [usuarios, setUsuarios] = useState([]);
  const [roles, setRoles] = useState([]);
  const [meta, setMeta] = useState({ count: 0, limit: LIMITE, offset: 0 });
  const [search, setSearch] = useState("");
  const [offset, setOffset] = useState(0);
  const [cargando, setCargando] = useState(true);
  const [error, setError] = useState(null);
  const [exito, setExito] = useState(null);

  const [editando, setEditando] = useState(null);
  const [form, setForm] = useState(null);
  const [guardando, setGuardando] = useState(false);
  const [revocando, setRevocando] = useState(null);

  async function cargar(busqueda, nuevoOffset) {
    setCargando(true);
    setError(null);
    try {
      const [resultado, rolesResp] = await Promise.all([
        listarUsuarios(busqueda, LIMITE, nuevoOffset),
        listarRoles(),
      ]);
      setUsuarios(resultado.items);
      setMeta(resultado.meta);
      setRoles(rolesResp);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "No se pudieron cargar los usuarios.");
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

  function abrirEdicion(usuario) {
    setEditando(usuario);
    setForm({
      email: usuario.email,
      nombres: usuario.nombres,
      apellidos: usuario.apellidos,
      activo: usuario.activo,
      roles: [...usuario.roles],
    });
  }

  function esUnoMismo(usuario) {
    return usuarioActual && usuario.id === usuarioActual.id;
  }

  async function guardar() {
    if (!editando || !form) return;

    // Regla de negocio: un admin no puede quitarse su propio rol de admin
    // ni desactivar su propia cuenta.
    if (esUnoMismo(editando)) {
      if (!form.activo) {
        setError("No puedes desactivar tu propia cuenta.");
        return;
      }
      if (!form.roles.includes("admin")) {
        setError("No puedes quitarte tu propio rol de administrador.");
        return;
      }
    }

    setGuardando(true);
    setError(null);
    try {
      const actualizado = await actualizarUsuario(editando.id, {
        email: form.email.trim(),
        nombres: form.nombres.trim(),
        apellidos: form.apellidos.trim(),
        activo: form.activo,
        roles: form.roles,
        version: editando.version,
      });
      setUsuarios((previos) =>
        previos.map((u) => (u.id === actualizado.id ? actualizado : u)),
      );
      setEditando(null);
      setExito("Usuario actualizado.");
    } catch (err) {
      if (err instanceof ApiError && err.isConflict) {
        setError("El usuario cambió en otra sesión. Se recargaron los datos.");
        setEditando(null);
        await cargar(search, offset);
      } else {
        setError(err instanceof ApiError ? err.message : "No se pudo actualizar el usuario.");
      }
    } finally {
      setGuardando(false);
    }
  }

  async function accionRapida(usuario, tipo) {
    setError(null);
    setExito(null);
    try {
      if (tipo === "desbloquear") {
        await desbloquearUsuario(usuario.id);
        setExito("Cuenta desbloqueada.");
      } else {
        await revocarSesionesUsuario(usuario.id);
        setExito("Sesiones revocadas.");
      }
      await cargar(search, offset);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "No se pudo completar la acción.");
    }
  }

  const total = meta?.count ?? usuarios.length;
  const paginas = Math.max(1, Math.ceil(total / LIMITE));

  return (
    <div className="mx-auto max-w-5xl space-y-6">
      <header>
        <h1 className="text-xl font-bold text-slate-900">Usuarios y roles</h1>
        <p className="mt-1 text-sm text-slate-500">
          Cuentas del sistema, roles asignados y acciones de seguridad.
        </p>
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

      <div>
        <label htmlFor="busqueda" className="mb-1 block text-sm font-medium text-slate-700">
          Buscar
        </label>
        <input
          id="busqueda"
          type="search"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Usuario, nombre o correo"
          className={inputClass}
        />
      </div>

      {cargando ? (
        <div className="flex justify-center py-8">
          <div className="h-6 w-6 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
        </div>
      ) : usuarios.length === 0 ? (
        <p className="rounded-lg border border-slate-200 bg-white px-4 py-3 text-sm text-slate-500">
          No se encontraron usuarios.
        </p>
      ) : (
        <div className="overflow-x-auto rounded-2xl border border-slate-200 bg-white shadow-sm">
          <table className="w-full text-left text-sm">
            <thead className="bg-slate-50 text-xs uppercase text-slate-500">
              <tr>
                <th className="px-4 py-3">Usuario</th>
                <th className="px-4 py-3">Roles</th>
                <th className="px-4 py-3">Estado</th>
                {puedeEscribir && <th className="px-4 py-3 text-right">Acciones</th>}
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {usuarios.map((usuario) => (
                <tr key={usuario.id}>
                  <td className="px-4 py-3">
                    <p className="font-medium text-slate-900">{usuario.username}</p>
                    <p className="text-xs text-slate-500">
                      {usuario.nombres} {usuario.apellidos} · {usuario.email}
                    </p>
                    {usuario.bloqueado_hasta && (
                      <p className="text-xs text-red-600">Bloqueado hasta {new Date(usuario.bloqueado_hasta).toLocaleString("es-EC")}</p>
                    )}
                  </td>
                  <td className="px-4 py-3 text-slate-600">{usuario.roles.join(", ")}</td>
                  <td className="px-4 py-3">
                    <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${
                      usuario.activo ? "bg-emerald-100 text-emerald-800" : "bg-slate-200 text-slate-600"
                    }`}>
                      {usuario.activo ? "Activo" : "Inactivo"}
                    </span>
                  </td>
                  {puedeEscribir && (
                    <td className="px-4 py-3">
                      <div className="flex justify-end gap-2">
                        <button
                          type="button"
                          onClick={() => abrirEdicion(usuario)}
                          className="rounded-lg border border-slate-300 px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
                        >
                          Editar
                        </button>
                        {usuario.bloqueado_hasta && (
                          <button
                            type="button"
                            onClick={() => accionRapida(usuario, "desbloquear")}
                            className="rounded-lg border border-slate-300 px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
                          >
                            Desbloquear
                          </button>
                        )}
                        <button
                          type="button"
                          onClick={() => setRevocando(usuario)}
                          className="rounded-lg border border-red-300 px-3 py-1.5 text-sm font-medium text-red-600 hover:bg-red-50"
                        >
                          Revocar Sesiones
                        </button>
                      </div>
                    </td>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
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

      {editando && form && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div className="absolute inset-0 bg-slate-900/40" onClick={() => setEditando(null)} />
          <div className="relative w-full max-w-md rounded-2xl bg-white p-6 shadow-xl">
            <h2 className="text-lg font-bold text-slate-900">
              Editar usuario {editando.username}
            </h2>
            {esUnoMismo(editando) && (
              <p className="mt-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800">
                Es tu propia cuenta: no puedes desactivarla ni quitarte el rol
                de administrador.
              </p>
            )}
            <div className="mt-4 grid grid-cols-1 gap-4">
              <div>
                <label className="mb-1 block text-sm font-medium text-slate-700">Email</label>
                <input value={form.email} onChange={(e) => setForm((f) => ({ ...f, email: e.target.value }))} className={inputClass} />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="mb-1 block text-sm font-medium text-slate-700">Nombres</label>
                  <input value={form.nombres} onChange={(e) => setForm((f) => ({ ...f, nombres: e.target.value }))} className={inputClass} />
                </div>
                <div>
                  <label className="mb-1 block text-sm font-medium text-slate-700">Apellidos</label>
                  <input value={form.apellidos} onChange={(e) => setForm((f) => ({ ...f, apellidos: e.target.value }))} className={inputClass} />
                </div>
              </div>
              <div>
                <span className="mb-1 block text-sm font-medium text-slate-700">Roles</span>
                <div className="grid grid-cols-2 gap-2">
                  {roles.map((rol) => {
                    const marcado = form.roles.includes(rol.codigo);
                    const bloqueado = esUnoMismo(editando) && rol.codigo === "admin";
                    return (
                      <label key={rol.id} className="flex items-center gap-2 text-sm text-slate-700">
                        <input
                          type="checkbox"
                          checked={marcado}
                          disabled={bloqueado}
                          onChange={(e) =>
                            setForm((f) => ({
                              ...f,
                              roles: e.target.checked
                                ? [...f.roles, rol.codigo]
                                : f.roles.filter((r) => r !== rol.codigo),
                            }))
                          }
                          className="h-4 w-4 rounded border-slate-300 text-brand-600 focus:ring-brand-500 disabled:opacity-50"
                        />
                        {rol.nombre}
                      </label>
                    );
                  })}
                </div>
              </div>
              <label className="flex items-center gap-2 text-sm text-slate-700">
                <input
                  type="checkbox"
                  checked={form.activo}
                  disabled={esUnoMismo(editando)}
                  onChange={(e) => setForm((f) => ({ ...f, activo: e.target.checked }))}
                  className="h-4 w-4 rounded border-slate-300 text-brand-600 focus:ring-brand-500 disabled:opacity-50"
                />
                Cuenta activa
              </label>
            </div>
            <div className="mt-5 flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setEditando(null)}
                className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
              >
                Cancelar
              </button>
              <button
                type="button"
                disabled={guardando}
                onClick={guardar}
                className="rounded-lg bg-brand-600 px-4 py-2 text-sm font-semibold text-white hover:bg-brand-700 disabled:opacity-60"
              >
                {guardando ? "Guardando…" : "Guardar"}
              </button>
            </div>
          </div>
        </div>
      )}

      {revocando && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div className="absolute inset-0 bg-slate-900/40" onClick={() => setRevocando(null)} />
          <div className="relative w-full max-w-sm rounded-2xl bg-white p-6 shadow-xl">
            <h2 className="text-lg font-bold text-slate-900">Revocar sesiones</h2>
            <p className="mt-2 text-sm text-slate-600">
              Todas las sesiones de <strong>{revocando.username}</strong> se
              cerrarán de inmediato. ¿Deseas continuar?
            </p>
            <div className="mt-5 flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setRevocando(null)}
                className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
              >
                Cancelar
              </button>
              <button
                type="button"
                onClick={() => {
                  const usuario = revocando;
                  setRevocando(null);
                  accionRapida(usuario, "revocar");
                }}
                className="rounded-lg bg-red-600 px-4 py-2 text-sm font-semibold text-white hover:bg-red-700"
              >
                Revocar
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
