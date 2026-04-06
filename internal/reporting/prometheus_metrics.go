package reporting

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type HistogramSnapshot struct {
	AvgMs float64
	P95Ms float64
	P99Ms float64
}

type histogramData struct {
	Buckets map[float64]float64
	Sum     float64
	Count   float64
}

func WorkerMetricsURL(backend string) string {
	switch backend {
	case "postgres":
		return "http://localhost:2112/metrics"
	case "clickhouse":
		return "http://localhost:2113/metrics"
	case "elasticsearch":
		return "http://localhost:2115/metrics"
	default:
		return ""
	}
}

func FetchWorkerLatencySnapshots(backend string) (HistogramSnapshot, HistogramSnapshot, error) {
	url := WorkerMetricsURL(backend)
	if url == "" {
		return HistogramSnapshot{}, HistogramSnapshot{}, fmt.Errorf("unsupported backend: %s", backend)
	}

	body, err := fetchMetricsBody(url)
	if err != nil {
		return HistogramSnapshot{}, HistogramSnapshot{}, err
	}

	e2eHist, err := parseHistogramFromPrometheusText(
		body,
		"siem_worker_e2e_latency_seconds",
		map[string]string{"backend": backend},
	)
	if err != nil {
		return HistogramSnapshot{}, HistogramSnapshot{}, fmt.Errorf("parse e2e histogram: %w", err)
	}

	queueHist, err := parseHistogramFromPrometheusText(
		body,
		"siem_worker_queue_latency_seconds",
		map[string]string{"backend": backend},
	)
	if err != nil {
		return HistogramSnapshot{}, HistogramSnapshot{}, fmt.Errorf("parse queue histogram: %w", err)
	}

	return buildHistogramSnapshot(e2eHist), buildHistogramSnapshot(queueHist), nil
}

func fetchMetricsBody(url string) (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: unexpected status %d", url, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read metrics body: %w", err)
	}

	return string(data), nil
}

func parseHistogramFromPrometheusText(body string, metricBase string, requiredLabels map[string]string) (histogramData, error) {
	h := histogramData{
		Buckets: make(map[float64]float64),
	}

	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		switch {
		case strings.HasPrefix(line, metricBase+"_bucket"):
			labels, value, err := parseMetricLine(line, metricBase+"_bucket")
			if err != nil {
				return histogramData{}, err
			}
			if !labelsMatch(labels, requiredLabels) {
				continue
			}

			leRaw, ok := labels["le"]
			if !ok {
				continue
			}

			var upperBound float64
			if leRaw == "+Inf" {
				upperBound = math.Inf(1)
			} else {
				upperBound, err = strconv.ParseFloat(leRaw, 64)
				if err != nil {
					return histogramData{}, fmt.Errorf("parse bucket le=%q: %w", leRaw, err)
				}
			}

			h.Buckets[upperBound] = value

		case strings.HasPrefix(line, metricBase+"_sum"):
			labels, value, err := parseMetricLine(line, metricBase+"_sum")
			if err != nil {
				return histogramData{}, err
			}
			if !labelsMatch(labels, requiredLabels) {
				continue
			}
			h.Sum = value

		case strings.HasPrefix(line, metricBase+"_count"):
			labels, value, err := parseMetricLine(line, metricBase+"_count")
			if err != nil {
				return histogramData{}, err
			}
			if !labelsMatch(labels, requiredLabels) {
				continue
			}
			h.Count = value
		}
	}

	if err := scanner.Err(); err != nil {
		return histogramData{}, fmt.Errorf("scan metrics body: %w", err)
	}

	if h.Count == 0 {
		return histogramData{}, fmt.Errorf("histogram %s not found or empty for labels %v", metricBase, requiredLabels)
	}

	return h, nil
}

