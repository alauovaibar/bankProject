package repository

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type CacheRepository interface {
	GetBalance(ctx context.Context, accountID string) (int64, bool, error)
	SetBalance(ctx context.Context, accountID string, balance int64, ttl time.Duration) error
	InvalidateBalance(ctx context.Context, accountID string) error
}

type cacheRepository struct {
	client *redis.Client
}

func NewCacheRepository(client *redis.Client) CacheRepository {
	return &cacheRepository{client: client}
}

func (r *cacheRepository) GetBalance(ctx context.Context, accountID string) (int64, bool, error) {
	key := "balance:" + accountID
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, false, nil
		}
		return 0, false, err
	}

	balance, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, false, err
	}

	return balance, true, nil
}

func (r *cacheRepository) SetBalance(ctx context.Context, accountID string, balance int64, ttl time.Duration) error {
	key := "balance:" + accountID
	return r.client.Set(ctx, key, balance, ttl).Err()
}

func (r *cacheRepository) InvalidateBalance(ctx context.Context, accountID string) error {
	key := "balance:" + accountID
	return r.client.Del(ctx, key).Err()
}
