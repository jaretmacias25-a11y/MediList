package service

import (
	"fmt"
	"unicode"

	"golang.org/x/crypto/bcrypt"

	"medilist/internal/service/entities"
)

// Hash bcrypt de una contraseña aleatoria. Se usa para igualar el tiempo de
// respuesta cuando el usuario no existe y evitar enumeración de cuentas.
const dummyPasswordHash = "$2a$12$IQcTpGH.hKVzKbOz2oI2GOv2FKLBdJwUqmO4u76CST5mvS/qBUa4a"

func validarPassword(password string) error {
	if len(password) < 12 {
		return entities.NewValidationError("la contraseña debe tener al menos 12 caracteres")
	}
	if len(password) > 72 {
		return entities.NewValidationError("la contraseña no puede superar los 72 caracteres")
	}

	var tieneMayuscula, tieneMinuscula, tieneDigito, tieneEspecial bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			tieneMayuscula = true
		case unicode.IsLower(r):
			tieneMinuscula = true
		case unicode.IsDigit(r):
			tieneDigito = true
		default:
			tieneEspecial = true
		}
	}
	if !tieneMayuscula || !tieneMinuscula || !tieneDigito || !tieneEspecial {
		return entities.NewValidationError(
			"la contraseña debe incluir mayúscula, minúscula, número y carácter especial")
	}
	return nil
}

func hashearPassword(password string, cost int) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", fmt.Errorf("generar hash de contraseña: %w", err)
	}
	return string(hash), nil
}

func verificarPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func consumirTiempoPassword(password string) {
	_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(password))
}
