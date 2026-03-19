package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"siem-bench/internal/model"
)

type Storage struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, dsn string) (*Storage, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return &Storage{
		pool: pool,
	}, nil
}

func (s *Storage) Close() {
	s.pool.Close()
}

func (s *Storage) InsertEvent(ctx context.Context, event model.Event) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO events (
			id,
			timestamp,
			source_type,
			host,
			user_name,
			src_ip,
			dst_ip,
			event_code,
			severity,
			message,
			raw
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)
		ON CONFLICT (id) DO NOTHING
	`,
		event.ID,
		event.Timestamp,
		event.SourceType,
		event.Host,
		event.UserName,
		event.SrcIP,
		event.DstIP,
		event.EventCode,
		event.Severity,
		event.Message,
		event.Raw,
	)

	return err
}

func (s *Storage) CountEvents(ctx context.Context) (int64, error) {
	var count int64
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM events`).Scan(&count)
	return count, err
}
