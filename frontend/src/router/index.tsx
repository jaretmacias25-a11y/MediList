import { createBrowserRouter, Navigate } from "react-router-dom";
import { AccountPendingScreen } from "@/components/AccountPendingScreen";
import { ForbiddenScreen } from "@/components/ForbiddenScreen";
import ClerkSignIn from "@/pages/auth/ClerkSignIn";
import ClerkSignUp from "@/pages/auth/ClerkSignUp";
import ClerkCallback from "@/pages/auth/ClerkCallback";
import PortalPacienteLayout from "@/layouts/PortalPacienteLayout";
import PortalMedicoLayout from "@/layouts/PortalMedicoLayout";
import Perfil from "@/pages/paciente/Perfil";
import InicioPaciente from "@/pages/paciente/InicioPaciente";
import CompletarRegistro from "@/pages/paciente/CompletarRegistro";
import Especialidades from "@/pages/paciente/Especialidades";
import Medicos from "@/pages/paciente/Medicos";
import AgendarCita from "@/pages/paciente/AgendarCita";
import MisCitas from "@/pages/paciente/MisCitas";
import CitaDetalle from "@/pages/paciente/CitaDetalle";
import Agenda from "@/pages/medico/Agenda";
import CitaAtencion from "@/pages/medico/CitaAtencion";
import MisDerivaciones from "@/pages/medico/MisDerivaciones";
import RecepcionLayout from "@/layouts/RecepcionLayout";
import Pacientes from "@/pages/recepcion/Pacientes";
import PacienteDetalle from "@/pages/recepcion/PacienteDetalle";
import CitasGlobales from "@/pages/recepcion/CitasGlobales";
import AgendaGlobal from "@/pages/recepcion/AgendaGlobal";
import Vinculaciones from "@/pages/recepcion/Vinculaciones";
import AdminLayout from "@/layouts/AdminLayout";
import UsuariosRoles from "@/pages/admin/UsuariosRoles";
import AdminEspecialidades from "@/pages/admin/AdminEspecialidades";
import AdminMedicos from "@/pages/admin/AdminMedicos";
import Auditoria from "@/pages/admin/Auditoria";
import ErroresSistema from "@/pages/admin/ErroresSistema";
import { ProtectedRoute } from "./ProtectedRoute";
import { RootRedirect } from "./RootRedirect";

/**
 * Reglas de acceso por portal. Las vistas reales se agregan en iteraciones
 * posteriores; por ahora cada ruta muestra un placeholder protegido.
 */
export const router = createBrowserRouter([
  {
    path: "/",
    element: <RootRedirect />,
  },
  {
    path: "/forbidden",
    element: <ForbiddenScreen />,
  },
  {
    path: "/cuenta-pendiente",
    element: <AccountPendingScreen />,
  },
  {
    path: "/completar-registro",
    element: <CompletarRegistro />,
  },

  // Autenticación (pública)
  { path: "/sign-in/*", element: <ClerkSignIn /> },
  { path: "/sign-up/*", element: <ClerkSignUp /> },
  { path: "/clerk/callback", element: <ClerkCallback /> },
  { path: "/login", element: <Navigate to="/sign-in" replace /> },
  { path: "/registro", element: <Navigate to="/sign-up" replace /> },
  { path: "/verificar-email", element: <Navigate to="/sign-in" replace /> },
  { path: "/recuperar", element: <Navigate to="/sign-in" replace /> },
  { path: "/reset-password", element: <Navigate to="/sign-in" replace /> },

  // Portal del paciente
  {
    path: "/paciente",
    element: (
      <ProtectedRoute
        required={{ permissions: ["portal:perfil:leer"], any: true }}
        requirePacienteVinculado
      >
        <PortalPacienteLayout />
      </ProtectedRoute>
    ),
    children: [
      {
        index: true,
        element: <InicioPaciente />,
      },
      { path: "perfil", element: <Perfil /> },
      { path: "agendar", element: <AgendarCita /> },
      { path: "citas", element: <MisCitas /> },
      { path: "citas/:id", element: <CitaDetalle /> },
      { path: "especialidades", element: <Especialidades /> },
      { path: "medicos", element: <Medicos /> },
      { path: "*", element: <Navigate to="/paciente" replace /> },
    ],
  },

  // Portal del médico
  {
    path: "/medico",
    element: (
      <ProtectedRoute
        required={{ permissions: ["medico:agenda:leer"], any: true }}
        requireMedicoVinculado
      >
        <PortalMedicoLayout />
      </ProtectedRoute>
    ),
    children: [
      {
        index: true,
        element: <Navigate to="/medico/agenda" replace />,
      },
      { path: "agenda", element: <Agenda /> },
      { path: "citas/:id", element: <CitaAtencion /> },
      { path: "derivaciones", element: <MisDerivaciones /> },
      { path: "*", element: <Navigate to="/medico/agenda" replace /> },
    ],
  },

  // Recepción / Enfermería
  {
    path: "/recepcion",
    element: (
      <ProtectedRoute required={{ permissions: ["pacientes:leer"], roles: ["recepcion", "admin", "enfermeria"] }}>
        <RecepcionLayout />
      </ProtectedRoute>
    ),
    children: [
      {
        index: true,
        element: <Navigate to="/recepcion/pacientes" replace />,
      },
      { path: "pacientes", element: <Pacientes /> },
      { path: "pacientes/:id", element: <PacienteDetalle /> },
      { path: "citas", element: <CitasGlobales /> },
      { path: "agenda", element: <AgendaGlobal /> },
      { path: "vinculaciones", element: <Vinculaciones /> },
      { path: "*", element: <Navigate to="/recepcion/pacientes" replace /> },
    ],
  },

  // Enfermería usa el mismo módulo en modo lectura
  {
    path: "/enfermeria/*",
    element: <Navigate to="/recepcion" replace />,
  },

  // Administración
  {
    path: "/admin",
    element: (
      <ProtectedRoute required={{ roles: ["admin"] }}>
        <AdminLayout />
      </ProtectedRoute>
    ),
    children: [
      {
        index: true,
        element: <Navigate to="/admin/usuarios" replace />,
      },
      { path: "usuarios", element: <UsuariosRoles /> },
      { path: "especialidades", element: <AdminEspecialidades /> },
      { path: "medicos", element: <AdminMedicos /> },
      { path: "auditoria", element: <Auditoria /> },
      { path: "errores", element: <ErroresSistema /> },
      { path: "*", element: <Navigate to="/admin/usuarios" replace /> },
    ],
  },

  // Auditoría (solo lectura)
  {
    path: "/auditor",
    element: (
      <ProtectedRoute required={{ permissions: ["auditoria:leer"], roles: ["auditor", "admin"], any: true }}>
        <AdminLayout />
      </ProtectedRoute>
    ),
    children: [
      {
        index: true,
        element: <Navigate to="/auditor/auditoria" replace />,
      },
      { path: "usuarios", element: <UsuariosRoles /> },
      { path: "auditoria", element: <Auditoria /> },
      { path: "errores", element: <ErroresSistema /> },
      { path: "*", element: <Navigate to="/auditor/auditoria" replace /> },
    ],
  },

  {
    path: "*",
    element: <Navigate to="/" replace />,
  },
]);
