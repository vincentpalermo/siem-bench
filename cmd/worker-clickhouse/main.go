package main

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"siem-bench/internal/buffer"
	"siem-bench/internal/config"
	"siem-bench/internal/metrics"
	"siem-bench/internal/model"
	chstorage "siem-bench/internal/storage/clickhouse"
)

func getPositiveInt64(value string, name string) int64 {
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n <= 0 {
		log.Fatalf("invalid %s: %s", name, value)
	}
	return n
}

func startMetricsServer(addr string, name string) {
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())

		log.Printf("%s metrics listening on %s", name, addr)
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Fatalf("%s metrics server failed: %v", name, err)
		}
	}()
}

func main() {
	cfg := config.Load()
	metrics.MustRegister()

	readCount := getPositiveInt64(cfg.WorkerReadCount, "WORKER_READ_COUNT")
	writeMode := cfg.WorkerWriteMode
	if writeMode != "batch" {
		log.Printf("WORKER_WRITE_MODE=%s is not supported for clickhouse yet, forcing batch mode", writeMode)
		writeMode = "batch"
	}

	backend := "clickhouse"
	scenario := cfg.RunScenario
	if scenario == "" {
		scenario = "ingest-only"
	}

	startMetricsServer(":2113", "worker-clickhouse")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	redisBuffer := buffer.NewRedisBuffer(cfg.RedisAddr, cfg.RedisStream)
	if err := redisBuffer.Ping(ctx); err != nil {
		log.Fatalf("redis ping failed: %v", err)
	}
	if err := redisBuffer.EnsureGroup(ctx, cfg.RedisGroup); err != nil {
		log.Fatalf("ensure redis group failed: %v", err)
	}

	storage, err := chstorage.New(ctx, cfg.ClickHouseDSN)
	if err != nil {
		log.Fatalf("clickhouse connect failed: %v", err)
	}
	defer func() {
		if err := storage.Close(); err != nil {
			log.Printf("clickhouse close error: %v", err)
		}
	}()

	metrics.RunInfo.WithLabelValues(
		backend,
		scenario,
		writeMode,
		cfg.RedisStream,
		cfg.RedisGroup,
	).Set(1)

	log.Printf(
		"worker-clickhouse started: stream=%s group=%s consumer=%s read_count=%d write_mode=%s",
		cfg.RedisStream, cfg.RedisGroup, cfg.RedisConsumer, readCount, writeMode,
	)

	for {
		msgs, err := redisBuffer.ReadGroup(context.Background(), cfg.RedisGroup, cfg.RedisConsumer, readCount)
		if err != nil {
			metrics.WorkerReadErrorsTotal.WithLabelValues(backend).Inc()
			log.Printf("read group error: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		ctxMetrics, cancelMetrics := context.WithTimeout(context.Background(), 2*time.Second)
		streamLen, errStream := redisBuffer.StreamLen(ctxMetrics)
		pendingCount, errPending := redisBuffer.PendingCount(ctxMetrics, cfg.RedisGroup)
		cancelMetrics()

		if errStream == nil {
			metrics.WorkerStreamLen.WithLabelValues(backend).Set(float64(streamLen))
		}
		if errPending == nil {
			metrics.WorkerPendingMessages.WithLabelValues(backend).Set(float64(pendingCount))
		}

		if len(msgs) == 0 {
			continue
		}

		metrics.WorkerMessagesReadTotal.WithLabelValues(backend).Add(float64(len(msgs)))

		events := make([]model.Event, 0, len(msgs))
		ackIDs := make([]string, 0, len(msgs))
		for _, msg := range msgs {
			events = append(events, msg.Event)
			ackIDs = append(ackIDs, msg.ID)
		}

		insertStart := time.Now()
		if err := storage.InsertEventsBatch(context.Background(), events); err != nil {
			metrics.WorkerInsertErrorsTotal.WithLabelValues(backend).Inc()
			log.Printf("batch insert error: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		metrics.WorkerEventsStoredTotal.WithLabelValues(backend).Add(float64(len(events)))
		metrics.WorkerInsertDuration.WithLabelValues(backend).Observe(time.Since(insertStart).Seconds())
		metrics.WorkerBatchSize.WithLabelValues(backend).Observe(float64(len(events)))

		now := time.Now().UTC()
		for _, event := range events {
			if !event.GeneratedAt.IsZero() {
				metrics.WorkerE2ELatency.WithLabelValues(backend).Observe(now.Sub(event.GeneratedAt).Seconds())
			}
			if !event.IngestedAt.IsZero() {
				metrics.WorkerQueueLatency.WithLabelValues(backend).Observe(now.Sub(event.IngestedAt).Seconds())
			}
		}

		if err := redisBuffer.Ack(context.Background(), cfg.RedisGroup, ackIDs...); err != nil {
			metrics.WorkerAckErrorsTotal.WithLabelValues(backend).Inc()
			log.Printf("ack error: %v", err)
			continue
		}

		log.Printf("batch stored in clickhouse: events=%d", len(events))
	}
}