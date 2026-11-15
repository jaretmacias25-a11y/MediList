/**
 * Configuración de Clerk.
 *
 * Clerk es el único proveedor de identidad. Si falta la clave pública, la SPA
 * muestra un aviso de configuración y no permite iniciar sesión.
 * Nunca pongas la clave secreta en el frontend.
 */
export const clerkPublishableKey =
  import.meta.env.VITE_CLERK_PUBLISHABLE_KEY?.trim() || "";

export const clerkEnabled = clerkPublishableKey.length > 0;

export const clerkCallbackPath = "/clerk/callback";

/**
 * Cierra la sesión de Clerk si el SDK del navegador está disponible.
 * Se usa al cerrar sesión en MediList para que /sign-in no vuelva a entrar
 * automáticamente con la sesión anterior.
 */
export async function signOutClerk(): Promise<void> {
  if (!clerkEnabled) return;
  const clerk = (
    window as unknown as {
      Clerk?: { signOut?: () => Promise<void> };
    }
  ).Clerk;
  if (!clerk?.signOut) return;
  try {
    await clerk.signOut();
  } catch {
    // Si Clerk falla, la sesión local ya quedó cerrada.
  }
}
