// Package cache реализует кэширование метрик в Redis
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"

	"highload-service/internal/models"
)

const (
	// MetricKeyPrefix префикс для ключей метрик
	MetricKeyPrefix = "metric:"
	// LatestMetricsKey ключ для последних метрик
	LatestMetricsKey = "metrics:latest"
	// AnalysisKeyPrefix префикс для результатов анализа
	AnalysisKeyPrefix = "analysis:"
	// StatsKey ключ для статистики
	StatsKey = "stats:global"
	// DefaultTTL время жизни записи по умолчанию
	DefaultTTL = 5 * time.Minute
	// MetricsTTL время жизни метрик
	MetricsTTL = 1 * time.Hour
)

// RedisCache реализует кэширование в Redis
type RedisCache struct {
	client *redis.Client
	ctx    context.Context
}

// NewRedisCache создает новое подключение к Redis
func NewRedisCache(addr, password string, db int) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		PoolSize:     100,
		MinIdleConns: 10,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	ctx := context.Background()

	// Проверяем подключение
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisCache{
		client: client,
		ctx:    ctx,
	}, nil
}

// CacheMetric сохраняет метрику в Redis
func (r *RedisCache) CacheMetric(m models.Metric) error {
	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("failed to marshal metric: %w", err)
	}

	key := fmt.Sprintf("%s%d", MetricKeyPrefix, m.Timestamp.UnixNano())

	pipe := r.client.Pipeline()
	pipe.Set(r.ctx, key, data, MetricsTTL)
	pipe.LPush(r.ctx, LatestMetricsKey, data)
	pipe.LTrim(r.ctx, LatestMetricsKey, 0, 999) // Храним последние 1000 метрик

	_, err = pipe.Exec(r.ctx)
	if err != nil {
		return fmt.Errorf("failed to cache metric: %w", err)
	}

	return nil
}

// GetLatestMetrics возвращает последние N метрик
func (r *RedisCache) GetLatestMetrics(count int64) ([]models.Metric, error) {
	data, err := r.client.LRange(r.ctx, LatestMetricsKey, 0, count-1).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get latest metrics: %w", err)
	}

	metrics := make([]models.Metric, 0, len(data))
	for _, d := range data {
		var m models.Metric
		if err := json.Unmarshal([]byte(d), &m); err != nil {
			continue
		}
		metrics = append(metrics, m)
	}

	return metrics, nil
}

// CacheAnalysisResult сохраняет результат анализа
func (r *RedisCache) CacheAnalysisResult(result models.AnalysisResult) error {
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal analysis result: %w", err)
	}

	key := fmt.Sprintf("%s%d", AnalysisKeyPrefix, result.Timestamp.UnixNano())
	return r.client.Set(r.ctx, key, data, DefaultTTL).Err()
}

// IncrementCounter увеличивает счетчик
func (r *RedisCache) IncrementCounter(key string) (int64, error) {
	return r.client.Incr(r.ctx, key).Result()
}

// GetCounter возвращает значение счетчика
func (r *RedisCache) GetCounter(key string) (int64, error) {
	val, err := r.client.Get(r.ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

// SetWithTTL устанавливает значение с TTL
func (r *RedisCache) SetWithTTL(key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return r.client.Set(r.ctx, key, data, ttl).Err()
}

// Get получает значение по ключу
func (r *RedisCache) Get(key string, dest interface{}) error {
	data, err := r.client.Get(r.ctx, key).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

// Ping проверяет соединение с Redis
func (r *RedisCache) Ping() error {
	return r.client.Ping(r.ctx).Err()
}

// Close закрывает соединение
func (r *RedisCache) Close() error {
	return r.client.Close()
}

// FlushDB очищает базу (только для тестов)
func (r *RedisCache) FlushDB() error {
	return r.client.FlushDB(r.ctx).Err()
}
