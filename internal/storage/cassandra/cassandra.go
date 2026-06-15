package cassandra

import (
	"context"
	"time"

	"github.com/gocql/gocql"
	"siem-bench/internal/model"
)

type Storage struct {
	session *gocql.Session
}

func New(host string) (*Storage, error) {
	cluster := gocql.NewCluster(host)
	cluster.Keyspace = "siem"
	cluster.Consistency = gocql.Quorum
	cluster.Timeout = 5 * time.Second

	session, err := cluster.CreateSession()
	if err != nil {
		return nil, err
	}

	return &Storage{session: session}, nil
}

func (s *Storage) Close() {
	s.session.Close()
}

func (s *Storage) InsertEventsBatch(ctx context.Context, events []model.Event) error {
	for _, e := range events {
		if err := s.session.Query(`
			INSERT INTO events (
				id, timestamp, source_type, host, user_name,
				src_ip, dst_ip, event_code, severity, message, raw
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			e.ID, e.Timestamp, e.SourceType, e.Host, e.UserName,
			e.SrcIP, e.DstIP, e.EventCode, e.Severity, e.Message, e.Raw,
		).WithContext(ctx).Exec(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Storage) CountEvents(ctx context.Context) (int64, error) {
	var count int64
	err := s.session.Query(`SELECT COUNT(*) FROM events`).WithContext(ctx).Scan(&count)
	return count, err
}