package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RetentionStore implementa la purga de datos históricos.
type RetentionStore struct {
	db *pgxpool.Pool
}

// NewRetentionStore construye el almacén de retención.
func NewRetentionStore(db *pgxpool.Pool) *RetentionStore {
	return &RetentionStore{db: db}
}

func (s *RetentionStore) PurgarErrores(ctx context.Context, antes time.Time) (int64, error) {
	tag, err := s.db.Exec(ctx, `DELETE FROM errores_aplicacion WHERE created_at < $1`, antes)
	if err != nil {
		return 0, mapError(err)
	}
	return tag.RowsAffected(), nil
}

func (s *RetentionStore) PurgarAuditoria(ctx context.Context, antes time.Time) (int64, error) {
	tag, err := s.db.Exec(ctx, `DELETE FROM auditoria WHERE created_at < $1`, antes)
	if err != nil {
		return 0, mapError(err)
	}
	return tag.RowsAffected(), nil
}
