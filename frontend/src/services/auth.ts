import { http, getData } from "@/lib/http";
import { signOutClerk } from "@/lib/clerk";
import { useAuthStore } from "@/stores/auth";
import type {
  ClerkPendienteResponse,
  Envelope,
  MeUser,
  RegistroClerkInput,
  TokenPair,
} from "@/types/api";

export type ClerkLoginResult = TokenPair | ClerkPendienteResponse;

/**
 * Intercambia un token de sesión de Clerk por una sesión local de MediList.
 * Si la cuenta local no existe, devuelve `registro_pendiente` y NO crea nada.
 */
export async function loginClerk(token: string): Promise<ClerkLoginResult> {
  const resultado = getData(
    await http.post<Envelope<ClerkLoginResult>>("/auth/clerk", { token }),
  );
  if ("registro_pendiente" in resultado) {
    return resultado;
  }
  useAuthStore.getState().setSession(resultado, resultado.usuario);
  return resultado;
}

/**
 * Crea la cuenta local después de que Clerk autenticó al usuario y este
 * completó sus datos clínicos.
 */
export async function registrarClerk(
  input: RegistroClerkInput,
): Promise<TokenPair> {
  const pair = getData(
    await http.post<Envelope<TokenPair>>("/auth/clerk/registro", input),
  );
  useAuthStore.getState().setSession(pair, pair.usuario);
  return pair;
}

/**
 * Restaura la sesión actual. Un 401 dispara automáticamente el refresh en el
 * cliente HTTP; si el refresh también falla, la sesión queda limpia.
 */
export async function fetchMe(): Promise<MeUser> {
  const usuario = getData(await http.get<Envelope<MeUser>>("/auth/me"));
  useAuthStore.getState().setUser(usuario);
  return usuario;
}

export async function logout(): Promise<void> {
  const { refreshToken } = useAuthStore.getState();
  try {
    if (refreshToken) {
      await http.post("/auth/logout", { refresh_token: refreshToken });
    }
  } finally {
    useAuthStore.getState().clearSession();
    await signOutClerk();
  }
}
