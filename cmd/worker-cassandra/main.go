package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gocql/gocql"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"siem-bench/internal/buffer"
	"siem-bench/internal/config"
	"siem-bench/internal/metrics"
	cassandrastorage "siem-bench/internal/storage/cassandra"
)

func getPositiveInt64(value string, name string) int64 {
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n <= 0 { log.Fatalf("invalid %s: %s", name, value) }
	return n
}

func getEnv(key, fallback string) string {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" { return fallback }
	return val
}

func splitAndTrim(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" { out = append(out, part) }
	}
	return out
}

func parseConsistency(value string) gocql.Consistency {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "any": return gocql.Any
	case "one": return gocql.One
	case "two": return gocql.Two
	case "three": return gocql.Three
	case "quorum": return gocql.Quorum
	case "all": return gocql.All
	case "localquorum": return gocql.LocalQuorum
	case "eachquorum": return gocql.EachQuorum
	case "localone": return gocql.LocalOne
	default: return gocql.One
	}
}

func startMetricsServer(addr string) {
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		log.Printf("worker-cassandra metrics listening on %s", addr)
		if err := http.ListenAndServe(addr, mux); err != nil { log.Fatalf("worker-cassandra metrics server failed: %v", err) }
	}()
}

func main() {
	cfg := config.Load()
	metrics.MustRegister()
	readCount := getPositiveInt64(cfg.WorkerReadCount, "WORKER_READ_COUNT")
	backend := "cassandra"
	scenario := cfg.RunScenario
	if scenario == "" { scenario = "ingest-only" }
	writeMode := "batch"
	startMetricsServer(":2116")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	redisBuffer := buffer.NewRedisBuffer(cfg.RedisAddr, cfg.RedisStream)
	if err := redisBuffer.Ping(ctx); err != nil { log.Fatalf("redis ping failed: %v", err) }
	if err := redisBuffer.EnsureGroup(ctx, cfg.RedisGroup); err != nil { log.Fatalf("ensure redis group failed: %v", err) }
	hosts := splitAndTrim(getEnv("CASSANDRA_HOSTS", "127.0.0.1"))
	if len(hosts) == 0 { hosts = []string{"127.0.0.1"} }
	port, err := strconv.Atoi(getEnv("CASSANDRA_PORT", "9042"))
	if err != nil || port <= 0 { log.Fatalf("invalid CASSANDRA_PORT: %v", err) }
	keyspace := getEnv("CASSANDRA_KEYSPACE", "siem")
	consistency := parseConsistency(getEnv("CASSANDRA_CONSISTENCY", "one"))
	storage, err := cassandrastorage.NewWithRetry(hosts, port, keyspace, consistency, 24, 5*time.Second)
	if err != nil { log.Fatalf("cassandra connect failed: %v", err) }
	defer storage.Close()
	metrics.RunInfo.WithLabelValues(backend, scenario, writeMode, cfg.RedisStream, cfg.RedisGroup).Set(1)
	log.Printf("worker-cassandra started: hosts=%s port=%d keyspace=%s stream=%s group=%s consumer=%s read_count=%d write_mode=%s", strings.Join(hosts, ","), port, keyspace, cfg.RedisStream, cfg.RedisGroup, cfg.RedisConsumer, readCount, writeMode)
	for {
		msgs, err := redisBuffer.ReadGroup(context.Background(), cfg.RedisGroup, cfg.RedisConsumer, readCount)
		if err != nil { metrics.WorkerReadErrorsTotal.WithLabelValues(backend).Inc(); log.Printf("read group error: %v", err); time.Sleep(2*time.Second); continue }
		ctxMetrics, cancelMetrics := context.WithTimeout(context.Background(), 2*time.Second)
		streamLen, errStream := redisBuffer.StreamLen(ctxMetrics)
		pendingCount, errPending := redisBuffer.PendingCount(ctxMetrics, cfg.RedisGroup)
		cancelMetrics()
		if errStream == nil { metrics.WorkerStreamLen.WithLabelValues(backend).Set(float64(streamLen)) }
		if errPending == nil { metrics.WorkerPendingMessages.WithLabelValues(backend).Set(float64(pendingCount)) }
		if len(msgs) == 0 { continue }
		metrics.WorkerMessagesReadTotal.WithLabelValues(backend).Add(float64(len(msgs)))
		events := make([]model.Event, 0, len(msgs))
		ackIDs := make([]string, 0, len(msgs))
		for _, msg := range msgs { events = append(events, msg.Event); ackIDs = append(ackIDs, msg.ID) }
		insertStart := time.Now()
		if err := storage.InsertEventsBatch(context.Background(), events); err != nil { metrics.WorkerInsertErrorsTotal.WithLabelValues(backend).Inc(); log.Printf("batch insert error: %v", err); time.Sleep(time.Second); continue }
		metrics.WorkerEventsStoredTotal.WithLabelValues(backend).Add(float64(len(events)))
		metrics.WorkerInsertDuration.WithLabelValues(backend).Observe(time.Since(insertStart).Seconds())
		metrics.WorkerBatchSize.WithLabelValues(backend).Observe(float64(len(events)))
		now := time.Now().UTC()
		for _, event := range events {
			if !event.GeneratedAt.IsZero() { metrics.WorkerE2ELatency.WithLabelValues(backend).Observe(now.Sub(event.GeneratedAt).Seconds()) }
			if !event.IngestedAt.IsZero() { metrics.WorkerQueueLatency.WithLabelValues(backend).Observe(now.Sub(event.IngestedAt).Seconds()) }
		}
		if err := redisBuffer.Ack(context.Background(), cfg.RedisGroup, ackIDs...); err != nil { metrics.WorkerAckErrorsTotal.WithLabelValues(backend).Inc(); log.Printf("ack error: %v", err); continue }
		log.Printf("batch stored in cassandra: events=%d", len(events))
	}
}
