// Package cache — тонкая обёртка над go-redis: JSON-значения с TTL.
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const pingTimeout = 3 * time.Second

// Client — клиент Redis.
type Client struct {
	rdb  *redis.Client
	addr string
}

// NewFromEnv читает REDIS_ADDR (по умолчанию localhost:6379), REDIS_DB (0), REDIS_PASSWORD и проверяет соединение.
func NewFromEnv(ctx context.Context) (*Client, error) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	dbNum := 0
	if v := os.Getenv("REDIS_DB"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("REDIS_DB должен быть числом, получено %q", v)
		}
		dbNum = n
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       dbNum,
	})

	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("Redis недоступен (%s): %w", addr, err)
	}
	return &Client{rdb: rdb, addr: addr}, nil
}

// Addr возвращает адрес Redis.
func (c *Client) Addr() string { return c.addr }

// GetJSON читает ключ и разбирает JSON в dst. hit=false, если ключа нет.
func (c *Client) GetJSON(ctx context.Context, key string, dst any) (bool, error) {
	data, err := c.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return false, fmt.Errorf("повреждённый JSON в ключе %s: %w", key, err)
	}
	return true, nil
}

// SetJSON сериализует v в JSON и записывает с TTL.
func (c *Client) SetJSON(ctx context.Context, key string, v any, ttl time.Duration) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, key, data, ttl).Err()
}

// Del удаляет ключи.
func (c *Client) Del(ctx context.Context, keys ...string) error {
	return c.rdb.Del(ctx, keys...).Err()
}

// Close закрывает соединение.
func (c *Client) Close() error { return c.rdb.Close() }
