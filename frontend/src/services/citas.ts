import { http, getData } from "@/lib/http";
import type { Envelope } from "@/types/api";

export interface Cita {
  id: number;
  codigo: string;
  paciente_id: number;
  paciente_nombre: string;
  medico_id: number;
  medico_nombre: string;
  especialidad_nombre: string;
  fecha_hora_inicio: string;
  duracion_min: number;
  tipo: string;
  motivo: string;
  estado: string;
  canal: string;
  derivacion_id?: number | null;
  motivo_cancelacion?: string;
  version: number;
  created_at: string;
  updated_at: string;
}

export interface ReprogramarCitaInput {
  fecha_hora_inicio: string;
  motivo?: string;
  version: number;
}

interface ListaRespuesta {
  data: Cita[];
  meta: { limit: number; offset: number; count: number };
}

export async function listarCitas(): Promise<Cita[]> {
  const respuesta = await http.get<ListaRespuesta>("/portal/citas");
  return respuesta.data.data;
}

export async function obtenerCita(id: number): Promise<Cita> {
  return getData(await http.get<Envelope<Cita>>(`/portal/citas/${id}`));
}

export async function reprogramarCita(
  id: number,
  input: ReprogramarCitaInput,
): Promise<Cita> {
  return getData(
    await http.put<Envelope<Cita>>(`/portal/citas/${id}`, input),
  );
}

export async function cancelarCita(id: number, motivo: string): Promise<Cita> {
  return getData(
    await http.post<Envelope<Cita>>(`/portal/citas/${id}/cancelar`, { motivo }),
  );
}
