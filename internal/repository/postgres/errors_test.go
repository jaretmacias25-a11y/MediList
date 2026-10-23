package postgres

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"medilist/internal/service/entities"
)

func TestMapErrorTraduceLongitudExcedida(t *testing.T) {
	err := mapError(&pgconn.PgError{Code: "22001", Message: "value too long"})
	if !errors.Is(err, entities.ErrValidation) {
		t.Fatalf("se esperaba ErrValidation, se obtuvo: %v", err)
	}
}

func TestMapErrorTraduceUniqueViolation(t *testing.T) {
	err := mapError(&pgconn.PgError{Code: "23505", Message: "duplicate key"})
	if !errors.Is(err, entities.ErrConflict) {
		t.Fatalf("se esperaba ErrConflict, se obtuvo: %v", err)
	}
}

func TestMapErrorTraduceExclusionViolation(t *testing.T) {
	err := mapError(&pgconn.PgError{Code: "23P01", Message: "exclusion constraint"})
	if !errors.Is(err, entities.ErrConflict) {
		t.Fatalf("se esperaba ErrConflict, se obtuvo: %v", err)
	}
}

func TestMapErrorTraduceConflictoDeSerializacion(t *testing.T) {
	err := mapError(&pgconn.PgError{Code: "40001", Message: "serialization failure"})
	if !errors.Is(err, entities.ErrConflict) {
		t.Fatalf("se esperaba ErrConflict, se obtuvo: %v", err)
	}
}
