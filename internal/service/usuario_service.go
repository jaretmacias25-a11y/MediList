package service

import (
	"context"
	"fmt"
	"net/mail"
	"regexp"
	"slices"
	"sort"
	"strings"

	"medilist/internal/service/entities"
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9._-]{3,50}$`)

// CrearUsuarioInput son los datos para crear una cuenta.
type CrearUsuarioInput struct {
	Username  string
	Email     string
	Nombres   string
	Apellidos string
	Password  string
	Roles     []string
}

// ActualizarUsuarioInput son los datos editables de una cuenta.
type ActualizarUsuarioInput struct {
	Email     string
	Nombres   string
	Apellidos string
	Activo    *bool
	Roles     []string
	Password  *string
	Version   int64
}

// UsuarioService gestiona usuarios, roles y bloqueos.
type UsuarioService struct {
	store      UsuarioStore
	roles      RolStore
	bcryptCost int
}

// NewUsuarioService construye el servicio de usuarios.
func NewUsuarioService(store UsuarioStore, roles RolStore, bcryptCost int) *UsuarioService {
	return &UsuarioService{store: store, roles: roles, bcryptCost: bcryptCost}
}

func (s *UsuarioService) Crear(ctx context.Context, in CrearUsuarioInput) (*entities.Usuario, error) {
	username := strings.ToLower(strings.TrimSpace(in.Username))
	if !usernamePattern.MatchString(username) {
		return nil, entities.NewValidationError(
			"username debe tener entre 3 y 50 caracteres y solo usar letras, números, punto, guion o guion bajo")
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if err := validarEmail(email); err != nil {
		return nil, err
	}
	nombres := strings.TrimSpace(in.Nombres)
	apellidos := strings.TrimSpace(in.Apellidos)
	if nombres == "" || apellidos == "" {
		return nil, entities.NewValidationError("nombres y apellidos son obligatorios")
	}
	if err := validarPassword(in.Password); err != nil {
		return nil, err
	}
	roles, err := s.resolverRoles(ctx, in.Roles)
	if err != nil {
		return nil, err
	}
	hash, err := hashearPassword(in.Password, s.bcryptCost)
	if err != nil {
		return nil, err
	}

	usuario := entities.Usuario{
		Username:     username,
		Email:        email,
		PasswordHash: hash,
		Nombres:      nombres,
		Apellidos:    apellidos,
		Activo:       true,
		Roles:        roles,
	}
	if err := s.store.Crear(ctx, &usuario); err != nil {
		return nil, err
	}
	return s.store.ObtenerPorID(ctx, usuario.ID)
}

func (s *UsuarioService) Obtener(ctx context.Context, id int64) (*entities.Usuario, error) {
	if id <= 0 {
		return nil, entities.NewValidationError("id de usuario inválido")
	}
	return s.store.ObtenerPorID(ctx, id)
}

func (s *UsuarioService) Listar(ctx context.Context, opts entities.ListOptions) ([]entities.Usuario, error) {
	opts = opts.Normalized()
	opts.Search = strings.TrimSpace(opts.Search)
	return s.store.Listar(ctx, opts)
}

// Actualizar cambia datos, roles, estado o contraseña. Si cambian roles,
// contraseña o se desactiva la cuenta, revoca todas las sesiones.
func (s *UsuarioService) Actualizar(ctx context.Context, id int64, in ActualizarUsuarioInput, actorID int64) (*entities.Usuario, error) {
	if id <= 0 {
		return nil, entities.NewValidationError("id de usuario inválido")
	}
	actual, err := s.store.ObtenerPorID(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.Version > 0 && in.Version != actual.Version {
		return nil, fmt.Errorf(
			"%w: el usuario fue modificado por otra operación; vuelve a consultarlo e inténtalo de nuevo",
			entities.ErrConflict)
	}

	email := strings.ToLower(strings.TrimSpace(in.Email))
	if err := validarEmail(email); err != nil {
		return nil, err
	}
	nombres := strings.TrimSpace(in.Nombres)
	apellidos := strings.TrimSpace(in.Apellidos)
	if nombres == "" || apellidos == "" {
		return nil, entities.NewValidationError("nombres y apellidos son obligatorios")
	}
	roles, err := s.resolverRoles(ctx, in.Roles)
	if err != nil {
		return nil, err
	}

	activo := actual.Activo
	if in.Activo != nil {
		activo = *in.Activo
	}
	if id == actorID && !activo {
		return nil, entities.NewValidationError("no puedes desactivar tu propia cuenta")
	}
	if id == actorID && slices.Contains(actual.Roles, "admin") && !slices.Contains(roles, "admin") {
		return nil, entities.NewValidationError("no puedes quitarte el rol admin a ti mismo")
	}

	passwordHash := ""
	revocarSesiones := !activo && actual.Activo
	if in.Password != nil {
		if err := validarPassword(*in.Password); err != nil {
			return nil, err
		}
		passwordHash, err = hashearPassword(*in.Password, s.bcryptCost)
		if err != nil {
			return nil, err
		}
		revocarSesiones = true
	}
	if !sameRoles(actual.Roles, roles) {
		revocarSesiones = true
	}

	usuario := entities.Usuario{
		ID:        id,
		Username:  actual.Username,
		Email:     email,
		Nombres:   nombres,
		Apellidos: apellidos,
		Activo:    activo,
		Roles:     roles,
		Version:   actual.Version,
	}
	if err := s.store.Actualizar(ctx, &usuario, passwordHash, revocarSesiones); err != nil {
		return nil, err
	}
	return s.store.ObtenerPorID(ctx, id)
}

// Desbloquear limpia el bloqueo por intentos fallidos.
func (s *UsuarioService) Desbloquear(ctx context.Context, id int64) error {
	if id <= 0 {
		return entities.NewValidationError("id de usuario inválido")
	}
	return s.store.Desbloquear(ctx, id)
}

// RevocarSesiones invalida de inmediato los access tokens y refresh tokens.
func (s *UsuarioService) RevocarSesiones(ctx context.Context, id int64) error {
	if id <= 0 {
		return entities.NewValidationError("id de usuario inválido")
	}
	return s.store.RevocarSesiones(ctx, id)
}

// ListarRoles devuelve el catálogo de roles con sus permisos.
func (s *UsuarioService) ListarRoles(ctx context.Context) ([]entities.Rol, error) {
	return s.roles.Listar(ctx)
}

func (s *UsuarioService) resolverRoles(ctx context.Context, codigos []string) ([]string, error) {
	normalizados := normalizarRoles(codigos)
	if len(normalizados) == 0 {
		return nil, entities.NewValidationError("debes asignar al menos un rol")
	}
	roles, err := s.roles.ObtenerPorCodigos(ctx, normalizados)
	if err != nil {
		return nil, err
	}
	if len(roles) != len(normalizados) {
		return nil, entities.NewValidationError("uno o más roles no existen")
	}
	resultado := make([]string, 0, len(roles))
	for _, rol := range roles {
		resultado = append(resultado, rol.Codigo)
	}
	sort.Strings(resultado)
	return resultado, nil
}

func normalizarRoles(codigos []string) []string {
	vistos := make(map[string]struct{}, len(codigos))
	resultado := make([]string, 0, len(codigos))
	for _, codigo := range codigos {
		codigo = strings.ToLower(strings.TrimSpace(codigo))
		if codigo == "" {
			continue
		}
		if _, existe := vistos[codigo]; existe {
			continue
		}
		vistos[codigo] = struct{}{}
		resultado = append(resultado, codigo)
	}
	return resultado
}

func validarEmail(email string) error {
	if email == "" {
		return entities.NewValidationError("email es obligatorio")
	}
	direccion, err := mail.ParseAddress(email)
	if err != nil || direccion.Address != email {
		return entities.NewValidationError("email no tiene un formato válido")
	}
	return nil
}

func sameRoles(a, b []string) bool {
	ac := append([]string(nil), a...)
	bc := append([]string(nil), b...)
	sort.Strings(ac)
	sort.Strings(bc)
	return slices.Equal(ac, bc)
}

// AuditoriaService expone la consulta de la bitácora.
type AuditoriaService struct {
	store AuditoriaStore
}

// NewAuditoriaService construye el servicio de auditoría.
func NewAuditoriaService(store AuditoriaStore) *AuditoriaService {
	return &AuditoriaService{store: store}
}

func (s *AuditoriaService) Listar(ctx context.Context, filtro entities.AuditoriaFiltro) ([]entities.Auditoria, error) {
	filtro.ListOptions = filtro.ListOptions.Normalized()
	filtro.Search = strings.TrimSpace(filtro.Search)
	filtro.Accion = strings.TrimSpace(filtro.Accion)
	if filtro.UsuarioID < 0 {
		return nil, entities.NewValidationError("usuario_id no es válido")
	}
	return s.store.Listar(ctx, filtro)
}
