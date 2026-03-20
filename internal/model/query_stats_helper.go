package model

type QueryAccumulator struct {
	Name            string
	Count           int
	Failed          int
	TotalDurationMs float64
	MaxDurationMs   float64
}

func (q *QueryAccumulator) Add(durationMs float64, failed bool) {
	q.Count++
	if failed {
		q.Failed++
	}
	q.TotalDurationMs += durationMs
	if durationMs > q.MaxDurationMs {
		q.MaxDurationMs = durationMs
	}
}

func (q *QueryAccumulator) ToStat() QueryStat {
	avg := 0.0
	if q.Count > 0 {
		avg = q.TotalDurationMs / float64(q.Count)
	}

	return QueryStat{
		Name:          q.Name,
		Count:         q.Count,
		Failed:        q.Failed,
		AvgDurationMs: avg,
		MaxDurationMs: q.MaxDurationMs,
	}
}