func parseMetricLine(line string, metricName string) (map[string]string, float64, error) {
	labels := map[string]string{}

	if strings.HasPrefix(line, metricName+"{") {
		openIdx := strings.Index(line, "{")
		closeIdx := strings.LastIndex(line, "}")
		if openIdx < 0 || closeIdx < 0 || closeIdx <= openIdx {
			return nil, 0, fmt.Errorf("invalid labeled metric line: %s", line)
		}

		labelPart := line[openIdx+1 : closeIdx]
		valuePart := strings.TrimSpace(line[closeIdx+1:])

		parsedLabels, err := parseLabels(labelPart)
		if err != nil {
			return nil, 0, err
		}

		val, err := strconv.ParseFloat(valuePart, 64)
		if err != nil {
			return nil, 0, fmt.Errorf("parse metric value %q: %w", valuePart, err)
		}

		return parsedLabels, val, nil
	}

	if strings.HasPrefix(line, metricName+" ") {
		valuePart := strings.TrimSpace(strings.TrimPrefix(line, metricName))
		val, err := strconv.ParseFloat(valuePart, 64)
		if err != nil {
			return nil, 0, fmt.Errorf("parse metric value %q: %w", valuePart, err)
		}
		return labels, val, nil
	}

	return nil, 0, fmt.Errorf("line does not match metric %s: %s", metricName, line)
}

func parseLabels(labelPart string) (map[string]string, error) {
	labels := map[string]string{}
	if strings.TrimSpace(labelPart) == "" {
		return labels, nil
	}

	parts := splitLabelPairs(labelPart)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		eqIdx := strings.Index(part, "=")
		if eqIdx <= 0 {
			return nil, fmt.Errorf("invalid label pair: %s", part)
		}

		key := strings.TrimSpace(part[:eqIdx])
		rawVal := strings.TrimSpace(part[eqIdx+1:])
		if len(rawVal) < 2 || rawVal[0] != '"' || rawVal[len(rawVal)-1] != '"' {
			return nil, fmt.Errorf("invalid label value: %s", part)
		}

		unquoted, err := strconv.Unquote(rawVal)
		if err != nil {
			return nil, fmt.Errorf("unquote label value %q: %w", rawVal, err)
		}

		labels[key] = unquoted
	}

	return labels, nil
}

func splitLabelPairs(s string) []string {
	var parts []string
	var current strings.Builder
	inQuotes := false
	escaped := false

	for _, r := range s {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false

		case r == '\\':
			current.WriteRune(r)
			escaped = true

		case r == '"':
			current.WriteRune(r)
			inQuotes = !inQuotes

		case r == ',' && !inQuotes:
			parts = append(parts, current.String())
			current.Reset()

		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

func labelsMatch(metricLabels map[string]string, required map[string]string) bool {
	for k, v := range required {
		if metricLabels[k] != v {
			return false
		}
	}
	return true
}

func buildHistogramSnapshot(h histogramData) HistogramSnapshot {
	snap := HistogramSnapshot{}

	if h.Count > 0 {
		snap.AvgMs = (h.Sum / h.Count) * 1000.0
	}

	snap.P95Ms = histogramQuantileMs(h.Buckets, h.Count, 0.95)
	snap.P99Ms = histogramQuantileMs(h.Buckets, h.Count, 0.99)

	return snap
}

func histogramQuantileMs(cumulativeBuckets map[float64]float64, totalCount float64, q float64) float64 {
	if totalCount <= 0 || len(cumulativeBuckets) == 0 {
		return 0
	}

	type bucket struct {
		upper float64
		count float64
	}

	var buckets []bucket
	for upper, count := range cumulativeBuckets {
		buckets = append(buckets, bucket{upper: upper, count: count})
	}

	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].upper < buckets[j].upper
	})

	target := totalCount * q
	if target <= 0 {
		return 0
	}

	prevCount := 0.0
	prevUpper := 0.0

	for _, b := range buckets {
		if b.count >= target {
			if math.IsInf(b.upper, 1) {
				if prevUpper > 0 {
					return prevUpper * 1000.0
				}
				return 0
			}

			bucketCount := b.count - prevCount
			if bucketCount <= 0 {
				return b.upper * 1000.0
			}

			posInBucket := (target - prevCount) / bucketCount
			if posInBucket < 0 {
				posInBucket = 0
			}
			if posInBucket > 1 {
				posInBucket = 1
			}

			value := prevUpper + (b.upper-prevUpper)*posInBucket
			return value * 1000.0
		}

		prevCount = b.count
		if !math.IsInf(b.upper, 1) {
			prevUpper = b.upper
		}
	}

	lastFiniteUpper := 0.0
	for i := len(buckets) - 1; i >= 0; i-- {
		if !math.IsInf(buckets[i].upper, 1) {
			lastFiniteUpper = buckets[i].upper
			break
		}
	}

	return lastFiniteUpper * 1000.0
}