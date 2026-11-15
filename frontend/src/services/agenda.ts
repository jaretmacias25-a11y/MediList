import { http, getData } from "@/lib/http";
import type { Envelope } from "@/types/api";

export interface EspecialidadPortal {
  id: number;
  codigo: string;
  nombre: string;
  descripcion: string;
  requiere_derivacion: boolean;
  duracion_cita_min: number;
}

export interface MedicoPortal {
  id: number;
  nombres: string;
  apellidos: string;
  especialidad: EspecialidadPortal;
  duracion_cita_min: number;
  bio: string;
  foto_url: string;
}

export interface SlotDisponible {
  inicio: string; // RFC3339 UTC
  fin: string; // RFC3339 UTC
  duracion_min: number;
}

export interface Disponibilidad {
  medico_id: number;
  especialidad_id: number;
  zona_horaria: string;
  desde: string;
  hasta: string;
  slots: SlotDisponible[];
}

export interface Derivacion {
  id: number;
  paciente_id: number;
  paciente_nombre: string;
  medico_origen_id: number;
  medico_origen_nombre: string;
  cita_origen_id: number;
  especialidad_destino_id: number;
  especialidad_destino: string;
  medico_destino_id?: number | null;
  medico_destino_nombre?: string;
  motivo: string;
  prioridad: string;
  estado: string;
  vigente_desde: string;
  vigente_hasta: string;
  motivo_cancelacion?: string;
  version: number;
}

export interface CitaCreada {
  id: number;
  codigo: string;
  fecha_hora_inicio: string;
  estado: string;
}

export interface CrearCitaInput {
  medico_id: number;
  fecha_hora_inicio: string;
  tipo: string;
  motivo: string;
  derivacion_id?: number;
}

export async function listarEspecialidades(): Promise<EspecialidadPortal[]> {
  return getData(
    await http.get<Envelope<EspecialidadPortal[]>>("/portal/especialidades"),
  );
}

export async function listarMedicos(
  especialidadID: number,
): Promise<MedicoPortal[]> {
  const respuesta = await http.get<{
    data: MedicoPortal[];
    meta: { limit: number; offset: number; count: number };
  }>(`/portal/medicos?especialidad_id=${especialidadID}`);
  return respuesta.data.data;
}

export async function listarDisponibilidad(
  medicoID: number,
  desde: string,
  hasta: string,
): Promise<Disponibilidad> {
  const respuesta = await http.get<Envelope<Disponibilidad>>(
    `/portal/disponibilidad?medico_id=${medicoID}&desde=${desde}&hasta=${hasta}`,
  );
  return getData(respuesta);
}

export async function listarDerivaciones(): Promise<Derivacion[]> {
  const respuesta = await http.get<{
    data: Derivacion[];
    meta: { limit: number; offset: number; count: number };
  }>("/portal/derivaciones");
  return respuesta.data.data;
}

export async function crearCita(
  input: CrearCitaInput,
): Promise<{ data: CitaCreada; status: number }> {
  const respuesta = await http.post<Envelope<CitaCreada>>("/portal/citas", input);
  return { data: getData(respuesta), status: respuesta.status };
}
