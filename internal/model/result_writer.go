package model

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func SaveRunResult(path string, result RunResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
