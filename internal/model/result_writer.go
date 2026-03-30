package model

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func saveJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

func SaveRunResult(path string, result RunResult) error {
	return saveJSON(path, result)
}

func SaveQueryRunResult(path string, result QueryRunResult) error {
	return saveJSON(path, result)
}