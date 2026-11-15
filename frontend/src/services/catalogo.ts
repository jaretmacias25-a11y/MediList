import { http, getData } from "@/lib/http";
import type { Envelope } from "@/types/api";

// ---------------------------------------------------------------------------
// Especialidades (administración)
// ---------------------------------------------------------------------------

export interface EspecialidadAdmin {
  id: number;
  codigo: string;
  nombre: string;
  descripcion: string;
  requiere_derivacion: boolean;
  activa: boolean;
  orden: number;
  duracion_cita_min: number;
  version: number;
  created_at: string;
  updated_at: string;
}

export interface EspecialidadInput {
  codigo: string;
  nombre: string;
  descripcion: string;
  requiere_derivacion: boolean;
  activa: boolean;
  orden: number;
  duracion_cita_min: number;
  version: number;
}

interface ListaEspecialidades {
  data: EspecialidadAdmin[];
  meta: { limit: number; offset: number; count: number };
}

export async function listarEspecialidadesAdmin(): Promise<EspecialidadAdmin[]> {
  const respuesta = await http.get<ListaEspecialidades>("/especialidades?limit=100&offset=0");
  return respuesta.data.data;
}

export async function crearEspecialidad(input: EspecialidadInput): Promise<EspecialidadAdmin> {
  return getData(await http.post<Envelope<EspecialidadAdmin>>("/especialidades", input));
}

export async function actualizarEspecialidad(
  id: number,
  input: EspecialidadInput,
): Promise<EspecialidadAdmin> {
  return getData(await http.put<Envelope<EspecialidadAdmin>>(`/especialidades/${id}`, input));
}

export async function eliminarEspecialidad(id: number): Promise<void> {
  await http.delete(`/especialidades/${id}`);
}

// ---------------------------------------------------------------------------
// Médicos (administración)
// ---------------------------------------------------------------------------

export interface MedicoAdmin {
  id: number;
  usuario_id?: number | null;
  persona: {
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
  };
  especialidad: { id: number; nombre: string };
  numero_licencia: string;
  tipo_profesional: string;
  duracion_cita_min: number;
  bio: string;
  foto_url: string;
  acepta_portal: boolean;
  activo: boolean;
  version: number;
}

export interface MedicoInput {
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
  especialidad_id: number;
  numero_licencia: string;
  tipo_profesional: string;
  duracion_cita_min: number;
  bio: string;
  foto_url: string;
  acepta_portal: boolean;
  version: number;
}

interface ListaMedicos {
  data: MedicoAdmin[];
  meta: { limit: number; offset: number; count: number };
}

export async function listarMedicosAdmin(): Promise<MedicoAdmin[]> {
  const respuesta = await http.get<ListaMedicos>("/medicos?limit=100&offset=0");
  return respuesta.data.data;
}

export async function crearMedico(input: MedicoInput): Promise<MedicoAdmin> {
  return getData(await http.post<Envelope<MedicoAdmin>>("/medicos", input));
}

export async function actualizarMedico(
  id: number,
  input: MedicoInput,
): Promise<MedicoAdmin> {
  return getData(await http.put<Envelope<MedicoAdmin>>(`/medicos/${id}`, input));
}

export async function eliminarMedico(id: number): Promise<void> {
  await http.delete(`/medicos/${id}`);
}
