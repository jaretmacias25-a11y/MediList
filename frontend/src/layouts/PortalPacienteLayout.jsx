import { useState } from "react";
import { NavLink, Outlet, useNavigate } from "react-router-dom";
import { logout } from "@/services/auth";
import { useAuthStore } from "@/stores/auth";

const ENLACES = [
  { to: "/paciente", label: "Inicio", end: true },
  { to: "/paciente/agendar", label: "Agendar Cita", end: false },
  { to: "/paciente/perfil", label: "Mi Perfil", end: false },
  { to: "/paciente/citas", label: "Mis Citas", end: false },
  { to: "/paciente/especialidades", label: "Especialidades", end: false },
  { to: "/paciente/medicos", label: "Médicos", end: false },
];

function claseEnlace({ isActive }) {
  return [
    "flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition",
    isActive
      ? "bg-brand-600 text-white"
      : "text-slate-700 hover:bg-brand-50 hover:text-brand-700",
  ].join(" ");
}

export default function PortalPacienteLayout() {
  const navigate = useNavigate();
  const user = useAuthStore((state) => state.user);
  const [menuAbierto, setMenuAbierto] = useState(false);

  async function handleLogout() {
    await logout();
    navigate("/sign-in", { replace: true });
  }

  const sidebar = (
    <nav className="flex h-full flex-col gap-1 p-4">
      <div className="mb-4 px-3">
        <p className="text-lg font-bold text-brand-700">MediList</p>
        <p className="truncate text-xs text-slate-500">
          {user?.nombres} {user?.apellidos}
        </p>
      </div>

      {ENLACES.map((enlace) => (
        <NavLink
          key={enlace.to}
          to={enlace.to}
          end={enlace.end}
          className={claseEnlace}
          onClick={() => setMenuAbierto(false)}
        >
          {enlace.label}
        </NavLink>
      ))}

      <button
        type="button"
        onClick={handleLogout}
        className="mt-auto flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium text-red-600 transition hover:bg-red-50"
      >
        Cerrar sesión
      </button>
    </nav>
  );

  return (
    <div className="min-h-screen bg-slate-50">
      {/* Barra superior */}
      <header className="sticky top-0 z-30 flex h-14 items-center gap-3 border-b border-slate-200 bg-white px-4 shadow-sm">
        <button
          type="button"
          aria-label="Abrir menú"
          onClick={() => setMenuAbierto((previo) => !previo)}
          className="rounded-lg border border-slate-200 p-2 text-slate-600 hover:bg-slate-100 lg:hidden"
        >
          <svg className="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <path strokeLinecap="round" d="M4 6h16M4 12h16M4 18h16" />
          </svg>
        </button>

        <span className="text-sm font-semibold text-slate-900">
          Portal del paciente
        </span>

        <div className="ml-auto hidden items-center gap-2 sm:flex">
          <span className="text-sm text-slate-600">{user?.username}</span>
          <button
            type="button"
            onClick={handleLogout}
            className="rounded-lg px-3 py-1.5 text-sm font-medium text-red-600 hover:bg-red-50"
          >
            Cerrar sesión
          </button>
        </div>
      </header>

      <div className="flex">
        {/* Sidebar de escritorio */}
        <aside className="sticky top-14 hidden h-[calc(100vh-3.5rem)] w-64 shrink-0 border-r border-slate-200 bg-white lg:block">
          {sidebar}
        </aside>

        {/* Sidebar móvil */}
        {menuAbierto && (
          <div className="fixed inset-0 z-40 lg:hidden">
            <div
              className="absolute inset-0 bg-slate-900/40"
              onClick={() => setMenuAbierto(false)}
            />
            <aside className="absolute left-0 top-0 h-full w-72 bg-white shadow-xl">
              {sidebar}
            </aside>
          </div>
        )}

        <main className="min-w-0 flex-1 p-4 sm:p-6">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
