package cassandra

import (
	"context"
	"sort"
	"time"

	"github.com/gocql/gocql"
	"siem-bench/internal/model"
)

type Storage struct { session *gocql.Session }

func New(host string) (*Storage, error) {
	return NewWithOptions([]string{host}, 9042, "siem", gocql.One)
}

func NewWithOptions(hosts []string, port int, keyspace string, consistency gocql.Consistency) (*Storage, error) {
	cluster := gocql.NewCluster(hosts...)
	cluster.Port = port
	cluster.Keyspace = keyspace
	cluster.Consistency = consistency
	cluster.Timeout = 10 * time.Second
	cluster.ConnectTimeout = 10 * time.Second
	cluster.ProtoVersion = 4
	session, err := cluster.CreateSession()
	if err != nil { return nil, err }
	return &Storage{session: session}, nil
}

func NewWithRetry(hosts []string, port int, keyspace string, consistency gocql.Consistency, attempts int, delay time.Duration) (*Storage, error) {
	var lastErr error
	for i := 0; i < attempts; i++ {
		s, err := NewWithOptions(hosts, port, keyspace, consistency)
		if err == nil { return s, nil }
		lastErr = err
		if i+1 < attempts { time.Sleep(delay) }
	}
	return nil, lastErr
}

func (s *Storage) Close() {
	if s.session != nil { s.session.Close() }
}

func bucketForEvent(event model.Event) string {
	ts := event.Timestamp
	if ts.IsZero() { ts = time.Now().UTC() }
	return ts.UTC().Format("2006-01-02")
}

func normalizeEvent(e model.Event) (model.Event, time.Time) {
	ts := e.Timestamp
	if ts.IsZero() { ts = time.Now().UTC() }
	e.Timestamp = ts
	return e, ts
}

func (s *Storage) InsertEventsBatch(ctx context.Context, events []model.Event) error {
	if len(events) == 0 { return nil }
	eventsBatch := s.session.NewBatch(gocql.UnloggedBatch).WithContext(ctx)
	counterBatch := s.session.NewBatch(gocql.CounterBatch).WithContext(ctx)
	for _, event := range events {
		event, ts := normalizeEvent(event)
		bucket := bucketForEvent(event)
		eventsBatch.Query(`INSERT INTO events_by_day (bucket, ts, id, source_type, host, user_name, src_ip, dst_ip, event_code, severity, message, raw) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, bucket, ts, event.ID, event.SourceType, event.Host, event.UserName, event.SrcIP, event.DstIP, event.EventCode, event.Severity, event.Message, event.Raw)
		eventsBatch.Query(`INSERT INTO events_by_host (host, ts, id, source_type, user_name, src_ip, dst_ip, event_code, severity, message, raw) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, event.Host, ts, event.ID, event.SourceType, event.UserName, event.SrcIP, event.DstIP, event.EventCode, event.Severity, event.Message, event.Raw)
		eventsBatch.Query(`INSERT INTO events_by_user (user_name, ts, id, source_type, host, src_ip, dst_ip, event_code, severity, message, raw) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, event.UserName, ts, event.ID, event.SourceType, event.Host, event.SrcIP, event.DstIP, event.EventCode, event.Severity, event.Message, event.Raw)
		counterBatch.Query(`UPDATE severity_counts SET cnt = cnt + 1 WHERE severity = ?`, event.Severity)
		counterBatch.Query(`UPDATE host_counts SET cnt = cnt + 1 WHERE host = ?`, event.Host)
	}
	if err := s.session.ExecuteBatch(eventsBatch); err != nil { return err }
	return s.session.ExecuteBatch(counterBatch)
}

func (s *Storage) CountEvents(ctx context.Context) (int64, error) {
	iter := s.session.Query(`SELECT severity, cnt FROM severity_counts`).WithContext(ctx).Iter()
	var severity int
	var count int64
	var total int64
	for iter.Scan(&severity, &count) { total += count }
	return total, iter.Close()
}

func (s *Storage) SearchByHost(ctx context.Context, host string, limit int) ([]model.EventQueryResult, error) {
	iter := s.session.Query(`SELECT id, ts, source_type, host, user_name, src_ip, dst_ip, event_code, severity, message FROM events_by_host WHERE host = ? LIMIT ?`, host, limit).WithContext(ctx).Iter()
	var out []model.EventQueryResult
	for {
		var item model.EventQueryResult
		if !iter.Scan(&item.ID, &item.Timestamp, &item.SourceType, &item.Host, &item.UserName, &item.SrcIP, &item.DstIP, &item.EventCode, &item.Severity, &item.Message) { break }
		out = append(out, item)
	}
	return out, iter.Close()
}

func (s *Storage) SearchByUser(ctx context.Context, userName string, limit int) ([]model.EventQueryResult, error) {
	iter := s.session.Query(`SELECT id, ts, source_type, host, user_name, src_ip, dst_ip, event_code, severity, message FROM events_by_user WHERE user_name = ? LIMIT ?`, userName, limit).WithContext(ctx).Iter()
	var out []model.EventQueryResult
	for {
		var item model.EventQueryResult
		if !iter.Scan(&item.ID, &item.Timestamp, &item.SourceType, &item.Host, &item.UserName, &item.SrcIP, &item.DstIP, &item.EventCode, &item.Severity, &item.Message) { break }
		out = append(out, item)
	}
	return out, iter.Close()
}

func (s *Storage) CountBySeverity(ctx context.Context) ([]model.SeverityCount, error) {
	iter := s.session.Query(`SELECT severity, cnt FROM severity_counts`).WithContext(ctx).Iter()
	var out []model.SeverityCount
	for {
		var item model.SeverityCount
		if !iter.Scan(&item.Severity, &item.Count) { break }
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Severity < out[j].Severity })
	return out, iter.Close()
}

func (s *Storage) TopHosts(ctx context.Context, limit int) ([]model.HostCount, error) {
	iter := s.session.Query(`SELECT host, cnt FROM host_counts`).WithContext(ctx).Iter()
	var out []model.HostCount
	for {
		var item model.HostCount
		if !iter.Scan(&item.Host, &item.Count) { break }
		out = append(out, item)
	}
	if err := iter.Close(); err != nil { return nil, err }
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	if limit > 0 && len(out) > limit { out = out[:limit] }
	return out, nil
}
