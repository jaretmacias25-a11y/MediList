import { Navigate } from "react-router-dom";
import { usePortalDestino } from "./usePortalDestino";

export function RootRedirect() {
  const destino = usePortalDestino();
  return <Navigate to={destino} replace />;
}
