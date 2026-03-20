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

func main() {
	files, err := filepath.Glob("results/*.json")
	if err != nil {
		log.Fatalf("failed to scan results directory: %v", err)
	}

	if len(files) == 0 {
		log.Fatal("no result files found in results/")
	}

	var runs []model.RunResult

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			log.Printf("skip file %s: read error: %v", file, err)
			continue
		}

		var run model.RunResult
		if err := json.Unmarshal(data, &run); err != nil {
			log.Printf("skip file %s: json error: %v", file, err)
			continue
		}

		runs = append(runs, run)
	}

	sort.Slice(runs, func(i, j int) bool {
		if runs[i].Backend == runs[j].Backend {
			if runs[i].EPS == runs[j].EPS {
				return runs[i].StartedAt.Before(runs[j].StartedAt)
			}
			return runs[i].EPS < runs[j].EPS
		}
		return runs[i].Backend < runs[j].Backend
	})

	outFile, err := os.Create("results/summary.csv")
	if err != nil {
		log.Fatalf("failed to create summary.csv: %v", err)
	}
	defer outFile.Close()

	writer := csv.NewWriter(outFile)
	defer writer.Flush()

	header := []string{
		"run_id",
		"backend",
		"eps",
		"batch_size",
		"duration_sec",
		"sent_events",
		"sent_requests",
		"failed_requests",
		"db_count_before",
		"db_count_after",
		"db_inserted",
		"loss",
		"started_at",
		"finished_at",
	}
	if err := writer.Write(header); err != nil {
		log.Fatalf("failed to write csv header: %v", err)
	}

	for _, run := range runs {
		loss := int64(run.SentEvents) - run.DBInserted

		row := []string{
			run.RunID,
			run.Backend,
			strconv.Itoa(run.EPS),
			strconv.Itoa(run.BatchSize),
			strconv.Itoa(run.DurationSec),
			strconv.Itoa(run.SentEvents),
			strconv.Itoa(run.SentRequests),
			strconv.Itoa(run.FailedRequests),
			strconv.FormatInt(run.DBCountBefore, 10),
			strconv.FormatInt(run.DBCountAfter, 10),
			strconv.FormatInt(run.DBInserted, 10),
			strconv.FormatInt(loss, 10),
			run.StartedAt.Format("2006-01-02 15:04:05"),
			run.FinishedAt.Format("2006-01-02 15:04:05"),
		}

		if err := writer.Write(row); err != nil {
			log.Fatalf("failed to write csv row: %v", err)
		}
	}

	log.Printf("summary saved to results/summary.csv (%d runs)", len(runs))
}
