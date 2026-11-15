import { useEffect, useRef, useState } from "react";
import { useAuth, useClerk } from "@clerk/react";
import { Navigate, useNavigate } from "react-router-dom";
import { ApiError } from "@/lib/http";
import { clerkEnabled } from "@/lib/clerk";
import { fetchMe, loginClerk } from "@/services/auth";
import { useAuthStore } from "@/stores/auth";
import { portalDestinoPara } from "@/router/usePortalDestino";

function mensajeDeError(error) {
  if (error instanceof ApiError) {
    if (error.status === 403) {
      return (
        error.message ||
        "El backend no tiene configurado Clerk. Agrega CLERK_SECRET_KEY y reinicia la API."
      );
    }
    if (error.status === 401) {
      return "Clerk rechazó la sesión. Vuelve a iniciar sesión.";
    }
    return error.message;
  }
  return error instanceof Error
    ? error.message
    : "No se pudo completar el inicio de sesión.";
}

function ClerkCallbackInner() {
  const navigate = useNavigate();
  const { isLoaded, isSignedIn, getToken } = useAuth();
  const { signOut } = useClerk();
  const [error, setError] = useState(null);
  const [saliendo, setSaliendo] = useState(false);
  const iniciado = useRef(false);

  async function volverAlLogin() {
    setSaliendo(true);
    try {
      await signOut();
    } catch {
      // Aunque Clerk falle, limpiamos la sesión local y volvemos al login.
    }
    useAuthStore.getState().clearSession();
    navigate("/sign-in", { replace: true });
  }

  useEffect(() => {
    if (!isLoaded) return;

    if (!isSignedIn) {
      navigate("/sign-in", { replace: true });
      return;
    }

    if (iniciado.current) return;
    iniciado.current = true;

    (async () => {
      const token = await getToken();
      if (!token) {
        throw new Error("Clerk no devolvió un token de sesión.");
      }

      const resultado = await loginClerk(token);
      if ("registro_pendiente" in resultado) {
        navigate("/completar-registro", { replace: true });
        return;
      }

      // El token de Clerk no incluye paciente_id/medico_id; /auth/me los
      // resuelve para elegir el portal correcto.
      let me = null;
      try {
        me = await fetchMe();
      } catch {
        // Si /auth/me falla, se usa el usuario devuelto por /auth/clerk.
      }

      const { user } = useAuthStore.getState();
      const destino =
        me?.requiere_completar_registro
          ? "/completar-registro"
          : portalDestinoPara(me ?? user);
      navigate(destino, { replace: true });
    })().catch((err) => {
      useAuthStore.getState().clearSession();
      setError(mensajeDeError(err));
      iniciado.current = false;
    });
  }, [getToken, isLoaded, isSignedIn, navigate]);

  if (error) {
    return (
      <main className="flex min-h-screen items-center justify-center bg-slate-50 p-4">
        <div className="w-full max-w-md rounded-2xl border border-red-200 bg-white p-8 text-center shadow-sm">
          <h1 className="text-xl font-bold text-slate-900">
            No se pudo iniciar sesión
          </h1>
          <p role="alert" className="mt-3 text-sm text-red-700">
            {error}
          </p>
          <div className="mt-6 flex flex-col gap-2">
            <button
              type="button"
              onClick={volverAlLogin}
              disabled={saliendo}
              className="rounded-lg bg-brand-600 px-4 py-2 text-sm font-medium text-white hover:bg-brand-700 disabled:opacity-60"
            >
              {saliendo ? "Cerrando sesión…" : "Cerrar sesión de Clerk y volver"}
            </button>
          </div>
        </div>
      </main>
    );
  }

  return (
    <main className="flex min-h-screen items-center justify-center bg-slate-50">
      <div className="text-center">
        <div className="mx-auto h-8 w-8 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
        <p className="mt-4 text-sm text-slate-500">
          Preparando tu sesión en MediList…
        </p>
      </div>
    </main>
  );
}

export default function ClerkCallback() {
  if (!clerkEnabled) {
    return <Navigate to="/sign-in" replace />;
  }
  return <ClerkCallbackInner />;
}
