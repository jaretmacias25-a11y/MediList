import { Link } from "react-router-dom";

export function ForbiddenScreen() {
  return (
    <main className="flex h-screen items-center justify-center p-6">
      <div className="max-w-md rounded-xl border border-slate-200 bg-white p-8 text-center shadow-sm">
        <h1 className="text-2xl font-bold text-slate-900">403 — Acceso denegado</h1>
        <p className="mt-2 text-sm text-slate-500">
          Tu cuenta no tiene permisos para ver esta sección.
        </p>
        <Link
          to="/"
          className="mt-6 inline-block rounded-lg bg-brand-600 px-4 py-2 text-sm font-medium text-white hover:bg-brand-700"
        >
          Volver al inicio
        </Link>
      </div>
    </main>
  );
}
