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

func env(k, d string) string { if v := os.Getenv(k); v != "" { return v }; return d }
func f64(v float64) string { return strconv.FormatFloat(v, 'f', 4, 64) }

func main() {
	glob := env("RESULTS_GLOB", "results/query/query-*.json")
	out := env("RESULTS_OUTPUT", "results/query/summary.csv")
	files, err := filepath.Glob(glob)
	if err != nil { log.Fatalf("list query results: %v", err) }
	if len(files) == 0 { log.Fatalf("no query result files found for glob: %s", glob) }

	runs := make([]model.QueryRunResult, 0, len(files))
	for _, p := range files {
		b, err := os.ReadFile(p); if err != nil { log.Printf("skip %s: %v", p, err); continue }
		var r model.QueryRunResult
		if err := json.Unmarshal(b, &r); err != nil { log.Printf("skip %s: %v", p, err); continue }
		runs = append(runs, r)
	}
	sort.Slice(runs, func(i, j int) bool {
		if runs[i].Backend != runs[j].Backend { return runs[i].Backend < runs[j].Backend }
		if runs[i].ConfigSnapshot.RunScenario != runs[j].ConfigSnapshot.RunScenario { return runs[i].ConfigSnapshot.RunScenario < runs[j].ConfigSnapshot.RunScenario }
		if runs[i].ConfigSnapshot.Concurrency != runs[j].ConfigSnapshot.Concurrency { return runs[i].ConfigSnapshot.Concurrency < runs[j].ConfigSnapshot.Concurrency }
		return runs[i].StartedAt.Before(runs[j].StartedAt)
	})

	fp, err := os.Create(out); if err != nil { log.Fatalf("create %s: %v", out, err) }
	defer fp.Close()
	w := csv.NewWriter(fp); defer w.Flush()
	w.Write([]string{"run_id","backend","run_scenario","duration_sec","interval_sec","warmup_sec","concurrency","workload_name","workload_path","total_queries","failed_queries","started_at","finished_at","cpu_avg_percent","cpu_max_percent","memory_avg_mb","memory_max_mb","disk_read_mb","disk_write_mb","net_rx_mb","net_tx_mb"})
	for _, r := range runs {
		c := r.ConfigSnapshot
		w.Write([]string{r.RunID,r.Backend,c.RunScenario,strconv.Itoa(c.DurationSec),strconv.Itoa(c.IntervalSec),strconv.Itoa(c.WarmupSec),strconv.Itoa(c.Concurrency),c.WorkloadName,c.WorkloadPath,strconv.Itoa(r.TotalQueries),strconv.Itoa(r.FailedQueries),r.StartedAt.Format("2006-01-02T15:04:05Z07:00"),r.FinishedAt.Format("2006-01-02T15:04:05Z07:00"),f64(r.SystemCPUAvgPercent),f64(r.SystemCPUMaxPercent),f64(r.SystemMemoryAvgMB),f64(r.SystemMemoryMaxMB),f64(r.SystemDiskReadMB),f64(r.SystemDiskWriteMB),f64(r.SystemNetRxMB),f64(r.SystemNetTxMB)})
	}
	log.Printf("query summary written: %d runs -> %s", len(runs), out)
}
