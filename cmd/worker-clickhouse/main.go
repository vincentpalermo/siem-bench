package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"siem-bench/internal/buffer"
	"siem-bench/internal/config"
	"siem-bench/internal/metrics"
	"siem-bench/internal/model"
	chstorage "siem-bench/internal/storage/clickhouse"
)

func main() {
	cfg := config.Load()
	metrics.MustRegister()

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())

		log.Printf("worker-clickhouse metrics listening on :2113")
		if err := http.ListenAndServe(":2113", mux); err != nil {
			log.Fatalf("worker-clickhouse metrics server failed: %v", err)
		}
	}()

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

	log.Printf("worker-clickhouse started: stream=%s group=%s consumer=%s", cfg.RedisStream, cfg.RedisGroup, cfg.RedisConsumer)

	for {
		msgs, err := redisBuffer.ReadGroup(context.Background(), cfg.RedisGroup, cfg.RedisConsumer, 100)
		if err != nil {
			metrics.WorkerReadErrorsTotal.Inc()
			log.Printf("read group error: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		if len(msgs) == 0 {
			continue
		}

		metrics.WorkerBatchSize.Observe(float64(len(msgs)))
		metrics.WorkerMessagesReadTotal.Add(float64(len(msgs)))

		events := make([]model.Event, 0, len(msgs))
		ackIDs := make([]string, 0, len(msgs))

		for _, msg := range msgs {
			events = append(events, msg.Event)
			ackIDs = append(ackIDs, msg.ID)
		}

		insertStart := time.Now()

		if err := storage.InsertEventsBatch(context.Background(), events); err != nil {
			metrics.WorkerInsertErrorsTotal.Inc()
			log.Printf("batch insert error: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		metrics.WorkerEventsStoredTotal.Add(float64(len(events)))
		metrics.WorkerInsertDuration.Observe(time.Since(insertStart).Seconds())

		if err := redisBuffer.Ack(context.Background(), cfg.RedisGroup, ackIDs...); err != nil {
			metrics.WorkerAckErrorsTotal.Inc()
			log.Printf("ack error: %v", err)
			continue
		}

		log.Printf("batch stored in clickhouse: events=%d", len(events))
	}
}
