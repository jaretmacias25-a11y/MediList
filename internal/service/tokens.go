package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"medilist/internal/service/entities"
)

// Claims contiene la identidad y los permisos embebidos en el token de acceso.
type Claims struct {
	UsuarioID    int64    `json:"uid"`
	Username     string   `json:"username"`
	TokenVersion int      `json:"ver"`
	Roles        []string `json:"roles"`
	Permisos     []string `json:"permisos"`
	jwt.RegisteredClaims
}

// TokenManager firma y valida tokens JWT HS256.
type TokenManager struct {
	secret    []byte
	issuer    string
	accessTTL time.Duration
}

// NewTokenManager construye el gestor de tokens.
func NewTokenManager(secret, issuer string, accessTTL time.Duration) *TokenManager {
	return &TokenManager{
		secret:    []byte(secret),
		issuer:    issuer,
		accessTTL: accessTTL,
	}
}

// Generar emite un token de acceso para el usuario.
func (m *TokenManager) Generar(u *entities.Usuario) (string, time.Time, error) {
	now := time.Now()
	expira := now.Add(m.accessTTL)
	jti, err := randomHex(16)
	if err != nil {
		return "", time.Time{}, err
	}
	roles := u.Roles
	if roles == nil {
		roles = []string{}
	}
	permisos := u.Permisos
	if permisos == nil {
		permisos = []string{}
	}

	claims := Claims{
		UsuarioID:    u.ID,
		Username:     u.Username,
		TokenVersion: u.TokenVersion,
		Roles:        roles,
		Permisos:     permisos,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   strconv.FormatInt(u.ID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(-5 * time.Second)),
			ExpiresAt: jwt.NewNumericDate(expira),
			ID:        jti,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("firmar token: %w", err)
	}
	return signed, expira, nil
}

// Validar comprueba firma, algoritmo, emisor y expiración.
func (m *TokenManager) Validar(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(t *jwt.Token) (any, error) {
			if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, fmt.Errorf("algoritmo de firma no permitido")
			}
			return m.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("%w: token inválido o expirado", entities.ErrUnauthorized)
	}
	if claims.UsuarioID <= 0 {
		return nil, fmt.Errorf("%w: token sin usuario", entities.ErrUnauthorized)
	}
	return claims, nil
}

func randomHex(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generar aleatoriedad segura: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
