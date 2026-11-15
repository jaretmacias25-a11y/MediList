import { SignIn } from "@clerk/react";
import { clerkCallbackPath, clerkEnabled } from "@/lib/clerk";

export default function ClerkSignIn() {
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
            backend. MediList solo permite iniciar sesión con Clerk.
          </p>
        </div>
      </main>
    );
  }

  return (
    <main className="flex min-h-screen items-center justify-center bg-slate-50 p-4">
      <div className="w-full max-w-md">
        <div className="mb-6 text-center">
          <h1 className="text-2xl font-bold text-slate-900">MediList</h1>
          <p className="mt-1 text-sm text-slate-500">
            Accede con tu cuenta de Clerk
          </p>
        </div>

        <SignIn
          routing="virtual"
          fallbackRedirectUrl={clerkCallbackPath}
          signUpUrl="/sign-up"
        />
      </div>
    </main>
  );
}
