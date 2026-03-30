package buffer

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/redis/go-redis/v9"
	"siem-bench/internal/model"
)

type RedisBuffer struct {
	client *redis.Client
	stream string
}

type StreamMessage struct {
	ID    string
	Event model.Event
}

func NewRedisBuffer(addr, stream string) *RedisBuffer {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	return &RedisBuffer{
		client: client,
		stream: stream,
	}
}

func (r *RedisBuffer) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *RedisBuffer) PublishEvent(ctx context.Context, event model.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return r.client.XAdd(ctx, &redis.XAddArgs{
		Stream: r.stream,
		Values: map[string]any{
			"payload": string(payload),
		},
	}).Err()
}

func (r *RedisBuffer) PublishEvents(ctx context.Context, events []model.Event) error {
	pipe := r.client.Pipeline()
	for _, event := range events {
		payload, err := json.Marshal(event)
		if err != nil {
			return err
		}
		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: r.stream,
			Values: map[string]any{
				"payload": string(payload),
			},
		})
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (r *RedisBuffer) EnsureGroup(ctx context.Context, group string) error {
	err := r.client.XGroupCreateMkStream(ctx, r.stream, group, "0").Err()
	if err != nil {
		if err.Error() == "BUSYGROUP Consumer Group name already exists" {
			return nil
		}
		return err
	}
	return nil
}

func (r *RedisBuffer) ReadGroup(ctx context.Context, group, consumer string, count int64) ([]StreamMessage, error) {
	streams, err := r.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{r.stream, ">"},
		Count:    count,
		Block:    5 * 1000 * 1000 * 1000,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}

	var result []StreamMessage
	for _, stream := range streams {
		for _, msg := range stream.Messages {
			rawPayload, ok := msg.Values["payload"]
			if !ok {
				continue
			}
			payloadStr, ok := rawPayload.(string)
			if !ok {
				continue
			}

			var event model.Event
			if err := json.Unmarshal([]byte(payloadStr), &event); err != nil {
				return nil, err
			}

			result = append(result, StreamMessage{
				ID:    msg.ID,
				Event: event,
			})
		}
	}
	return result, nil
}

func (r *RedisBuffer) Ack(ctx context.Context, group string, ids ...string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.client.XAck(ctx, r.stream, group, ids...).Err()
}

func (r *RedisBuffer) StreamLen(ctx context.Context) (int64, error) {
	return r.client.XLen(ctx, r.stream).Result()
}

func (r *RedisBuffer) PendingCount(ctx context.Context, group string) (int64, error) {
	summary, err := r.client.XPending(ctx, r.stream, group).Result()
	if err != nil {
		return 0, err
	}
	return summary.Count, nil
}