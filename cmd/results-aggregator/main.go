package main

import (
	"encoding/csv"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"siem-bench/internal/model"
)

func getEnv(key, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}

func main() {
	resultsGlob := getEnv("RESULTS_GLOB", "results/ingest/*.json")
	files, err := filepath.Glob(resultsGlob)
	if err != nil {
		log.Fatalf("failed to list result files: %v", err)
	}
	if len(files) == 0 {
		log.Fatal("no ingest result files found in results/ingest/")
	}

	var runs []model.RunResult

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			log.Printf("skip %s: read error: %v", f, err)
			continue
		}

		var run model.RunResult
		if err := json.Unmarshal(data, &run); err != nil {
			log.Printf("skip %s: unmarshal error: %v", f, err)
			continue
		}

		runs = append(runs, run)
	}

	sort.Slice(runs, func(i, j int) bool {
		if runs[i].Backend != runs[j].Backend {
			return runs[i].Backend < runs[j].Backend
		}
		if runs[i].ConfigSnapshot.GeneratorEPS != runs[j].ConfigSnapshot.GeneratorEPS {
			return runs[i].ConfigSnapshot.GeneratorEPS < runs[j].ConfigSnapshot.GeneratorEPS
		}
		return runs[i].StartedAt.Before(runs[j].StartedAt)
	})

	outputPath := getEnv("RESULTS_OUTPUT", "results/ingest/summary.csv")
	outFile, err := os.Create(outputPath)
	if err != nil {
		log.Fatalf("failed to create results/ingest/summary.csv: %v", err)
	}
	defer outFile.Close()

	w := csv.NewWriter(outFile)
	defer w.Flush()

	header := []string{
		"run_id",
		"backend",
		"worker_write_mode",
		"generator_eps",
		"generator_batch",
		"generator_sec",
		"sent_events",
		"sent_requests",
		"failed_requests",
		"db_count_before",
		"db_count_after",
		"db_inserted",
		"generator_sent_eps",
		"storage_effective_eps",
		"send_elapsed_sec",
		"total_elapsed_sec",
		"drain_wait_sec",
		"loss_percent",
		"started_at",
		"finished_at",
	}
	if err := w.Write(header); err != nil {
		log.Fatalf("failed to write CSV header: %v", err)
	}

	for _, run := range runs {
		row := []string{
			run.RunID,
			run.Backend,
			run.ConfigSnapshot.WorkerWriteMode,
			strconv.Itoa(run.ConfigSnapshot.GeneratorEPS),
			strconv.Itoa(run.ConfigSnapshot.GeneratorBatch),
			strconv.Itoa(run.ConfigSnapshot.GeneratorSec),
			strconv.Itoa(run.SentEvents),
			strconv.Itoa(run.SentRequests),
			strconv.Itoa(run.FailedRequests),
			strconv.FormatInt(run.DBCountBefore, 10),
			strconv.FormatInt(run.DBCountAfter, 10),
			strconv.FormatInt(run.DBInserted, 10),
			strconv.FormatFloat(run.GeneratorSentEPS, 'f', 4, 64),
			strconv.FormatFloat(run.StorageEffectiveEPS, 'f', 4, 64),
			strconv.FormatFloat(run.SendElapsedSec, 'f', 4, 64),
			strconv.FormatFloat(run.TotalElapsedSec, 'f', 4, 64),
			strconv.FormatFloat(run.DrainWaitSec, 'f', 4, 64),
			strconv.FormatFloat(run.LossPercent, 'f', 4, 64),
			run.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
			run.FinishedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if err := w.Write(row); err != nil {
			log.Fatalf("failed to write CSV row: %v", err)
		}
	}

	log.Printf("summary written: %d runs -> %s", len(runs), outputPath)
}