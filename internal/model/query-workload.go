package model

import (
	"encoding/json"
	"os"
)

type QueryWorkload struct {
	Name    string              `json:"name"`
	Queries []QueryWorkloadItem `json:"queries"`
}

func LoadQueryWorkload(path string) (QueryWorkload, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return QueryWorkload{}, err
	}

	var workload QueryWorkload
	if err := json.Unmarshal(data, &workload); err != nil {
		return QueryWorkload{}, err
	}

	return workload, nil
}