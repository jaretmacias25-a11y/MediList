import { http, getData, ApiError } from "@/lib/http";
import type { Envelope } from "@/types/api";
import type { Cita } from "@/services/citas";

interface ListaRespuesta {
  data: Cita[];
  meta: { limit: number; offset: number; count: number };
}

export async function listarAgenda(): Promise<Cita[]> {
  const respuesta = await http.get<ListaRespuesta>("/medico/agenda");
  return respuesta.data.data;
}

export async function obtenerCita(id: number): Promise<Cita> {
  return getData(await http.get<Envelope<Cita>>(`/medico/citas/${id}`));
}

export async function cambiarEstado(id: number, estado: string): Promise<Cita> {
  return getData(
    await http.patch<Envelope<Cita>>(`/medico/citas/${id}/estado`, { estado }),
  );
}

export interface NotaCita {
  id: number;
  cita_id: number;
  medico_id: number;
  motivo: string;
  evaluacion: string;
  plan: string;
  version: number;
  created_at: string;
  updated_at: string;
}

export interface NotaCitaInput {
  motivo: string;
  evaluacion: string;
  plan: string;
  version: number;
}

/** Devuelve la nota o null si todavía no existe (404). */
export async function obtenerNota(id: number): Promise<NotaCita | null> {
  try {
    return getData(await http.get<Envelope<NotaCita>>(`/medico/citas/${id}/nota`));
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) return null;
    throw error;
  }
}

export async function guardarNota(
  id: number,
  input: NotaCitaInput,
  esActualizacion: boolean,
): Promise<NotaCita> {
  const respuesta = esActualizacion
    ? await http.put<Envelope<NotaCita>>(`/medico/citas/${id}/nota`, input)
    : await http.post<Envelope<NotaCita>>(`/medico/citas/${id}/nota`, input);
  return getData(respuesta);
}

export interface DerivacionMedico {
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

export interface DerivacionInput {
  especialidad_destino_id: number;
  medico_destino_id?: number | null;
  motivo: string;
  prioridad: string;
}

interface ListaDerivaciones {
  data: DerivacionMedico[];
  meta: { limit: number; offset: number; count: number };
}

export async function listarDerivaciones(): Promise<DerivacionMedico[]> {
  const respuesta = await http.get<ListaDerivaciones>("/medico/derivaciones");
  return respuesta.data.data;
}

export async function crearDerivacion(
  id: number,
  input: DerivacionInput,
): Promise<DerivacionMedico> {
  return getData(
    await http.post<Envelope<DerivacionMedico>>(
      `/medico/citas/${id}/derivaciones`,
      input,
    ),
  );
}

export interface EspecialidadClinica {
  id: number;
  codigo: string;
  nombre: string;
  descripcion: string;
  requiere_derivacion: boolean;
  activa: boolean;
  orden: number;
  duracion_cita_min: number;
  version: number;
}

export interface MedicoClinico {
  id: number;
  persona: { nombres: string; apellidos: string };
  especialidad: { id: number; nombre: string };
  numero_licencia: string;
  activo: boolean;
}

interface ListaEspecialidades {
  data: EspecialidadClinica[];
  meta: { limit: number; offset: number; count: number };
}

interface ListaMedicos {
  data: MedicoClinico[];
  meta: { limit: number; offset: number; count: number };
}

export async function listarEspecialidadesClinicas(): Promise<EspecialidadClinica[]> {
  const respuesta = await http.get<ListaEspecialidades>("/especialidades");
  return respuesta.data.data;
}

export async function listarMedicosClinicos(): Promise<MedicoClinico[]> {
  const respuesta = await http.get<ListaMedicos>("/medicos");
  return respuesta.data.data;
}
