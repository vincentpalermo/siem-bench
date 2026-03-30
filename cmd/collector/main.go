package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"siem-bench/internal/buffer"
	"siem-bench/internal/config"
	"siem-bench/internal/metrics"
	"siem-bench/internal/model"
)

func main() {
	cfg := config.Load()
	metrics.MustRegister()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	redisBuffer := buffer.NewRedisBuffer(cfg.RedisAddr, cfg.RedisStream)
	if err := redisBuffer.Ping(ctx); err != nil {
		log.Fatalf("redis ping failed: %v", err)
	}

	r := chi.NewRouter()

	r.Handle("/metrics", promhttp.Handler())

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			metrics.CollectorRequestsTotal.WithLabelValues("/health", "GET", "200").Inc()
			metrics.CollectorRequestDuration.WithLabelValues("/health", "GET", "200").Observe(time.Since(start).Seconds())
		}()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
		})
	})

	r.Post("/ingest", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		statusCode := http.StatusAccepted

		defer func() {
			status := strconv.Itoa(statusCode)
			metrics.CollectorRequestsTotal.WithLabelValues("/ingest", "POST", status).Inc()
			metrics.CollectorRequestDuration.WithLabelValues("/ingest", "POST", status).Observe(time.Since(start).Seconds())
		}()

		defer r.Body.Close()

		var raw json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			statusCode = http.StatusBadRequest
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}

		var events []model.Event
		if err := json.Unmarshal(raw, &events); err == nil {
			if len(events) == 0 {
				statusCode = http.StatusBadRequest
				http.Error(w, "empty events array", http.StatusBadRequest)
				return
			}

			ingestedAt := time.Now().UTC()
			for i := range events {
				events[i].IngestedAt = ingestedAt
			}

			if err := redisBuffer.PublishEvents(r.Context(), events); err != nil {
				metrics.CollectorPublishErrorsTotal.Inc()
				statusCode = http.StatusInternalServerError
				http.Error(w, "failed to publish events", http.StatusInternalServerError)
				log.Printf("publish events error: %v", err)
				return
			}

			metrics.CollectorEventsAcceptedTotal.Add(float64(len(events)))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "accepted",
				"count":  len(events),
			})
			return
		}

		var event model.Event
		if err := json.Unmarshal(raw, &event); err == nil {
			event.IngestedAt = time.Now().UTC()

			if err := redisBuffer.PublishEvent(r.Context(), event); err != nil {
				metrics.CollectorPublishErrorsTotal.Inc()
				statusCode = http.StatusInternalServerError
				http.Error(w, "failed to publish event", http.StatusInternalServerError)
				log.Printf("publish event error: %v", err)
				return
			}

			metrics.CollectorEventsAcceptedTotal.Inc()

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "accepted",
				"count":  1,
			})
			return
		}

		statusCode = http.StatusBadRequest
		http.Error(w, "expected JSON object or array of events", http.StatusBadRequest)
	})

	log.Printf("collector listening on %s", cfg.HTTPAddr)
	if err := http.ListenAndServe(cfg.HTTPAddr, r); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}