import { useEffect } from "react";
import { RouterProvider } from "react-router-dom";
import { router } from "@/router";
import { fetchMe } from "@/services/auth";
import { useAuthStore } from "@/stores/auth";

/**
 * Resuelve la sesión una sola vez al montar la aplicación.
 * - Sin access token: estado `unauthenticated` (ir al login).
 * - Con access token: GET /auth/me; si el token expiró, el cliente HTTP
 *   ejecuta el refresh automático y reintenta. Si el refresh falla, la
 *   sesión queda limpia.
 */
export default function App() {
  useEffect(() => {
    const { accessToken, status, setStatus } = useAuthStore.getState();

    if (!accessToken) {
      setStatus("unauthenticated");
      return;
    }

    if (status !== "checking") return;

    fetchMe()
      .then(() => {
        if (useAuthStore.getState().status === "checking") {
          setStatus("authenticated");
        }
      })
      .catch(() => {
        // Si todavía hay access token, se conserva la sesión en memoria y la
        // UI podrá reintentar; si el refresh falló, el interceptor ya limpió.
        if (useAuthStore.getState().accessToken) {
          setStatus("authenticated");
        } else {
          setStatus("unauthenticated");
        }
      });
  }, []);

  return <RouterProvider router={router} />;
}
