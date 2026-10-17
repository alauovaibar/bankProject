package repository

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type SessionRepository interface {
	SaveRefreshToken(ctx context.Context, userID, token string, ttl time.Duration) error
	ValidateRefreshToken(ctx context.Context, userID, token string) (bool, error)
	RevokeRefreshToken(ctx context.Context, userID string) error
	AddToBlacklist(ctx context.Context, token string, ttl time.Duration) error
	IsBlacklisted(ctx context.Context, token string) (bool, error)
}

type sessionRepository struct {
	client *redis.Client
}

func NewSessionRepository(client *redis.Client) SessionRepository {
	return &sessionRepository{client: client}
}

func (r *sessionRepository) SaveRefreshToken(ctx context.Context, userID, token string, ttl time.Duration) error {
	key := "refresh:" + userID
	return r.client.Set(ctx, key, token, ttl).Err()
}

func (r *sessionRepository) ValidateRefreshToken(ctx context.Context, userID, token string) (bool, error) {
	key := "refresh:" + userID
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		return false, err
	}
	return val == token, nil
}

func (r *sessionRepository) RevokeRefreshToken(ctx context.Context, userID string) error {
	key := "refresh:" + userID
	return r.client.Del(ctx, key).Err()
}

func (r *sessionRepository) AddToBlacklist(ctx context.Context, token string, ttl time.Duration) error {
	key := "blacklist:" + token
	return r.client.Set(ctx, key, "revoked", ttl).Err()
}

func (r *sessionRepository) IsBlacklisted(ctx context.Context, token string) (bool, error) {
	key := "blacklist:" + token
	exists, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}
