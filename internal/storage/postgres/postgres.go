package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
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

func (s *Storage) InsertEventsBatch(ctx context.Context, events []model.Event) error {
	if len(events) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for _, event := range events {
		batch.Queue(`
			INSERT INTO events (
				id, timestamp, source_type, host, user_name, src_ip, dst_ip, event_code, severity, message, raw
			) VALUES (
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
	}

	results := s.pool.SendBatch(ctx, batch)
	defer results.Close()

	for range events {
		if _, err := results.Exec(); err != nil {
			return err
		}
	}

	return nil
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

func (s *Storage) SearchByHost(ctx context.Context, host string, limit int) ([]model.EventQueryResult, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, timestamp, source_type, host, user_name, src_ip, dst_ip, event_code, severity, message
		FROM events
		WHERE host = $1
		ORDER BY timestamp DESC
		LIMIT $2
	`, host, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.EventQueryResult
	for rows.Next() {
		var item model.EventQueryResult
		if err := rows.Scan(
			&item.ID,
			&item.Timestamp,
			&item.SourceType,
			&item.Host,
			&item.UserName,
			&item.SrcIP,
			&item.DstIP,
			&item.EventCode,
			&item.Severity,
			&item.Message,
		); err != nil {
			return nil, err
		}
		results = append(results, item)
	}

	return results, rows.Err()
}

func (s *Storage) SearchByUser(ctx context.Context, userName string, limit int) ([]model.EventQueryResult, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, timestamp, source_type, host, user_name, src_ip, dst_ip, event_code, severity, message
		FROM events
		WHERE user_name = $1
		ORDER BY timestamp DESC
		LIMIT $2
	`, userName, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.EventQueryResult
	for rows.Next() {
		var item model.EventQueryResult
		if err := rows.Scan(
			&item.ID,
			&item.Timestamp,
			&item.SourceType,
			&item.Host,
			&item.UserName,
			&item.SrcIP,
			&item.DstIP,
			&item.EventCode,
			&item.Severity,
			&item.Message,
		); err != nil {
			return nil, err
		}
		results = append(results, item)
	}

	return results, rows.Err()
}

func (s *Storage) CountBySeverity(ctx context.Context) ([]model.SeverityCount, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT severity, COUNT(*) AS cnt
		FROM events
		GROUP BY severity
		ORDER BY severity
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.SeverityCount
	for rows.Next() {
		var item model.SeverityCount
		if err := rows.Scan(&item.Severity, &item.Count); err != nil {
			return nil, err
		}
		results = append(results, item)
	}

	return results, rows.Err()
}

func (s *Storage) TopHosts(ctx context.Context, limit int) ([]model.HostCount, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT host, COUNT(*) AS cnt
		FROM events
		GROUP BY host
		ORDER BY cnt DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.HostCount
	for rows.Next() {
		var item model.HostCount
		if err := rows.Scan(&item.Host, &item.Count); err != nil {
			return nil, err
		}
		results = append(results, item)
	}

	return results, rows.Err()
}
