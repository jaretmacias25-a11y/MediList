import { http, getData } from "@/lib/http";
import type { Envelope } from "@/types/api";

export interface PersonaPerfil {
  id: number;
  tipo_documento: string;
  numero_documento: string;
  nombres: string;
  apellidos: string;
  fecha_nacimiento: string;
  sexo: string;
  telefono: string;
  email: string;
  direccion: string;
}

export interface PacientePerfil {
  id: number;
  usuario_id?: number | null;
  medico_responsable_id?: number | null;
  persona: PersonaPerfil;
  numero_historia: string;
  grupo_sanguineo: string;
  factor_rh: string;
  contacto_emergencia_nombre: string;
  contacto_emergencia_telefono: string;
  activo: boolean;
  version: number;
}

export interface CuentaPerfil {
  usuario_id: number;
  username: string;
  email: string;
  email_verificado: boolean;
  paciente_id?: number | null;
  medico_id?: number | null;
}

export interface PerfilPaciente {
  paciente: PacientePerfil;
  cuenta: CuentaPerfil;
}

export interface PerfilUpdateInput {
  telefono: string;
  direccion: string;
  contacto_emergencia_nombre: string;
  contacto_emergencia_telefono: string;
  version: number;
}

export interface CompletarRegistroInput {
  nombres: string;
  apellidos: string;
  tipo_documento: string;
  numero_documento: string;
  fecha_nacimiento: string;
  sexo: string;
  telefono: string;
  direccion: string;
}

export interface CompletarRegistroResponse {
  usuario_id: number;
  paciente_id?: number | null;
  solicitud_id?: number | null;
  email: string;
  email_verificado: boolean;
  vinculacion_pendiente: boolean;
  mensaje: string;
}

export async function completarRegistroPaciente(
  input: CompletarRegistroInput,
): Promise<CompletarRegistroResponse> {
  return getData(
    await http.post<Envelope<CompletarRegistroResponse>>(
      "/portal/cuenta/completar-registro",
      input,
    ),
  );
}

export async function getPerfil(): Promise<PerfilPaciente> {
  return getData(await http.get<Envelope<PerfilPaciente>>("/portal/perfil"));
}

export async function updatePerfil(
  input: PerfilUpdateInput,
): Promise<PerfilPaciente> {
  return getData(
    await http.put<Envelope<PerfilPaciente>>("/portal/perfil", input),
  );
}
