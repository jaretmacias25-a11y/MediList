package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/jwks"
	"github.com/clerk/clerk-sdk-go/v2/jwt"
	"github.com/clerk/clerk-sdk-go/v2/user"
)

// IdentidadClerk es la información mínima extraída de una sesión de Clerk.
type IdentidadClerk struct {
	Subject   string
	Email     string
	Nombres   string
	Apellidos string
}

// VerificadorClerk valida tokens de sesión contra la JWKS de Clerk y
// consulta el perfil del usuario con la API de backend.
type VerificadorClerk struct {
	jwks              *jwks.Client
	users             *user.Client
	authorizedParties map[string]struct{}
}

// NewVerificadorClerk construye el verificador con la clave secreta de Clerk.
// authorizedParties es opcional: si se indica, se valida el claim `azp`.
func NewVerificadorClerk(secretKey string, authorizedParties ...string) (*VerificadorClerk, error) {
	if strings.TrimSpace(secretKey) == "" {
		return nil, errors.New("CLERK_SECRET_KEY no puede estar vacía")
	}
	cfg := &clerk.ClientConfig{
		BackendConfig: clerk.BackendConfig{Key: clerk.String(secretKey)},
	}
	partes := make(map[string]struct{}, len(authorizedParties))
	for _, parte := range authorizedParties {
		if parte = strings.TrimRight(strings.TrimSpace(parte), "/"); parte != "" {
			partes[parte] = struct{}{}
		}
	}
	return &VerificadorClerk{
		jwks:              jwks.NewClient(cfg),
		users:             user.NewClient(cfg),
		authorizedParties: partes,
	}, nil
}

// Verificar comprueba la firma, el emisor y la expiración del token y
// obtiene el correo verificado del usuario.
func (v *VerificadorClerk) Verificar(ctx context.Context, token string) (*IdentidadClerk, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("token de Clerk vacío")
	}

	claims, err := jwt.Verify(ctx, &jwt.VerifyParams{
		Token:      token,
		JWKSClient: v.jwks,
		AuthorizedPartyHandler: func(azp string) bool {
			if len(v.authorizedParties) == 0 {
				return true
			}
			_, ok := v.authorizedParties[strings.TrimRight(azp, "/")]
			return ok
		},
	})
	if err != nil {
		slog.Warn("clerk_jwt_verification_failed", "error", err)
		return nil, fmt.Errorf("token de Clerk inválido: %w", err)
	}
	if strings.TrimSpace(claims.Subject) == "" {
		return nil, errors.New("token de Clerk sin subject")
	}

	perfil, err := v.users.Get(ctx, claims.Subject)
	if err != nil {
		return nil, fmt.Errorf("consultar usuario de Clerk: %w", err)
	}

	email := correoVerificado(perfil)
	if email == "" {
		return nil, errors.New("la cuenta de Clerk no tiene un correo verificado")
	}

	var nombres, apellidos string
	if perfil.FirstName != nil {
		nombres = strings.TrimSpace(*perfil.FirstName)
	}
	if perfil.LastName != nil {
		apellidos = strings.TrimSpace(*perfil.LastName)
	}

	return &IdentidadClerk{
		Subject:   claims.Subject,
		Email:     strings.ToLower(email),
		Nombres:   nombres,
		Apellidos: apellidos,
	}, nil
}

func correoVerificado(perfil *clerk.User) string {
	if perfil == nil {
		return ""
	}
	var respaldo string
	for _, correo := range perfil.EmailAddresses {
		if correo == nil || correo.Verification == nil || correo.Verification.Status != "verified" {
			continue
		}
		if perfil.PrimaryEmailAddressID != nil && correo.ID == *perfil.PrimaryEmailAddressID {
			return strings.TrimSpace(correo.EmailAddress)
		}
		if respaldo == "" {
			respaldo = correo.EmailAddress
		}
	}
	return strings.TrimSpace(respaldo)
}
