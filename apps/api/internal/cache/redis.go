package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache interface {
	Get(context.Context, string) ([]byte, bool, error)
	Set(context.Context, string, []byte, time.Duration) error
	Delete(context.Context, string) error
	Available(context.Context) bool
}

type Redis struct{ client *redis.Client }

func NewRedis(rawURL string) (*Redis, error) {
	options, err := redis.ParseURL(rawURL)
	if err != nil {
		return nil, err
	}
	return &Redis{client: redis.NewClient(options)}, nil
}

func (r *Redis) Close() error { return r.client.Close() }

func (r *Redis) Get(ctx context.Context, key string) ([]byte, bool, error) {
	value, err := r.client.Get(ctx, "rbc:external:"+key).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	return value, err == nil, err
}

func (r *Redis) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	return r.client.Set(ctx, "rbc:external:"+key, value, ttl).Err()
}

func (r *Redis) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, "rbc:external:"+key).Err()
}

func (r *Redis) Available(ctx context.Context) bool { return r.client.Ping(ctx).Err() == nil }

type Noop struct{}

func (Noop) Get(context.Context, string) ([]byte, bool, error)        { return nil, false, nil }
func (Noop) Set(context.Context, string, []byte, time.Duration) error { return nil }
func (Noop) Delete(context.Context, string) error                     { return nil }
func (Noop) Available(context.Context) bool                           { return false }
