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
	"strings"

	"siem-bench/internal/buffer"
	"siem-bench/internal/config"
	"siem-bench/internal/model"
	"siem-bench/internal/reporting"

	chstorage "siem-bench/internal/storage/clickhouse"
	esstorage "siem-bench/internal/storage/elasticsearch"
	pgstorage "siem-bench/internal/storage/postgres"
)

type counter interface {
	CountEvents(ctx context.Context) (int64, error)
}

func getPositiveInt(value string, name string) int {
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		log.Fatalf("invalid %s: %s", name, value)
	}
	return n
}

func waitForDrain(cfg config.Config, db counter, buf *buffer.RedisBuffer) (int64, error) {
	drainTimeoutSec := getPositiveInt(cfg.DrainTimeoutSec, "DRAIN_TIMEOUT_SEC")
	drainPollMs := getPositiveInt(cfg.DrainPollMs, "DRAIN_POLL_MS")
	drainStableChecks := getPositiveInt(cfg.DrainStableChecks, "DRAIN_STABLE_CHECKS")

	deadline := time.Now().Add(time.Duration(drainTimeoutSec) * time.Second)
	ticker := time.NewTicker(time.Duration(drainPollMs) * time.Millisecond)
	defer ticker.Stop()

	var lastDBCount int64 = -1
	stableChecks := 0
	warnedNoGroup := false

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		streamLen, streamErr := buf.StreamLen(ctx)
		pendingCount, pendingErr := buf.PendingCount(ctx, cfg.RedisGroup)
		dbCount, dbErr := db.CountEvents(ctx)

		cancel()

		if streamErr != nil {
			return 0, streamErr
		}

		if pendingErr != nil {
			if strings.Contains(pendingErr.Error(), "NOGROUP") {
				if !warnedNoGroup {
					log.Printf(
						"drain check: consumer group not found yet for stream=%s group=%s; treating pending=0",
						cfg.RedisStream, cfg.RedisGroup,
					)
					warnedNoGroup = true
				}
				pendingCount = 0
			} else {
				return 0, pendingErr
			}
		}

		if dbErr != nil {
			return 0, dbErr
		}

		if dbCount == lastDBCount {
			stableChecks++
		} else {
			stableChecks = 0
			lastDBCount = dbCount
		}

		ready := pendingCount == 0 && stableChecks >= drainStableChecks

		log.Printf(
			"drain check: stream_len=%d pending=%d db_count=%d stable_checks=%d/%d",
			streamLen, pendingCount, dbCount, stableChecks, drainStableChecks,
		)

		if ready {
			return dbCount, nil
		}

		if time.Now().After(deadline) {
			log.Printf(
				"drain timeout reached: stream_len=%d pending=%d db_count=%d stable_checks=%d/%d",
				streamLen, pendingCount, dbCount, stableChecks, drainStableChecks,
			)
			return dbCount, nil
		}

		<-ticker.C
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())

	cfg := config.Load()

	eps := getPositiveInt(cfg.GeneratorEPS, "GENERATOR_EPS")
	batchSize := getPositiveInt(cfg.GeneratorBatch, "GENERATOR_BATCH")
	durationSec := getPositiveInt(cfg.GeneratorSec, "GENERATOR_SEC")

	backend := cfg.IngestBackend
	runScenario := cfg.RunScenario
	if runScenario == "" {
		runScenario = "ingest-only"
	}

	var db counter

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	switch backend {
	case "postgres":
		storage, err := pgstorage.New(ctx, cfg.PostgresDSN)
		if err != nil {
			log.Fatalf("postgres connect failed: %v", err)
		}
		defer storage.Close()
		db = storage

	case "clickhouse":
		storage, err := chstorage.New(ctx, cfg.ClickHouseDSN)
		if err != nil {
			log.Fatalf("clickhouse connect failed: %v", err)
		}
		defer func() {
			if err := storage.Close(); err != nil {
				log.Printf("clickhouse close error: %v", err)
			}
		}()
		db = storage

	case "elasticsearch":
		storage, err := esstorage.New(cfg.ElasticsearchURL)
		if err != nil {
			log.Fatalf("elasticsearch connect failed: %v", err)
		}
		defer func() {
			if err := storage.Close(); err != nil {
				log.Printf("elasticsearch close error: %v", err)
			}
		}()
		db = storage

	default:
		log.Fatalf("unsupported INGEST_BACKEND: %s", backend)
	}

	buf := buffer.NewRedisBuffer(cfg.RedisAddr, cfg.RedisStream)
	ctxPing, cancelPing := context.WithTimeout(context.Background(), 5*time.Second)
	if err := buf.Ping(ctxPing); err != nil {
		cancelPing()
		log.Fatalf("redis ping failed: %v", err)
	}
	cancelPing()

	dbCountBefore, err := db.CountEvents(context.Background())
	if err != nil {
		log.Fatalf("count before failed: %v", err)
	}

	startedAt := time.Now().UTC()
	runID := startedAt.Format("20060102-150405")
	totalEvents := eps * durationSec

	log.Printf(
		"generator started: backend=%s collector=%s eps=%d batch=%d duration=%ds total_events=%d db_before=%d scenario=%s",
		backend, cfg.CollectorURL, eps, batchSize, durationSec, totalEvents, dbCountBefore, runScenario,
	)

	client := &http.Client{Timeout: 10 * time.Second}

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

		log.Printf(
			"progress: sent_events=%d sent_requests=%d failed_requests=%d",
			sentEvents, sentRequests, failedRequests,
		)
	}

	sendFinishedAt := time.Now().UTC()

	streamLenAtSendFinish, _ := buf.StreamLen(context.Background())
	pendingAtSendFinish, _ := buf.PendingCount(context.Background(), cfg.RedisGroup)
	dbCountAtSendFinish, _ := db.CountEvents(context.Background())

	dbCountAfter, err := waitForDrain(cfg, db, buf)
	if err != nil {
		log.Fatalf("wait for drain failed: %v", err)
	}

	finishedAt := time.Now().UTC()

	e2eSnap, queueSnap, err := reporting.FetchWorkerLatencySnapshots(backend)
	if err != nil {
		log.Printf("failed to fetch worker latency snapshots: %v", err)
	}

	sysSnap, err := reporting.FetchSystemMetricsForRun(backend, startedAt, finishedAt)
	if err != nil {
		log.Printf("failed to fetch system metrics: %v", err)
	}

	dbInserted := dbCountAfter - dbCountBefore

	streamLenAtFinish, _ := buf.StreamLen(context.Background())
	pendingAtFinish, _ := buf.PendingCount(context.Background(), cfg.RedisGroup)

	sendElapsedSec := sendFinishedAt.Sub(startedAt).Seconds()
	if sendElapsedSec < 0 {
		sendElapsedSec = 0
	}

	totalElapsedSec := finishedAt.Sub(startedAt).Seconds()
	if totalElapsedSec < 0 {
		totalElapsedSec = 0
	}

	drainWaitSec := finishedAt.Sub(sendFinishedAt).Seconds()
	if drainWaitSec < 0 {
		drainWaitSec = 0
	}

	generatorSentEPS := 0.0
	if sendElapsedSec > 0 {
		generatorSentEPS = float64(sentEvents) / sendElapsedSec
	}

	storageEffectiveEPS := 0.0
	if totalElapsedSec > 0 {
		storageEffectiveEPS = float64(dbInserted) / totalElapsedSec
	}

	lossPercent := 0.0
	dbInserted = dbCountAfter - dbCountBefore
	if sentEvents > 0 {
		lossPercent = (1.0 - float64(dbInserted)/float64(sentEvents)) * 100.0
		if lossPercent < 0 {
			lossPercent = 0
		}
	}

	result := model.RunResult{
		RunID:          runID,
		Backend:        backend,
		SentEvents:     sentEvents,
		SentRequests:   sentRequests,
		FailedRequests: failedRequests,
		DBCountBefore:  dbCountBefore,
		DBCountAfter:   dbCountAfter,
		DBInserted:           dbInserted,
		GeneratorSentEPS:     generatorSentEPS,
		StorageEffectiveEPS:  storageEffectiveEPS,
		SendElapsedSec:       sendElapsedSec,
		TotalElapsedSec:      totalElapsedSec,
		DrainWaitSec:         drainWaitSec,
		LossPercent:          lossPercent,
		StreamLenAtSendFinish: streamLenAtSendFinish,
		PendingAtSendFinish:   pendingAtSendFinish,
		DBCountAtSendFinish:   dbCountAtSendFinish,
		StreamLenAtFinish:     streamLenAtFinish,
		PendingAtFinish:       pendingAtFinish,
		E2ELatencyAvgMs:   e2eSnap.AvgMs,
		E2ELatencyP95Ms:   e2eSnap.P95Ms,
		E2ELatencyP99Ms:   e2eSnap.P99Ms,
		QueueLatencyAvgMs: queueSnap.AvgMs,
		QueueLatencyP95Ms: queueSnap.P95Ms,
		QueueLatencyP99Ms: queueSnap.P99Ms,
		SystemCPUAvgPercent: sysSnap.CPUAvgPercent,
		SystemCPUMaxPercent: sysSnap.CPUMaxPercent,
		SystemMemoryAvgMB:   sysSnap.MemoryAvgMB,
		SystemMemoryMaxMB:   sysSnap.MemoryMaxMB,
		SystemDiskReadMB:    sysSnap.DiskReadMB,
		SystemDiskWriteMB:   sysSnap.DiskWriteMB,
		SystemNetRxMB:       sysSnap.NetRxMB,
		SystemNetTxMB:       sysSnap.NetTxMB,

		Notes:          cfg.RunTag,
		ConfigSnapshot: model.RunConfigSnapshot{
			CollectorURL:    cfg.CollectorURL,
			WorkerBackend:   backend,
			WorkerWriteMode: cfg.WorkerWriteMode,
			RunScenario:     runScenario,
			GeneratorEPS:    eps,
			GeneratorBatch:  batchSize,
			GeneratorSec:    durationSec,
		},
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
	}

	resultPath := ""
	switch runScenario {
	case "mixed":
		resultPath = fmt.Sprintf("results/mixed/ingest-%s-%s.json", backend, runID)
	default:
		resultPath = fmt.Sprintf("results/ingest/run-%s-%s.json", backend, runID)
	}

	if err := model.SaveRunResult(resultPath, result); err != nil {
		log.Printf("failed to save run result: %v", err)
	} else {
		log.Printf("run result saved: %s", resultPath)
	}

	log.Printf(
		"generator finished: backend=%s sent_events=%d sent_requests=%d failed_requests=%d db_before=%d db_after=%d db_inserted=%d generator_sent_eps=%.2f storage_effective_eps=%.2f send_elapsed_sec=%.2f total_elapsed_sec=%.2f drain_wait_sec=%.2f loss_percent=%.2f",
		backend,
		sentEvents,
		sentRequests,
		failedRequests,
		dbCountBefore,
		dbCountAfter,
		dbInserted,
		generatorSentEPS,
		storageEffectiveEPS,
		sendElapsedSec,
		totalElapsedSec,
		drainWaitSec,
		lossPercent,
	)
}