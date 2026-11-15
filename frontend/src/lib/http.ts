import axios, {
  AxiosError,
  type AxiosInstance,
  type AxiosResponse,
  type InternalAxiosRequestConfig,
} from "axios";
import { useAuthStore } from "@/stores/auth";
import type { ApiErrorBody, Envelope, TokenPair } from "@/types/api";

const baseURL: string =
  import.meta.env.VITE_API_BASE_URL?.trim() || "/api/v1";

/**
 * Rutas que nunca deben disparar la renovación automática del token.
 */
const AUTH_PUBLIC_PATHS = [
  "/auth/clerk",
  "/auth/refresh",
  "/auth/logout",
];

type RetriableConfig = InternalAxiosRequestConfig & { _retry?: boolean };

/** Error normalizado para toda la aplicación. */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly requestId?: string;
  readonly retryAfter?: number;

  constructor(params: {
    message: string;
    status: number;
    code: string;
    requestId?: string;
    retryAfter?: number;
  }) {
    super(params.message);
    this.name = "ApiError";
    this.status = params.status;
    this.code = params.code;
    this.requestId = params.requestId;
    this.retryAfter = params.retryAfter;
  }

  get isUnauthorized(): boolean {
    return this.status === 401;
  }

  get isForbidden(): boolean {
    return this.status === 403;
  }

  get isConflict(): boolean {
    return this.status === 409;
  }

  get isRateLimited(): boolean {
    return this.status === 429;
  }

  get isFeatureDisabled(): boolean {
    return this.status === 503 && this.code === "feature_disabled";
  }
}

function parseRetryAfter(value: string | undefined): number | undefined {
  if (!value) return undefined;
  const seconds = Number(value);
  return Number.isFinite(seconds) && seconds > 0 ? seconds : undefined;
}

function normalizarError(error: AxiosError<ApiErrorBody>): ApiError {
  if (!error.response) {
    return new ApiError({
      message: "No se pudo conectar con el servidor. Verifica tu conexión.",
      status: 0,
      code: "network_error",
    });
  }

  const { status, headers, data } = error.response;
  const body = data?.error;
  return new ApiError({
    message:
      body?.message ??
      error.message ??
      "Ocurrió un error inesperado.",
    status,
    code: body?.code ?? "unknown_error",
    requestId: data?.request_id ?? headers?.["x-request-id"],
    retryAfter: parseRetryAfter(headers?.["retry-after"]),
  });
}

/**
 * Renovación del access token con una sola promesa compartida:
 * N solicitudes con 401 disparan un único POST /auth/refresh.
 */
let refreshPromise: Promise<string> | null = null;

async function renovarAccessToken(): Promise<string> {
  const { refreshToken } = useAuthStore.getState();

  if (!refreshToken) {
    throw new ApiError({
      message: "Sesión expirada. Inicia sesión nuevamente.",
      status: 401,
      code: "session_expired",
    });
  }

  if (!refreshPromise) {
    refreshPromise = axios
      .post<Envelope<TokenPair>>(`${baseURL}/auth/refresh`, {
        refresh_token: refreshToken,
      })
      .then((respuesta) => {
        const pair = respuesta.data.data;
        useAuthStore.getState().setTokens(pair);
        return pair.access_token;
      })
      .catch((error: AxiosError<ApiErrorBody>) => {
        throw normalizarError(error);
      })
      .finally(() => {
        refreshPromise = null;
      });
  }

  return refreshPromise;
}

export const http: AxiosInstance = axios.create({
  baseURL,
  headers: { "Content-Type": "application/json" },
  timeout: 15000,
});

http.interceptors.request.use((config) => {
  const { accessToken } = useAuthStore.getState();
  if (accessToken && !config.headers.has("Authorization")) {
    config.headers.set("Authorization", `Bearer ${accessToken}`);
  }
  return config;
});

http.interceptors.response.use(
  (respuesta) => respuesta,
  async (error: AxiosError<ApiErrorBody>) => {
    const config = error.config as RetriableConfig | undefined;
    const url = config?.url ?? "";
    const esRutaAuth = AUTH_PUBLIC_PATHS.some((ruta) => url.includes(ruta));

    if (error.response?.status === 401 && config && !esRutaAuth && !config._retry) {
      try {
        const nuevoToken = await renovarAccessToken();
        config._retry = true;
        config.headers.set("Authorization", `Bearer ${nuevoToken}`);
        return http(config);
      } catch (refreshError) {
        useAuthStore.getState().clearSession();
        throw refreshError;
      }
    }

    throw normalizarError(error);
  },
);

/** Extrae `data` de una respuesta con envelope `{ data: ... }`. */
export function getData<T>(respuesta: AxiosResponse<Envelope<T>>): T {
  return respuesta.data.data;
}
