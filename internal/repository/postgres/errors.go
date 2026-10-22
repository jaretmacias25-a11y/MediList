package postgres

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"medilist/internal/service/entities"
)

// mapError traduce errores de PostgreSQL a errores de dominio.
func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return entities.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return fmt.Errorf("%w: ya existe un registro con el mismo valor único", entities.ErrConflict)
		case "23503": // foreign_key_violation
			return fmt.Errorf("%w: el registro está referenciado por otros datos", entities.ErrConflict)
		case "23514": // check_violation
			return fmt.Errorf("%w: %s", entities.ErrValidation, pgErr.Message)
		case "22001": // string_data_right_truncation
			return fmt.Errorf("%w: uno o más campos exceden la longitud permitida", entities.ErrValidation)
		case "23502": // not_null_violation
			return fmt.Errorf("%w: falta un campo obligatorio", entities.ErrValidation)
		case "22P02": // invalid_text_representation
			return fmt.Errorf("%w: uno o más valores tienen un formato inválido", entities.ErrValidation)
		}
	}
	return err
}

func noEncontrado(recurso string, id int64) error {
	return fmt.Errorf("%w: %s %d", entities.ErrNotFound, recurso, id)
}

type rowScanner interface {
	Scan(dest ...any) error
}
