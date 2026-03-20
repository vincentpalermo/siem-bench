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
