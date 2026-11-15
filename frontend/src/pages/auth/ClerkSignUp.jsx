import { SignUp } from "@clerk/react";
import { Link, useSearchParams } from "react-router-dom";
import { clerkCallbackPath, clerkEnabled } from "@/lib/clerk";

export default function ClerkSignUp() {
  const [searchParams] = useSearchParams();
  const email = searchParams.get("email")?.trim() || "";

  if (!clerkEnabled) {
    return (
      <main className="flex min-h-screen items-center justify-center bg-slate-50 p-4">
        <div className="w-full max-w-md rounded-2xl border border-amber-200 bg-amber-50 p-8 text-center shadow-sm">
          <h1 className="text-xl font-bold text-slate-900">
            Clerk no está configurado
          </h1>
          <p className="mt-2 text-sm text-slate-600">
            Define{" "}
            <code className="rounded bg-white px-1">VITE_CLERK_PUBLISHABLE_KEY</code>{" "}
            en el frontend y{" "}
            <code className="rounded bg-white px-1">CLERK_SECRET_KEY</code> en el
            backend. MediList solo permite registrarse con Clerk.
          </p>
        </div>
      </main>
    );
  }

  return (
    <main className="flex min-h-screen items-center justify-center bg-slate-50 p-4">
      <div className="w-full max-w-md">
        <div className="mb-6 text-center">
          <h1 className="text-2xl font-bold text-slate-900">Crea tu cuenta</h1>
          <p className="mt-1 text-sm text-slate-500">
            Clerk verifica tu correo y gestiona la recuperación de contraseña.
          </p>
        </div>

        <SignUp
          routing="virtual"
          fallbackRedirectUrl={clerkCallbackPath}
          signInUrl="/sign-in"
          initialValues={email ? { emailAddress: email } : undefined}
        />

        <p className="mt-6 text-center text-sm text-slate-500">
          ¿Ya tienes cuenta?{" "}
          <Link
            to="/sign-in"
            className="font-medium text-brand-600 hover:text-brand-700"
          >
            Inicia sesión
          </Link>
        </p>
      </div>
    </main>
  );
}
