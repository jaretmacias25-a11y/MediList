import { create } from "zustand";
import type { MeUser, TokenPair } from "@/types/api";

export type AuthStatus =
  | "checking"
  | "authenticated"
  | "unauthenticated";

interface AuthState {
  accessToken: string | null;
  refreshToken: string | null;
  user: MeUser | null;
  status: AuthStatus;

  setTokens: (pair: Pick<TokenPair, "access_token" | "refresh_token">) => void;
  setSession: (
    pair: Pick<TokenPair, "access_token" | "refresh_token">,
    user: MeUser,
  ) => void;
  setUser: (user: MeUser) => void;
  setStatus: (status: AuthStatus) => void;
  clearSession: () => void;
}

/**
 * Los tokens viven SOLO en memoria. No se persisten en localStorage ni
 * sessionStorage: al recargar la página hay que iniciar sesión de nuevo.
 * Esto evita robo de tokens por XSS en un sistema con datos clínicos.
 */
export const useAuthStore = create<AuthState>()((set) => ({
  accessToken: null,
  refreshToken: null,
  user: null,
  status: "checking",

  setTokens: (pair) =>
    set({
      accessToken: pair.access_token,
      refreshToken: pair.refresh_token,
    }),

  setSession: (pair, user) =>
    set({
      accessToken: pair.access_token,
      refreshToken: pair.refresh_token,
      user,
      status: "authenticated",
    }),

  setUser: (user) => set({ user, status: "authenticated" }),

  setStatus: (status) => set({ status }),

  clearSession: () =>
    set({
      accessToken: null,
      refreshToken: null,
      user: null,
      status: "unauthenticated",
    }),
}));
