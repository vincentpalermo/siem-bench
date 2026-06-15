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
func i(v int) string { return strconv.Itoa(v) }
func i64(v int64) string { return strconv.FormatInt(v, 10) }

func main() {
	glob := env("RESULTS_GLOB", "results/ingest/*.json")
	out := env("RESULTS_OUTPUT", "results/ingest/summary.csv")
	files, err := filepath.Glob(glob)
	if err != nil { log.Fatalf("list ingest results: %v", err) }
	if len(files) == 0 { log.Fatalf("no ingest result files found for glob: %s", glob) }

	runs := make([]model.RunResult, 0, len(files))
	for _, p := range files {
		b, err := os.ReadFile(p); if err != nil { log.Printf("skip %s: %v", p, err); continue }
		var r model.RunResult
		if err := json.Unmarshal(b, &r); err != nil { log.Printf("skip %s: %v", p, err); continue }
		runs = append(runs, r)
	}
	sort.Slice(runs, func(a, b int) bool {
		if runs[a].Backend != runs[b].Backend { return runs[a].Backend < runs[b].Backend }
		if runs[a].ConfigSnapshot.RunScenario != runs[b].ConfigSnapshot.RunScenario { return runs[a].ConfigSnapshot.RunScenario < runs[b].ConfigSnapshot.RunScenario }
		if runs[a].ConfigSnapshot.GeneratorEPS != runs[b].ConfigSnapshot.GeneratorEPS { return runs[a].ConfigSnapshot.GeneratorEPS < runs[b].ConfigSnapshot.GeneratorEPS }
		return runs[a].StartedAt.Before(runs[b].StartedAt)
	})

	fp, err := os.Create(out); if err != nil { log.Fatalf("create %s: %v", out, err) }
	defer fp.Close()
	w := csv.NewWriter(fp); defer w.Flush()
	w.Write([]string{"run_id","backend","run_scenario","run_tag","worker_write_mode","generator_eps","generator_batch","generator_sec","sent_events","sent_requests","failed_requests","db_count_before","db_count_after","db_inserted","generator_sent_eps","storage_effective_eps","send_elapsed_sec","total_elapsed_sec","drain_wait_sec","loss_percent","started_at","finished_at","stream_len_at_send_finish","pending_at_send_finish","db_count_at_send_finish","stream_len_at_finish","pending_at_finish","e2e_latency_avg_ms","e2e_latency_p95_ms","e2e_latency_p99_ms","queue_latency_avg_ms","queue_latency_p95_ms","queue_latency_p99_ms","cpu_avg_percent","cpu_max_percent","memory_avg_mb","memory_max_mb","disk_read_mb","disk_write_mb","net_rx_mb","net_tx_mb"})
	for _, r := range runs {
		c := r.ConfigSnapshot
		w.Write([]string{r.RunID,r.Backend,c.RunScenario,r.Notes,c.WorkerWriteMode,i(c.GeneratorEPS),i(c.GeneratorBatch),i(c.GeneratorSec),i(r.SentEvents),i(r.SentRequests),i(r.FailedRequests),i64(r.DBCountBefore),i64(r.DBCountAfter),i64(r.DBInserted),f64(r.GeneratorSentEPS),f64(r.StorageEffectiveEPS),f64(r.SendElapsedSec),f64(r.TotalElapsedSec),f64(r.DrainWaitSec),f64(r.LossPercent),r.StartedAt.Format("2006-01-02T15:04:05Z07:00"),r.FinishedAt.Format("2006-01-02T15:04:05Z07:00"),i64(r.StreamLenAtSendFinish),i64(r.PendingAtSendFinish),i64(r.DBCountAtSendFinish),i64(r.StreamLenAtFinish),i64(r.PendingAtFinish),f64(r.E2ELatencyAvgMs),f64(r.E2ELatencyP95Ms),f64(r.E2ELatencyP99Ms),f64(r.QueueLatencyAvgMs),f64(r.QueueLatencyP95Ms),f64(r.QueueLatencyP99Ms),f64(r.SystemCPUAvgPercent),f64(r.SystemCPUMaxPercent),f64(r.SystemMemoryAvgMB),f64(r.SystemMemoryMaxMB),f64(r.SystemDiskReadMB),f64(r.SystemDiskWriteMB),f64(r.SystemNetRxMB),f64(r.SystemNetTxMB)})
	}
	log.Printf("summary written: %d runs -> %s", len(runs), out)
}
