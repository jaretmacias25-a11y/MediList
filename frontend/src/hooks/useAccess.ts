import { useAuthStore } from "@/stores/auth";
import type { AccessRule, MeUser, Rol } from "@/types/api";

function canPermission(user: MeUser | null, permission: string): boolean {
  if (!user) return false;
  // El backend trata al rol admin como poseedor de todos los permisos.
  if (user.roles.includes("admin")) return true;
  return user.permisos.includes(permission);
}

function canRoles(user: MeUser | null, roles: Rol[]): boolean {
  if (!user) return false;
  return roles.some((rol) => user.roles.includes(rol));
}

/** Evalúa una regla de acceso contra la sesión actual. */
export function canAccess(user: MeUser | null, rule: AccessRule): boolean {
  const permissions = rule.permissions ?? [];
  const roles = rule.roles ?? [];

  const cumplePermisos =
    permissions.length === 0 ||
    permissions.every((p) => canPermission(user, p));

  const cumpleRoles =
    roles.length === 0 || canRoles(user, roles);

  if (rule.any) {
    if (permissions.length === 0 && roles.length === 0) return true;
    return (permissions.length > 0 && cumplePermisos) || (roles.length > 0 && cumpleRoles);
  }

  return cumplePermisos && cumpleRoles;
}

/** Hook para las guardas de ruta y la navegación condicional. */
export function useAccess() {
  const user = useAuthStore((state) => state.user);

  return {
    user,
    can: (permission: string) => canPermission(user, permission),
    hasRole: (rol: Rol) => canRoles(user, [rol]),
    canAccess: (rule: AccessRule) => canAccess(user, rule),
  };
}
