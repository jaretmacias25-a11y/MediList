import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { logout } from "@/services/auth";
import { useAuthStore } from "@/stores/auth";

export function AccountPendingScreen() {
  const navigate = useNavigate();
  const requiereCompletar = useAuthStore(
    (state) => state.user?.requiere_completar_registro,
  );
  const [saliendo, setSaliendo] = useState(false);

  async function cerrarSesion() {
    setSaliendo(true);
    try {
      await logout();
    } finally {
      navigate("/sign-in", { replace: true });
    }
  }

  return (
    <main className="flex h-screen items-center justify-center p-6">
      <div className="max-w-md rounded-xl border border-slate-200 bg-white p-8 text-center shadow-sm">
        <h1 className="text-2xl font-bold text-slate-900">Cuenta pendiente de vinculación</h1>
        <p className="mt-2 text-sm text-slate-500">
          Tu cuenta aún no está vinculada a una historia clínica. Contacta a
          recepción o espera la aprobación de tu solicitud.
        </p>
        <div className="mt-6 flex flex-col gap-2">
          {requiereCompletar && (
            <Link
              to="/completar-registro"
              className="rounded-lg bg-brand-600 px-4 py-2 text-sm font-medium text-white hover:bg-brand-700"
            >
              Completar mis datos
            </Link>
          )}
          <button
            type="button"
            onClick={cerrarSesion}
            disabled={saliendo}
            className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50 disabled:opacity-60"
          >
            {saliendo ? "Cerrando sesión…" : "Cerrar sesión"}
          </button>
          <Link
            to="/"
            className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
          >
            Volver al inicio
          </Link>
        </div>
      </div>
    </main>
  );
}
