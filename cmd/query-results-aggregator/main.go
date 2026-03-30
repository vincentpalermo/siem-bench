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
	resultsGlob := getEnv("RESULTS_GLOB", "results/query/query-*.json")
	outputPath := getEnv("RESULTS_OUTPUT", "results/query/summary.csv")

	files, err := filepath.Glob(resultsGlob)
	if err != nil {
		log.Fatalf("failed to list query result files: %v", err)
	}
	if len(files) == 0 {
		log.Fatalf("no query result files found for glob: %s", resultsGlob)
	}

	var runs []model.QueryRunResult

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			log.Printf("skip %s: read error: %v", f, err)
			continue
		}

		var run model.QueryRunResult
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
		if runs[i].ConfigSnapshot.Concurrency != runs[j].ConfigSnapshot.Concurrency {
			return runs[i].ConfigSnapshot.Concurrency < runs[j].ConfigSnapshot.Concurrency
		}
		return runs[i].StartedAt.Before(runs[j].StartedAt)
	})

	outFile, err := os.Create(outputPath)
	if err != nil {
		log.Fatalf("failed to create %s: %v", outputPath, err)
	}
	defer outFile.Close()

	w := csv.NewWriter(outFile)
	defer w.Flush()

	header := []string{
		"run_id",
		"backend",
		"duration_sec",
		"interval_sec",
		"warmup_sec",
		"concurrency",
		"run_scenario",
		"workload_name",
		"workload_path",
		"total_queries",
		"failed_queries",
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
			strconv.Itoa(run.ConfigSnapshot.DurationSec),
			strconv.Itoa(run.ConfigSnapshot.IntervalSec),
			strconv.Itoa(run.ConfigSnapshot.WarmupSec),
			strconv.Itoa(run.ConfigSnapshot.Concurrency),
			run.ConfigSnapshot.RunScenario,
			run.ConfigSnapshot.WorkloadName,
			run.ConfigSnapshot.WorkloadPath,
			strconv.Itoa(run.TotalQueries),
			strconv.Itoa(run.FailedQueries),
			run.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
			run.FinishedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if err := w.Write(row); err != nil {
			log.Fatalf("failed to write CSV row: %v", err)
		}
	}

	log.Printf("query summary written: %d runs -> %s", len(runs), outputPath)
}