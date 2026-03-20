package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"siem-bench/internal/config"
	"siem-bench/internal/metrics"
	chstorage "siem-bench/internal/storage/clickhouse"
	pgstorage "siem-bench/internal/storage/postgres"
)

func recordQuery(backend, query string, started time.Time, err error) {
	status := "ok"
	if err != nil {
		status = "error"
	}
	metrics.QueryRequestsTotal.WithLabelValues(backend, query, status).Inc()
	metrics.QueryDuration.WithLabelValues(backend, query).Observe(time.Since(started).Seconds())
}

func runPostgresQueries(ctx context.Context, storage *pgstorage.Storage) {
	start := time.Now()
	_, err := storage.SearchByHost(ctx, "host-1", 100)
	recordQuery("postgres", "search_by_host", start, err)
	if err != nil {
		log.Printf("postgres search_by_host error: %v", err)
	}

	start = time.Now()
	_, err = storage.SearchByUser(ctx, "admin", 100)
	recordQuery("postgres", "search_by_user", start, err)
	if err != nil {
		log.Printf("postgres search_by_user error: %v", err)
	}

	start = time.Now()
	_, err = storage.CountBySeverity(ctx)
	recordQuery("postgres", "count_by_severity", start, err)
	if err != nil {
		log.Printf("postgres count_by_severity error: %v", err)
	}

	start = time.Now()
	_, err = storage.TopHosts(ctx, 10)
	recordQuery("postgres", "top_hosts", start, err)
	if err != nil {
		log.Printf("postgres top_hosts error: %v", err)
	}
}

func runClickHouseQueries(ctx context.Context, storage *chstorage.Storage) {
	start := time.Now()
	_, err := storage.SearchByHost(ctx, "host-1", 100)
	recordQuery("clickhouse", "search_by_host", start, err)
	if err != nil {
		log.Printf("clickhouse search_by_host error: %v", err)
	}

	start = time.Now()
	_, err = storage.SearchByUser(ctx, "admin", 100)
	recordQuery("clickhouse", "search_by_user", start, err)
	if err != nil {
		log.Printf("clickhouse search_by_user error: %v", err)
	}

	start = time.Now()
	_, err = storage.CountBySeverity(ctx)
	recordQuery("clickhouse", "count_by_severity", start, err)
	if err != nil {
		log.Printf("clickhouse count_by_severity error: %v", err)
	}

	start = time.Now()
	_, err = storage.TopHosts(ctx, 10)
	recordQuery("clickhouse", "top_hosts", start, err)
	if err != nil {
		log.Printf("clickhouse top_hosts error: %v", err)
	}
}

func main() {
	cfg := config.Load()
	metrics.MustRegister()

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())

		log.Printf("query-runner metrics listening on :2114")
		if err := http.ListenAndServe(":2114", mux); err != nil {
			log.Fatalf("query-runner metrics server failed: %v", err)
		}
	}()

	backend := cfg.GeneratorBackend
	log.Printf("query-runner started for backend=%s", backend)

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)

		switch backend {
		case "postgres":
			storage, err := pgstorage.New(ctx, cfg.PostgresDSN)
			if err != nil {
				cancel()
				log.Printf("postgres connect failed: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}
			runPostgresQueries(context.Background(), storage)
			storage.Close()

		case "clickhouse":
			storage, err := chstorage.New(ctx, cfg.ClickHouseDSN)
			if err != nil {
				cancel()
				log.Printf("clickhouse connect failed: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}
			runClickHouseQueries(context.Background(), storage)
			_ = storage.Close()

		default:
			cancel()
			log.Fatalf("unsupported GENERATOR_BACKEND: %s", backend)
		}

		cancel()
		time.Sleep(5 * time.Second)
	}
}
