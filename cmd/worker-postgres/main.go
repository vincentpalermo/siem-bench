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
	"siem-bench/internal/storage/postgres"
)

func main() {
	cfg := config.Load()
	metrics.MustRegister()

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())

		log.Printf("worker metrics listening on :2112")
		if err := http.ListenAndServe(":2112", mux); err != nil {
			log.Fatalf("worker metrics server failed: %v", err)
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

	storage, err := postgres.New(ctx, cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("postgres connect failed: %v", err)
	}
	defer storage.Close()

	log.Printf("worker started: stream=%s group=%s consumer=%s", cfg.RedisStream, cfg.RedisGroup, cfg.RedisConsumer)

	for {
		msgs, err := redisBuffer.ReadGroup(context.Background(), cfg.RedisGroup, cfg.RedisConsumer, 10)
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

		var ackIDs []string

		for _, msg := range msgs {
			insertStart := time.Now()

			if err := storage.InsertEvent(context.Background(), msg.Event); err != nil {
				metrics.WorkerInsertErrorsTotal.Inc()
				log.Printf("insert event error (id=%s): %v", msg.Event.ID, err)
				continue
			}

			metrics.WorkerEventsStoredTotal.Inc()
			metrics.WorkerInsertDuration.Observe(time.Since(insertStart).Seconds())

			ackIDs = append(ackIDs, msg.ID)
			log.Printf("event stored: id=%s stream_id=%s", msg.Event.ID, msg.ID)
		}

		if err := redisBuffer.Ack(context.Background(), cfg.RedisGroup, ackIDs...); err != nil {
			metrics.WorkerAckErrorsTotal.Inc()
			log.Printf("ack error: %v", err)
		}
	}
}
