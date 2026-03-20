package config

import "os"

type Config struct {
	HTTPAddr      string
	RedisAddr     string
	RedisStream   string
	RedisGroup    string
	RedisConsumer string
	PostgresDSN   string
	ClickHouseDSN string

	CollectorURL     string
	GeneratorEPS     string
	GeneratorBatch   string
	GeneratorSec     string
	GeneratorBackend string
}

func getEnv(key, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}

func Load() Config {
	return Config{
		HTTPAddr:      getEnv("HTTP_ADDR", ":8080"),
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisStream:   getEnv("REDIS_STREAM", "events"),
		RedisGroup:    getEnv("REDIS_GROUP", "workers"),
		RedisConsumer: getEnv("REDIS_CONSUMER", "worker-1"),
		PostgresDSN:   getEnv("POSTGRES_DSN", "postgres://siem:siem@localhost:5432/siem?sslmode=disable"),
		ClickHouseDSN: getEnv("CLICKHOUSE_DSN", "clickhouse://localhost:9000?database=siem"),

		CollectorURL:     getEnv("COLLECTOR_URL", "http://localhost:8080/ingest"),
		GeneratorEPS:     getEnv("GENERATOR_EPS", "100"),
		GeneratorBatch:   getEnv("GENERATOR_BATCH", "10"),
		GeneratorSec:     getEnv("GENERATOR_SEC", "10"),
		GeneratorBackend: getEnv("GENERATOR_BACKEND", "postgres"),
	}
}
