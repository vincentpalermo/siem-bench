package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	esv9 "github.com/elastic/go-elasticsearch/v9"

	"siem-bench/internal/model"
)

type Storage struct {
	client *esv9.Client
	index  string
}

func New(url string) (*Storage, error) {
	cfg := esv9.Config{
		Addresses: []string{url},
	}

	client, err := esv9.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	return &Storage{
		client: client,
		index:  "siem-events",
	}, nil
}

func (s *Storage) Close() error {
	return nil
}

func (s *Storage) CountEvents(ctx context.Context) (int64, error) {
	res, err := s.client.Count(
		s.client.Count.WithContext(ctx),
		s.client.Count.WithIndex(s.index),
	)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()

	var body struct {
		Count int64 `json:"count"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return 0, err
	}

	return body.Count, nil
}

func (s *Storage) InsertEventsBatch(ctx context.Context, events []model.Event) error {
	var b strings.Builder

	for _, event := range events {
		meta := fmt.Sprintf(`{"index":{"_index":"%s","_id":"%s"}}`, s.index, event.ID)
		docBytes, err := json.Marshal(event)
		if err != nil {
			return err
		}

		b.WriteString(meta)
		b.WriteByte('\n')
		b.Write(docBytes)
		b.WriteByte('\n')
	}

	res, err := s.client.Bulk(
		bytes.NewReader([]byte(b.String())),
		s.client.Bulk.WithContext(ctx),
		s.client.Bulk.WithIndex(s.index),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("bulk insert failed: %s", res.String())
	}

	var body struct {
		Errors bool `json:"errors"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return err
	}
	if body.Errors {
		return fmt.Errorf("bulk insert completed with item errors")
	}

	return nil
}

func (s *Storage) SearchByHost(ctx context.Context, host string, limit int) ([]model.EventQueryResult, error) {
	query := map[string]any{
		"size": limit,
		"sort": []any{
			map[string]any{"timestamp": map[string]any{"order": "desc"}},
		},
		"query": map[string]any{
			"term": map[string]any{
				"host": host,
			},
		},
	}

	return s.search(ctx, query)
}

func (s *Storage) SearchByUser(ctx context.Context, userName string, limit int) ([]model.EventQueryResult, error) {
	query := map[string]any{
		"size": limit,
		"sort": []any{
			map[string]any{"timestamp": map[string]any{"order": "desc"}},
		},
		"query": map[string]any{
			"term": map[string]any{
				"user_name": userName,
			},
		},
	}

	return s.search(ctx, query)
}

func (s *Storage) CountBySeverity(ctx context.Context) ([]model.SeverityCount, error) {
	query := map[string]any{
		"size": 0,
		"aggs": map[string]any{
			"by_severity": map[string]any{
				"terms": map[string]any{
					"field": "severity",
					"size":  10,
					"order": map[string]any{"_key": "asc"},
				},
			},
		},
	}

	bodyBytes, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}

	res, err := s.client.Search(
		s.client.Search.WithContext(ctx),
		s.client.Search.WithIndex(s.index),
		s.client.Search.WithBody(bytes.NewReader(bodyBytes)),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var body struct {
		Aggregations struct {
			BySeverity struct {
				Buckets []struct {
					Key      int   `json:"key"`
					DocCount int64 `json:"doc_count"`
				} `json:"buckets"`
			} `json:"by_severity"`
		} `json:"aggregations"`
	}

	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return nil, err
	}

	results := make([]model.SeverityCount, 0, len(body.Aggregations.BySeverity.Buckets))
	for _, bucket := range body.Aggregations.BySeverity.Buckets {
		results = append(results, model.SeverityCount{
			Severity: bucket.Key,
			Count:    bucket.DocCount,
		})
	}

	return results, nil
}

func (s *Storage) TopHosts(ctx context.Context, limit int) ([]model.HostCount, error) {
	query := map[string]any{
		"size": 0,
		"aggs": map[string]any{
			"top_hosts": map[string]any{
				"terms": map[string]any{
					"field": "host",
					"size":  limit,
				},
			},
		},
	}

	bodyBytes, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}

	res, err := s.client.Search(
		s.client.Search.WithContext(ctx),
		s.client.Search.WithIndex(s.index),
		s.client.Search.WithBody(bytes.NewReader(bodyBytes)),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var body struct {
		Aggregations struct {
			TopHosts struct {
				Buckets []struct {
					Key      string `json:"key"`
					DocCount int64  `json:"doc_count"`
				} `json:"buckets"`
			} `json:"top_hosts"`
		} `json:"aggregations"`
	}

	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return nil, err
	}

	results := make([]model.HostCount, 0, len(body.Aggregations.TopHosts.Buckets))
	for _, bucket := range body.Aggregations.TopHosts.Buckets {
		results = append(results, model.HostCount{
			Host:  bucket.Key,
			Count: bucket.DocCount,
		})
	}

	return results, nil
}

func (s *Storage) search(ctx context.Context, query map[string]any) ([]model.EventQueryResult, error) {
	bodyBytes, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}

	res, err := s.client.Search(
		s.client.Search.WithContext(ctx),
		s.client.Search.WithIndex(s.index),
		s.client.Search.WithBody(bytes.NewReader(bodyBytes)),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var body struct {
		Hits struct {
			Hits []struct {
				Source model.EventQueryResult `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return nil, err
	}

	results := make([]model.EventQueryResult, 0, len(body.Hits.Hits))
	for _, hit := range body.Hits.Hits {
		results = append(results, hit.Source)
	}

	return results, nil
}
