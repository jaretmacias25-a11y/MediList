import type { ReactNode } from "react";
import { Navigate, useLocation } from "react-router-dom";
import { useAccess } from "@/hooks/useAccess";
import { useAuthStore } from "@/stores/auth";
import type { AccessRule } from "@/types/api";

interface ProtectedRouteProps {
  children: ReactNode;
  required?: AccessRule;
  /** Portal paciente: exige cuenta con paciente_id vinculado. */
  requirePacienteVinculado?: boolean;
  /** Portal médico: exige cuenta con medico_id vinculado. */
  requireMedicoVinculado?: boolean;
}

export function ProtectedRoute({
  children,
  required,
  requirePacienteVinculado = false,
  requireMedicoVinculado = false,
}: ProtectedRouteProps) {
  const status = useAuthStore((state) => state.status);
  const location = useLocation();
  const { user, canAccess } = useAccess();

  if (status === "checking") {
    return (
      <div className="flex h-screen items-center justify-center bg-slate-50">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
      </div>
    );
  }

  if (status === "unauthenticated") {
    return <Navigate to="/sign-in" replace state={{ from: location.pathname }} />;
  }

  if (required && !canAccess(required)) {
    return (
      <Navigate to="/forbidden" replace state={{ from: location.pathname }} />
    );
  }

  if (requirePacienteVinculado && (!user || !user.paciente_id)) {
    if (user?.requiere_completar_registro) {
      return (
        <Navigate to="/completar-registro" replace state={{ from: location.pathname }} />
      );
    }
    return (
      <Navigate to="/cuenta-pendiente" replace state={{ from: location.pathname }} />
    );
  }

  if (requireMedicoVinculado && (!user || !user.medico_id)) {
    return (
      <Navigate to="/cuenta-pendiente" replace state={{ from: location.pathname }} />
    );
  }

  return <>{children}</>;
}
