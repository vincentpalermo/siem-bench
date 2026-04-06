package reporting

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type SystemMetricsSnapshot struct {
	CPUAvgPercent float64
	CPUMaxPercent float64

	MemoryAvgMB float64
	MemoryMaxMB float64

	DiskReadMB  float64
	DiskWriteMB float64

	NetRxMB float64
	NetTxMB float64
}

type promQueryRangeResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Values [][]interface{}   `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

func backendContainerName(backend string) (string, error) {
	switch backend {
	case "postgres":
		return "siem-postgres", nil
	case "clickhouse":
		return "siem-clickhouse", nil
	case "elasticsearch":
		return "siem-elasticsearch", nil
	default:
		return "", fmt.Errorf("unsupported backend: %s", backend)
	}
}

func FetchSystemMetricsForRun(backend string, startedAt, finishedAt time.Time) (SystemMetricsSnapshot, error) {
	containerName, err := backendContainerName(backend)
	if err != nil {
		return SystemMetricsSnapshot{}, err
	}

	if !finishedAt.After(startedAt) {
		return SystemMetricsSnapshot{}, fmt.Errorf("invalid time window: finishedAt <= startedAt")
	}

	startUnix := startedAt.Unix()
	endUnix := finishedAt.Unix()

	// Step size for range queries.
	stepSec := 5

	// NOTE:
	// We intentionally use container label matching by name.
	// Depending on environment, cadvisor may expose name as /siem-postgres.
	// To be robust, we match with regex on either exact or prefixed slash form.
	containerRegex := fmt.Sprintf("(/)?%s", regexpEscape(containerName))

	cpuAvgQuery := fmt.Sprintf(
		`avg(rate(container_cpu_usage_seconds_total{name=~"%s"}[30s])) * 100`,
		containerRegex,
	)
	cpuMaxQuery := fmt.Sprintf(
		`max(rate(container_cpu_usage_seconds_total{name=~"%s"}[30s])) * 100`,
		containerRegex,
	)

	memAvgQuery := fmt.Sprintf(
		`avg(container_memory_usage_bytes{name=~"%s"}) / 1024 / 1024`,
		containerRegex,
	)
	memMaxQuery := fmt.Sprintf(
		`max(container_memory_usage_bytes{name=~"%s"}) / 1024 / 1024`,
		containerRegex,
	)

	diskReadQuery := fmt.Sprintf(
		`max(container_fs_reads_bytes_total{name=~"%s"}) - min(container_fs_reads_bytes_total{name=~"%s"})`,
		containerRegex, containerRegex,
	)
	diskWriteQuery := fmt.Sprintf(
		`max(container_fs_writes_bytes_total{name=~"%s"}) - min(container_fs_writes_bytes_total{name=~"%s"})`,
		containerRegex, containerRegex,
	)

	netRxQuery := fmt.Sprintf(
		`max(container_network_receive_bytes_total{name=~"%s"}) - min(container_network_receive_bytes_total{name=~"%s"})`,
		containerRegex, containerRegex,
	)
	netTxQuery := fmt.Sprintf(
		`max(container_network_transmit_bytes_total{name=~"%s"}) - min(container_network_transmit_bytes_total{name=~"%s"})`,
		containerRegex, containerRegex,
	)

	cpuAvg, err := queryRangeMean(cpuAvgQuery, startUnix, endUnix, stepSec)
	if err != nil {
		return SystemMetricsSnapshot{}, fmt.Errorf("cpu avg query failed: %w", err)
	}

	cpuMax, err := queryRangeMax(cpuMaxQuery, startUnix, endUnix, stepSec)
	if err != nil {
		return SystemMetricsSnapshot{}, fmt.Errorf("cpu max query failed: %w", err)
	}

	memAvg, err := queryRangeMean(memAvgQuery, startUnix, endUnix, stepSec)
	if err != nil {
		return SystemMetricsSnapshot{}, fmt.Errorf("memory avg query failed: %w", err)
	}

	memMax, err := queryRangeMax(memMaxQuery, startUnix, endUnix, stepSec)
	if err != nil {
		return SystemMetricsSnapshot{}, fmt.Errorf("memory max query failed: %w", err)
	}

	diskReadBytes, err := queryRangeMax(diskReadQuery, startUnix, endUnix, stepSec)
	if err != nil {
		return SystemMetricsSnapshot{}, fmt.Errorf("disk read query failed: %w", err)
	}

	diskWriteBytes, err := queryRangeMax(diskWriteQuery, startUnix, endUnix, stepSec)
	if err != nil {
		return SystemMetricsSnapshot{}, fmt.Errorf("disk write query failed: %w", err)
	}

	netRxBytes, err := queryRangeMax(netRxQuery, startUnix, endUnix, stepSec)
	if err != nil {
		return SystemMetricsSnapshot{}, fmt.Errorf("net rx query failed: %w", err)
	}

	netTxBytes, err := queryRangeMax(netTxQuery, startUnix, endUnix, stepSec)
	if err != nil {
		return SystemMetricsSnapshot{}, fmt.Errorf("net tx query failed: %w", err)
	}

	return SystemMetricsSnapshot{
		CPUAvgPercent: cpuAvg,
		CPUMaxPercent: cpuMax,
		MemoryAvgMB:   memAvg,
		MemoryMaxMB:   memMax,
		DiskReadMB:    diskReadBytes / 1024.0 / 1024.0,
		DiskWriteMB:   diskWriteBytes / 1024.0 / 1024.0,
		NetRxMB:       netRxBytes / 1024.0 / 1024.0,
		NetTxMB:       netTxBytes / 1024.0 / 1024.0,
	}, nil
}

func queryRangeMean(promQL string, startUnix, endUnix int64, stepSec int) (float64, error) {
	values, err := queryRangeValues(promQL, startUnix, endUnix, stepSec)
	if err != nil {
		return 0, err
	}
	if len(values) == 0 {
		return 0, nil
	}

	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values)), nil
}

func queryRangeMax(promQL string, startUnix, endUnix int64, stepSec int) (float64, error) {
	values, err := queryRangeValues(promQL, startUnix, endUnix, stepSec)
	if err != nil {
		return 0, err
	}
	if len(values) == 0 {
		return 0, nil
	}

	maxVal := values[0]
	for _, v := range values[1:] {
		if v > maxVal {
			maxVal = v
		}
	}
	return maxVal, nil
}

func queryRangeValues(promQL string, startUnix, endUnix int64, stepSec int) ([]float64, error) {
	baseURL := "http://localhost:9090/api/v1/query_range"

	params := url.Values{}
	params.Set("query", promQL)
	params.Set("start", strconv.FormatInt(startUnix, 10))
	params.Set("end", strconv.FormatInt(endUnix, 10))
	params.Set("step", fmt.Sprintf("%ds", stepSec))

	fullURL := baseURL + "?" + params.Encode()

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(fullURL)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", fullURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("prometheus returned status %d: %s", resp.StatusCode, string(body))
	}

	var parsed promQueryRangeResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode prometheus response: %w", err)
	}

	if parsed.Status != "success" {
		return nil, fmt.Errorf("prometheus status is not success: %s", parsed.Status)
	}

	var out []float64
	for _, series := range parsed.Data.Result {
		for _, pair := range series.Values {
			if len(pair) != 2 {
				continue
			}

			rawStr, ok := pair[1].(string)
			if !ok {
				continue
			}

			val, err := strconv.ParseFloat(rawStr, 64)
			if err != nil {
				continue
			}

			out = append(out, val)
		}
	}

	return out, nil
}

func regexpEscape(s string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`.`, `\.`,
		`+`, `\+`,
		`*`, `\*`,
		`?`, `\?`,
		`(`, `\(`,
		`)`, `\)`,
		`[`, `\[`,
		`]`, `\]`,
		`{`, `\{`,
		`}`, `\}`,
		`^`, `\^`,
		`$`, `\$`,
		`|`, `\|`,
	)
	return replacer.Replace(s)
}