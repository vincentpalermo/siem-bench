package cassandra

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/gocql/gocql"

	"siem-bench/internal/model"
)

type Storage struct {
	session  *gocql.Session
	keyspace string
}

func New(ctx context.Context, hostsCSV string, keyspace string) (*Storage, error) {
	hosts := splitHosts(hostsCSV)
	cluster := gocql.NewCluster(hosts...)
	cluster.Keyspace = keyspace
	cluster.Consistency = gocql.One
	cluster.Timeout = 10 * time.Second
	cluster.ConnectTimeout = 10 * time.Second

	session, err := cluster.CreateSession()
	if err != nil {
		return nil, err
	}

	return &Storage{session: session, keyspace: keyspace}, nil
}

func splitHosts(hostsCSV string) []string {
	parts := strings.Split(hostsCSV, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return []string{"127.0.0.1:9042"}
	}
	return out
}

func (s *Storage) Close() error {
	if s.session != nil {
		s.session.Close()
	}
	return nil
}

func bucketForEvent(event model.Event) string {
	if event.Timestamp.IsZero() {
		return time.Now().UTC().Format("2006-01-02")
	}
	return event.Timestamp.UTC().Format("2006-01-02")
}

func (s *Storage) InsertEvent(ctx context.Context, event model.Event) error {
	return s.session.Query(`
		INSERT INTO events (
			bucket, timestamp, id, source_type, host, user_name, src_ip, dst_ip, event_code, severity, message, raw
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		bucketForEvent(event),
		event.Timestamp,
		event.ID,
		event.SourceType,
		event.Host,
		event.UserName,
		event.SrcIP,
		event.DstIP,
		event.EventCode,
		event.Severity,
		event.Message,
		event.Raw,
	).WithContext(ctx).Exec()
}

func (s *Storage) InsertEventsBatch(ctx context.Context, events []model.Event) error {
	if len(events) == 0 {
		return nil
	}

	batch := s.session.NewBatch(gocql.UnloggedBatch).WithContext(ctx)
	for _, event := range events {
		batch.Query(`
			INSERT INTO events (
				bucket, timestamp, id, source_type, host, user_name, src_ip, dst_ip, event_code, severity, message, raw
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			bucketForEvent(event),
			event.Timestamp,
			event.ID,
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

	return s.session.ExecuteBatch(batch)
}

func (s *Storage) CountEvents(ctx context.Context) (int64, error) {
	var count int64
	err := s.session.Query(`SELECT COUNT(*) FROM events`).WithContext(ctx).Scan(&count)
	return count, err
}

func (s *Storage) SearchByHost(ctx context.Context, host string, limit int) ([]model.EventQueryResult, error) {
	iter := s.session.Query(`
		SELECT id, timestamp, source_type, host, user_name, src_ip, dst_ip, event_code, severity, message
		FROM events
		WHERE host = ?
		LIMIT ?
	`, host, limit).WithContext(ctx).Iter()
	return scanEventResults(iter)
}

func (s *Storage) SearchByUser(ctx context.Context, userName string, limit int) ([]model.EventQueryResult, error) {
	iter := s.session.Query(`
		SELECT id, timestamp, source_type, host, user_name, src_ip, dst_ip, event_code, severity, message
		FROM events
		WHERE user_name = ?
		LIMIT ?
	`, userName, limit).WithContext(ctx).Iter()
	return scanEventResults(iter)
}

func scanEventResults(iter *gocql.Iter) ([]model.EventQueryResult, error) {
	var results []model.EventQueryResult
	var item model.EventQueryResult

	for iter.Scan(
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
	) {
		results = append(results, item)
		item = model.EventQueryResult{}
	}

	return results, iter.Close()
}

func (s *Storage) CountBySeverity(ctx context.Context) ([]model.SeverityCount, error) {
	iter := s.session.Query(`SELECT severity FROM events`).WithContext(ctx).Iter()

	counts := map[int]int64{}
	var severity int
	for iter.Scan(&severity) {
		counts[severity]++
	}
	if err := iter.Close(); err != nil {
		return nil, err
	}

	severities := make([]int, 0, len(counts))
	for severity := range counts {
		severities = append(severities, severity)
	}
	sort.Ints(severities)

	results := make([]model.SeverityCount, 0, len(severities))
	for _, severity := range severities {
		results = append(results, model.SeverityCount{
			Severity: severity,
			Count:    counts[severity],
		})
	}

	return results, nil
}

func (s *Storage) TopHosts(ctx context.Context, limit int) ([]model.HostCount, error) {
	iter := s.session.Query(`SELECT host FROM events`).WithContext(ctx).Iter()

	counts := map[string]int64{}
	var host string
	for iter.Scan(&host) {
		counts[host]++
	}
	if err := iter.Close(); err != nil {
		return nil, err
	}

	results := make([]model.HostCount, 0, len(counts))
	for host, count := range counts {
		results = append(results, model.HostCount{Host: host, Count: count})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Count == results[j].Count {
			return results[i].Host < results[j].Host
		}
		return results[i].Count > results[j].Count
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}
