package config

import "os"

type Config struct {
	HTTPAddr string

	RedisAddr     string
	RedisStream   string
	RedisGroup    string
	RedisConsumer string

	PostgresDSN      string
	ClickHouseDSN    string
	ElasticsearchURL string

	CollectorURL string

	GeneratorEPS   string
	GeneratorBatch string
	GeneratorSec   string

	// backward-compatible legacy backend selector
	GeneratorBackend string

	// step 3: drain-aware ingest completion
	DrainTimeoutSec   string
	DrainPollMs       string
	DrainStableChecks string

	// step 4: worker tuning
	WorkerReadCount string
	WorkerWriteMode string

	// step 5: explicit scenario/backend separation
	IngestBackend string
	QueryBackend  string
	RunScenario   string
	RunTag        string

	QueryRunnerWarmupSec string
	QueryWorkloadPath    string
	QueryRunnerConcurrency string
}

func getEnv(key, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}

func Load() Config {
	legacyBackend := getEnv("GENERATOR_BACKEND", "postgres")

	return Config{
		HTTPAddr: getEnv("HTTP_ADDR", ":8080"),

		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisStream:   getEnv("REDIS_STREAM", "events"),
		RedisGroup:    getEnv("REDIS_GROUP", "workers"),
		RedisConsumer: getEnv("REDIS_CONSUMER", "worker-1"),

		PostgresDSN:      getEnv("POSTGRES_DSN", "postgres://siem:siem@localhost:5432/siem?sslmode=disable"),
		ClickHouseDSN:    getEnv("CLICKHOUSE_DSN", "clickhouse://localhost:9000?database=siem"),
		ElasticsearchURL: getEnv("ELASTICSEARCH_URL", "http://127.0.0.1:9200"),

		CollectorURL: getEnv("COLLECTOR_URL", "http://localhost:8080/ingest"),

		GeneratorEPS:   getEnv("GENERATOR_EPS", "100"),
		GeneratorBatch: getEnv("GENERATOR_BATCH", "10"),
		GeneratorSec:   getEnv("GENERATOR_SEC", "10"),

		GeneratorBackend: legacyBackend,

		DrainTimeoutSec:   getEnv("DRAIN_TIMEOUT_SEC", "30"),
		DrainPollMs:       getEnv("DRAIN_POLL_MS", "500"),
		DrainStableChecks: getEnv("DRAIN_STABLE_CHECKS", "3"),

		WorkerReadCount: getEnv("WORKER_READ_COUNT", "100"),
		WorkerWriteMode: getEnv("WORKER_WRITE_MODE", "batch"),

		IngestBackend: getEnv("INGEST_BACKEND", legacyBackend),
		QueryBackend:  getEnv("QUERY_BACKEND", legacyBackend),
		RunScenario:   getEnv("RUN_SCENARIO", ""),
		RunTag:        getEnv("RUN_TAG", ""),

		QueryRunnerWarmupSec: getEnv("QUERY_RUNNER_WARMUP_SEC", "3"),
		QueryWorkloadPath:    getEnv("QUERY_WORKLOAD_PATH", "scenarios/query-default.json"),
		QueryRunnerConcurrency: getEnv("QUERY_RUNNER_CONCURRENCY", "1"),
	}
}