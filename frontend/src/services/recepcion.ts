import { http, getData } from "@/lib/http";
import type { Envelope } from "@/types/api";
import type { Cita } from "@/services/citas";

export interface PersonaPaciente {
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

export interface PacienteGlobal {
  id: number;
  usuario_id?: number | null;
  medico_responsable_id?: number | null;
  persona: PersonaPaciente;
  numero_historia: string;
  grupo_sanguineo: string;
  factor_rh: string;
  contacto_emergencia_nombre: string;
  contacto_emergencia_telefono: string;
  activo: boolean;
  version: number;
  created_at: string;
  updated_at: string;
}

export interface PacienteInput {
  persona: {
    tipo_documento: string;
    numero_documento: string;
    nombres: string;
    apellidos: string;
    fecha_nacimiento: string;
    sexo: string;
    telefono: string;
    email: string;
    direccion: string;
  };
  grupo_sanguineo: string;
  factor_rh: string;
  contacto_emergencia_nombre: string;
  contacto_emergencia_telefono: string;
  version: number;
}

interface ListaPacientes {
  data: PacienteGlobal[];
  meta: { limit: number; offset: number; count: number };
}

export async function listarPacientes(
  search: string,
  limit: number,
  offset: number,
): Promise<{ items: PacienteGlobal[]; meta: { limit: number; offset: number; count: number } }> {
  const params = new URLSearchParams();
  if (search.trim()) params.set("search", search.trim());
  params.set("limit", String(limit));
  params.set("offset", String(offset));
  const respuesta = await http.get<ListaPacientes>(`/pacientes?${params.toString()}`);
  return { items: respuesta.data.data, meta: respuesta.data.meta };
}

export async function obtenerPaciente(id: number): Promise<PacienteGlobal> {
  return getData(await http.get<Envelope<PacienteGlobal>>(`/pacientes/${id}`));
}

export async function crearPaciente(input: PacienteInput): Promise<PacienteGlobal> {
  return getData(await http.post<Envelope<PacienteGlobal>>("/pacientes", input));
}

export async function actualizarPaciente(
  id: number,
  input: PacienteInput,
): Promise<PacienteGlobal> {
  return getData(await http.put<Envelope<PacienteGlobal>>(`/pacientes/${id}`, input));
}

export async function eliminarPaciente(id: number): Promise<void> {
  await http.delete(`/pacientes/${id}`);
}

interface ListaCitas {
  data: Cita[];
  meta: { limit: number; offset: number; count: number };
}

export async function listarCitasGlobales(
  search: string,
  limit: number,
  offset: number,
): Promise<{ items: Cita[]; meta: { limit: number; offset: number; count: number } }> {
  const params = new URLSearchParams();
  if (search.trim()) params.set("search", search.trim());
  params.set("limit", String(limit));
  params.set("offset", String(offset));
  const respuesta = await http.get<ListaCitas>(`/citas?${params.toString()}`);
  return { items: respuesta.data.data, meta: respuesta.data.meta };
}

export interface CitaAdminInput {
  paciente_id: number;
  medico_id: number;
  fecha_hora_inicio: string;
  duracion_min: number;
  tipo: string;
  motivo: string;
  estado: string;
  version: number;
}

export async function actualizarCita(
  id: number,
  input: CitaAdminInput,
): Promise<Cita> {
  return getData(await http.put<Envelope<Cita>>(`/citas/${id}`, input));
}

export async function eliminarCita(id: number): Promise<void> {
  await http.delete(`/citas/${id}`);
}

// ---------------------------------------------------------------------------
// Agenda global
// ---------------------------------------------------------------------------

export interface Horario {
  id: number;
  medico_id: number;
  dia_semana: number;
  hora_inicio: string;
  hora_fin: string;
  duracion_slot_min: number;
  vigente_desde: string;
  vigente_hasta?: string | null;
  activo: boolean;
  version: number;
  created_at: string;
  updated_at: string;
}

export interface HorarioInput {
  dia_semana: number;
  hora_inicio: string;
  hora_fin: string;
  duracion_slot_min: number;
  vigente_desde?: string;
  vigente_hasta?: string | null;
  activo?: boolean;
  version: number;
}

export interface Bloqueo {
  id: number;
  medico_id: number;
  inicio: string;
  fin: string;
  motivo: string;
  creado_por?: number | null;
  created_at: string;
  updated_at: string;
}

export interface BloqueoInput {
  inicio: string;
  fin: string;
  motivo: string;
}

interface ListaHorarios {
  data: Horario[];
  meta: { limit: number; offset: number; count: number };
}

interface ListaBloqueos {
  data: Bloqueo[];
  meta: { limit: number; offset: number; count: number };
}

export async function listarHorarios(medicoID: number): Promise<Horario[]> {
  const respuesta = await http.get<ListaHorarios>(`/medicos/${medicoID}/horarios`);
  return respuesta.data.data;
}

export async function crearHorario(
  medicoID: number,
  input: HorarioInput,
): Promise<Horario> {
  return getData(await http.post<Envelope<Horario>>(`/medicos/${medicoID}/horarios`, input));
}

export async function actualizarHorario(
  medicoID: number,
  horarioID: number,
  input: HorarioInput,
): Promise<Horario> {
  return getData(
    await http.put<Envelope<Horario>>(`/medicos/${medicoID}/horarios/${horarioID}`, input),
  );
}

export async function listarBloqueos(medicoID: number): Promise<Bloqueo[]> {
  const respuesta = await http.get<ListaBloqueos>(`/medicos/${medicoID}/bloqueos`);
  return respuesta.data.data;
}

export async function crearBloqueo(
  medicoID: number,
  input: BloqueoInput,
): Promise<Bloqueo> {
  return getData(await http.post<Envelope<Bloqueo>>(`/medicos/${medicoID}/bloqueos`, input));
}

export async function actualizarBloqueo(
  medicoID: number,
  bloqueoID: number,
  input: BloqueoInput,
): Promise<Bloqueo> {
  return getData(
    await http.put<Envelope<Bloqueo>>(`/medicos/${medicoID}/bloqueos/${bloqueoID}`, input),
  );
}

// ---------------------------------------------------------------------------
// Vinculaciones de cuentas
// ---------------------------------------------------------------------------

export interface SolicitudVinculacion {
  id: number;
  usuario_id: number;
  username: string;
  email: string;
  paciente_id: number;
  numero_historia: string;
  nombres: string;
  apellidos: string;
  tipo_documento: string;
  numero_documento: string;
  estado: string;
  motivo?: string;
  resuelta_por_usuario_id?: number | null;
  resuelta_en?: string | null;
  created_at: string;
  updated_at: string;
}

interface ListaSolicitudes {
  data: SolicitudVinculacion[];
  meta: { limit: number; offset: number; count: number };
}

export async function listarSolicitudes(): Promise<SolicitudVinculacion[]> {
  const respuesta = await http.get<ListaSolicitudes>(
    "/cuentas/vinculaciones?estado=todas",
  );
  return respuesta.data.data;
}

export async function aprobarSolicitud(id: number): Promise<void> {
  await http.post(`/cuentas/vinculaciones/${id}/aprobar`);
}

export async function rechazarSolicitud(id: number, motivo: string): Promise<void> {
  await http.post(`/cuentas/vinculaciones/${id}/rechazar`, { motivo });
}
