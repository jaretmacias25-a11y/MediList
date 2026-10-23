package postgres

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PoolOptions controla el pool y los límites de espera de PostgreSQL.
// Los timeouts evitan que una transacción bloqueada consuma una conexión
// indefinidamente cuando hay alta concurrencia.
type PoolOptions struct {
	MaxConns         int
	MinConns         int
	MaxConnIdleTime  time.Duration
	MaxConnLifetime  time.Duration
	StatementTimeout time.Duration
	LockTimeout      time.Duration
	IdleTxTimeout    time.Duration
	ApplicationName  string
}

func (o PoolOptions) normalized() PoolOptions {
	if o.MaxConns <= 0 {
		o.MaxConns = 20
	}
	if o.MinConns < 0 {
		o.MinConns = 0
	}
	if o.MinConns > o.MaxConns {
		o.MinConns = o.MaxConns
	}
	if o.MaxConnIdleTime <= 0 {
		o.MaxConnIdleTime = 15 * time.Minute
	}
	if o.MaxConnLifetime <= 0 {
		o.MaxConnLifetime = time.Hour
	}
	if o.ApplicationName == "" {
		o.ApplicationName = "medilist-api"
	}
	return o
}

// NewPool crea y verifica el pool de conexiones a PostgreSQL.
func NewPool(ctx context.Context, databaseURL string, opts PoolOptions) (*pgxpool.Pool, error) {
	opts = opts.normalized()

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("configuración de base de datos inválida: %w", err)
	}
	cfg.MaxConns = int32(opts.MaxConns)
	cfg.MinConns = int32(opts.MinConns)
	cfg.MaxConnIdleTime = opts.MaxConnIdleTime
	cfg.MaxConnLifetime = opts.MaxConnLifetime
	cfg.HealthCheckPeriod = 30 * time.Second
	cfg.ConnConfig.RuntimeParams["application_name"] = opts.ApplicationName

	if opts.StatementTimeout > 0 {
		cfg.ConnConfig.RuntimeParams["statement_timeout"] = strconv.FormatInt(opts.StatementTimeout.Milliseconds(), 10)
	}
	if opts.LockTimeout > 0 {
		cfg.ConnConfig.RuntimeParams["lock_timeout"] = strconv.FormatInt(opts.LockTimeout.Milliseconds(), 10)
	}
	if opts.IdleTxTimeout > 0 {
		cfg.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] = strconv.FormatInt(opts.IdleTxTimeout.Milliseconds(), 10)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("crear pool de conexiones: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("conectar a postgres: %w", err)
	}
	return pool, nil
}
