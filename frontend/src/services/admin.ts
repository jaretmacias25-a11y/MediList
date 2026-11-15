import { http, getData } from "@/lib/http";
import type { Envelope, Usuario } from "@/types/api";
import type { MedicoAdmin } from "@/services/catalogo";

interface ListaUsuarios {
  data: Usuario[];
  meta: { limit: number; offset: number; count: number };
}

export interface Rol {
  id: number;
  codigo: string;
  nombre: string;
  descripcion: string;
  permisos: string[];
}

export interface UsuarioUpdateInput {
  email: string;
  nombres: string;
  apellidos: string;
  activo: boolean;
  roles: string[];
  version: number;
}

export async function listarUsuarios(
  search: string,
  limit: number,
  offset: number,
): Promise<{ items: Usuario[]; meta: { limit: number; offset: number; count: number } }> {
  const params = new URLSearchParams();
  if (search.trim()) params.set("search", search.trim());
  params.set("limit", String(limit));
  params.set("offset", String(offset));
  const respuesta = await http.get<ListaUsuarios>(`/usuarios?${params.toString()}`);
  return { items: respuesta.data.data, meta: respuesta.data.meta };
}

export async function listarRoles(): Promise<Rol[]> {
  const respuesta = await http.get<{ data: Rol[] }>("/roles");
  return respuesta.data.data;
}

export async function actualizarUsuario(
  id: number,
  input: UsuarioUpdateInput,
): Promise<Usuario> {
  return getData(await http.put<Envelope<Usuario>>(`/usuarios/${id}`, input));
}

export async function desbloquearUsuario(id: number): Promise<void> {
  await http.post(`/usuarios/${id}/desbloquear`);
}

export async function revocarSesionesUsuario(id: number): Promise<void> {
  await http.post(`/usuarios/${id}/revocar-sesiones`);
}

export interface ConvertirMedicoInput {
  usuario_id: number;
  especialidad_id: number;
  numero_licencia: string;
  tipo_profesional: string;
  duracion_cita_min: number;
  bio: string;
  foto_url: string;
  acepta_portal: boolean;
}

export async function convertirPacienteEnMedico(
  input: ConvertirMedicoInput,
): Promise<MedicoAdmin> {
  return getData(
    await http.post<Envelope<MedicoAdmin>>(
      "/cuentas/convertir-paciente-medico",
      input,
    ),
  );
}

export interface AuditoriaEvento {
  id: number;
  usuario_id?: number | null;
  username: string;
  accion: string;
  recurso: string;
  metodo: string;
  ruta: string;
  estado_http: number;
  ip: string;
  user_agent: string;
  trace_id?: string;
  created_at: string;
}

interface ListaAuditoria {
  data: AuditoriaEvento[];
  meta: { limit: number; offset: number; count: number };
}

export async function listarAuditoria(
  search: string,
  accion: string,
  traceID: string,
  limit: number,
  offset: number,
): Promise<{ items: AuditoriaEvento[]; meta: { limit: number; offset: number; count: number } }> {
  const params = new URLSearchParams();
  if (search.trim()) params.set("search", search.trim());
  if (accion.trim()) params.set("accion", accion.trim());
  if (traceID.trim()) params.set("trace_id", traceID.trim());
  params.set("limit", String(limit));
  params.set("offset", String(offset));
  const respuesta = await http.get<ListaAuditoria>(`/auditoria?${params.toString()}`);
  return { items: respuesta.data.data, meta: respuesta.data.meta };
}

export interface ErrorSistema {
  id: number;
  trace_id: string;
  servicio: string;
  instancia: string;
  componente: string;
  nivel: string;
  metodo: string;
  ruta: string;
  estado_http: number;
  codigo: string;
  mensaje: string;
  usuario_id?: number | null;
  username: string;
  ip: string;
  user_agent: string;
  stack_trace?: string;
  created_at: string;
}

interface ListaErrores {
  data: ErrorSistema[];
  meta: { limit: number; offset: number; count: number };
}

export async function listarErrores(
  nivel: string,
  servicio: string,
  traceID: string,
  limit: number,
  offset: number,
): Promise<{ items: ErrorSistema[]; meta: { limit: number; offset: number; count: number } }> {
  const params = new URLSearchParams();
  if (nivel.trim()) params.set("nivel", nivel.trim());
  if (servicio.trim()) params.set("servicio", servicio.trim());
  if (traceID.trim()) params.set("trace_id", traceID.trim());
  params.set("limit", String(limit));
  params.set("offset", String(offset));
  const respuesta = await http.get<ListaErrores>(`/errores?${params.toString()}`);
  return { items: respuesta.data.data, meta: respuesta.data.meta };
}
