package clickhouse

import (
	"context"

	ch "github.com/ClickHouse/clickhouse-go/v2"

	"siem-bench/internal/model"
)

type Storage struct {
	conn ch.Conn
}

func New(ctx context.Context, dsn string) (*Storage, error) {
	opts, err := ch.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}

	conn, err := ch.Open(opts)
	if err != nil {
		return nil, err
	}

	if err := conn.Ping(ctx); err != nil {
		return nil, err
	}

	return &Storage{
		conn: conn,
	}, nil
}

func (s *Storage) Close() error {
	return s.conn.Close()
}

func (s *Storage) InsertEvent(ctx context.Context, event model.Event) error {
	return s.conn.Exec(ctx, `
		INSERT INTO siem.events
		(id, timestamp, source_type, host, user_name, src_ip, dst_ip, event_code, severity, message, raw)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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

func (s *Storage) InsertEventsBatch(ctx context.Context, events []model.Event) error {
	batch, err := s.conn.PrepareBatch(ctx, `
		INSERT INTO siem.events
		(id, timestamp, source_type, host, user_name, src_ip, dst_ip, event_code, severity, message, raw)
	`)
	if err != nil {
		return err
	}

	for _, event := range events {
		if err := batch.Append(
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
		); err != nil {
			return err
		}
	}

	return batch.Send()
}

func (s *Storage) CountEvents(ctx context.Context) (int64, error) {
	var count uint64
	err := s.conn.QueryRow(ctx, `SELECT count() FROM siem.events`).Scan(&count)
	return int64(count), err
}

func (s *Storage) SearchByHost(ctx context.Context, host string, limit int) ([]model.EventQueryResult, error) {
	rows, err := s.conn.Query(ctx, `
		SELECT id, timestamp, source_type, host, user_name, src_ip, dst_ip, event_code, severity, message
		FROM siem.events
		WHERE host = ?
		ORDER BY timestamp DESC
		LIMIT ?
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
	rows, err := s.conn.Query(ctx, `
		SELECT id, timestamp, source_type, host, user_name, src_ip, dst_ip, event_code, severity, message
		FROM siem.events
		WHERE user_name = ?
		ORDER BY timestamp DESC
		LIMIT ?
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
	rows, err := s.conn.Query(ctx, `
		SELECT severity, count() AS cnt
		FROM siem.events
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
		var count uint64
		if err := rows.Scan(&item.Severity, &count); err != nil {
			return nil, err
		}
		item.Count = int64(count)
		results = append(results, item)
	}

	return results, rows.Err()
}

func (s *Storage) TopHosts(ctx context.Context, limit int) ([]model.HostCount, error) {
	rows, err := s.conn.Query(ctx, `
		SELECT host, count() AS cnt
		FROM siem.events
		GROUP BY host
		ORDER BY cnt DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.HostCount
	for rows.Next() {
		var item model.HostCount
		var count uint64
		if err := rows.Scan(&item.Host, &count); err != nil {
			return nil, err
		}
		item.Count = int64(count)
		results = append(results, item)
	}

	return results, rows.Err()
}
