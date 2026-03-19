package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"siem-bench/internal/config"
	"siem-bench/internal/model"
	"siem-bench/internal/storage/postgres"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	cfg := config.Load()

	eps, err := strconv.Atoi(cfg.GeneratorEPS)
	if err != nil || eps <= 0 {
		log.Fatalf("invalid GENERATOR_EPS: %s", cfg.GeneratorEPS)
	}

	batchSize, err := strconv.Atoi(cfg.GeneratorBatch)
	if err != nil || batchSize <= 0 {
		log.Fatalf("invalid GENERATOR_BATCH: %s", cfg.GeneratorBatch)
	}

	durationSec, err := strconv.Atoi(cfg.GeneratorSec)
	if err != nil || durationSec <= 0 {
		log.Fatalf("invalid GENERATOR_SEC: %s", cfg.GeneratorSec)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	storage, err := postgres.New(ctx, cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("postgres connect failed: %v", err)
	}
	defer storage.Close()

	dbCountBefore, err := storage.CountEvents(context.Background())
	if err != nil {
		log.Fatalf("count before failed: %v", err)
	}

	startedAt := time.Now().UTC()
	runID := startedAt.Format("20060102-150405")

	totalEvents := eps * durationSec
	log.Printf("generator started: collector=%s eps=%d batch=%d duration=%ds total_events=%d db_before=%d",
		cfg.CollectorURL, eps, batchSize, durationSec, totalEvents, dbCountBefore)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	sentEvents := 0
	sentRequests := 0
	failedRequests := 0

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	globalID := 1
	deadline := time.Now().Add(time.Duration(durationSec) * time.Second)

	for time.Now().Before(deadline) {
		<-ticker.C

		eventsThisSecond := make([]model.Event, 0, eps)
		for i := 0; i < eps; i++ {
			eventID := fmt.Sprintf("evt-%s-%d", runID, globalID)
			eventsThisSecond = append(eventsThisSecond, model.GenerateEvent(eventID))
			globalID++
		}

		for start := 0; start < len(eventsThisSecond); start += batchSize {
			end := start + batchSize
			if end > len(eventsThisSecond) {
				end = len(eventsThisSecond)
			}

			batch := eventsThisSecond[start:end]

			body, err := json.Marshal(batch)
			if err != nil {
				log.Printf("marshal batch failed: %v", err)
				failedRequests++
				continue
			}

			req, err := http.NewRequest(http.MethodPost, cfg.CollectorURL, bytes.NewReader(body))
			if err != nil {
				log.Printf("create request failed: %v", err)
				failedRequests++
				continue
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				log.Printf("send batch failed: %v", err)
				failedRequests++
				continue
			}
			_ = resp.Body.Close()

			if resp.StatusCode >= 300 {
				log.Printf("unexpected status code: %d", resp.StatusCode)
				failedRequests++
				continue
			}

			sentRequests++
			sentEvents += len(batch)
		}

		log.Printf("progress: sent_events=%d sent_requests=%d failed_requests=%d",
			sentEvents, sentRequests, failedRequests)
	}

	time.Sleep(2 * time.Second)

	dbCountAfter, err := storage.CountEvents(context.Background())
	if err != nil {
		log.Fatalf("count after failed: %v", err)
	}

	finishedAt := time.Now().UTC()

	result := model.RunResult{
		RunID:          runID,
		Backend:        "postgres",
		EPS:            eps,
		BatchSize:      batchSize,
		DurationSec:    durationSec,
		SentEvents:     sentEvents,
		SentRequests:   sentRequests,
		FailedRequests: failedRequests,
		DBCountBefore:  dbCountBefore,
		DBCountAfter:   dbCountAfter,
		DBInserted:     dbCountAfter - dbCountBefore,
		StartedAt:      startedAt,
		FinishedAt:     finishedAt,
	}

	resultPath := fmt.Sprintf("results/run-%s.json", runID)
	if err := model.SaveRunResult(resultPath, result); err != nil {
		log.Printf("failed to save run result: %v", err)
	} else {
		log.Printf("run result saved: %s", resultPath)
	}

	log.Printf("generator finished: sent_events=%d sent_requests=%d failed_requests=%d db_before=%d db_after=%d db_inserted=%d",
		sentEvents, sentRequests, failedRequests, dbCountBefore, dbCountAfter, dbCountAfter-dbCountBefore)
}
