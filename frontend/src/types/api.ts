// Tipos del contrato de MediList (docs/openapi.yaml).
// Los campos que no use la UI pueden omitirse, pero no inventar nombres:
// la fuente de verdad es el JSON real de la API.

export interface Envelope<T> {
  data: T;
}

export interface ListMeta {
  total: number;
  limit: number;
  offset: number;
}

export interface ListEnvelope<T> {
  data: T[];
  meta: ListMeta;
}

export interface ApiErrorBody {
  error: {
    code: string;
    message: string;
  };
  request_id?: string;
}

export interface TokenPair {
  access_token: string;
  refresh_token: string;
  token_type: "Bearer";
  expires_in: number;
  usuario: Usuario;
}

export interface Usuario {
  id: number;
  username: string;
  email: string;
  nombres: string;
  apellidos: string;
  activo: boolean;
  ultimo_acceso?: string | null;
  email_verificado: boolean;
  email_verificado_en?: string | null;
  terminos_aceptados_en?: string | null;
  version_terminos?: string;
  roles: string[];
  permisos: string[];
  version: number;
  created_at: string;
  updated_at: string;
}

// GET /auth/me: usuario + perfiles clínicos vinculados (solo lectura).
export interface MeUser extends Usuario {
  paciente_id?: number | null;
  medico_id?: number | null;
  requiere_completar_registro?: boolean;
}

export interface ClerkPendienteResponse {
  registro_pendiente: true;
  email: string;
  nombres: string;
  apellidos: string;
}

export interface RegistroClerkInput {
  token: string;
  nombres: string;
  apellidos: string;
  tipo_documento: string;
  numero_documento: string;
  fecha_nacimiento: string;
  sexo: string;
  telefono: string;
  direccion: string;
}

/** Roles reconocidos por el sistema. */
export const ROLES = [
  "admin",
  "recepcion",
  "enfermeria",
  "medico",
  "auditor",
  "paciente",
] as const;

export type Rol = (typeof ROLES)[number];

/** Regla de acceso para guardas de ruta. */
export interface AccessRule {
  /** Todos los permisos listados deben estar presentes. */
  permissions?: string[];
  /** Al menos uno de los roles listados debe estar presente. */
  roles?: Rol[];
  /** Si true, basta con cumplir uno de los criterios (permission OR role). */
  any?: boolean;
}
