package model

import "sync"

type QueryAccumulator struct {
	Name           string
	Count          int
	Failed         int
	SumDurationMs  float64
	MaxDurationMs  float64
	MinDurationMs  float64
	LastDurationMs float64
	initialized    bool
	mu             sync.Mutex
}

func (a *QueryAccumulator) Add(durationMs float64, failed bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.Count++
	if failed {
		a.Failed++
	}

	a.SumDurationMs += durationMs
	a.LastDurationMs = durationMs

	if !a.initialized {
		a.MinDurationMs = durationMs
		a.MaxDurationMs = durationMs
		a.initialized = true
		return
	}

	if durationMs > a.MaxDurationMs {
		a.MaxDurationMs = durationMs
	}
	if durationMs < a.MinDurationMs {
		a.MinDurationMs = durationMs
	}
}

func (a *QueryAccumulator) ToStat() QueryStat {
	a.mu.Lock()
	defer a.mu.Unlock()

	avg := 0.0
	if a.Count > 0 {
		avg = a.SumDurationMs / float64(a.Count)
	}

	min := 0.0
	max := 0.0
	last := 0.0
	if a.initialized {
		min = a.MinDurationMs
		max = a.MaxDurationMs
		last = a.LastDurationMs
	}

	return QueryStat{
		Name:           a.Name,
		Count:          a.Count,
		Failed:         a.Failed,
		AvgDurationMs:  avg,
		MaxDurationMs:  max,
		MinDurationMs:  min,
		LastDurationMs: last,
	}
}