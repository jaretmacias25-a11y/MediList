import { useAuthStore } from "@/stores/auth";
import type { MeUser } from "@/types/api";

const ORDEN_POR_ROL: Array<{ rol: string; destino: string }> = [
  { rol: "admin", destino: "/admin" },
  { rol: "recepcion", destino: "/recepcion" },
  { rol: "enfermeria", destino: "/enfermeria" },
  { rol: "medico", destino: "/medico" },
  { rol: "auditor", destino: "/auditor" },
  { rol: "paciente", destino: "/paciente" },
];

/**
 * Devuelve el portal de destino según el rol de la sesión actual.
 * La resolución de la sesión ocurre en <App> con GET /auth/me.
 */
export function portalDestinoPara(user: MeUser | null): string {
  if (!user) return "/sign-in";
  if (user.requiere_completar_registro) return "/completar-registro";

  for (const entrada of ORDEN_POR_ROL) {
    if (user.roles.includes(entrada.rol)) return entrada.destino;
  }
  return "/sign-in";
}

export function usePortalDestino(): string {
  const user = useAuthStore((state) => state.user);
  return portalDestinoPara(user);
}

