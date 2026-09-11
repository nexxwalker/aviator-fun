package cache

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	_ "github.com/joho/godotenv/autoload"
)

type Service interface {
	GetClient() *redis.Client
	Health() map[string]string
	Close() error
}

type service struct {
	client *redis.Client
}

type redisConfig struct {
	addr     string
	password string
	db       int
	tls      bool
}

var cacheInstance *service

func loadRedisConfig() (redisConfig, error) {
	value := strings.TrimSpace(os.Getenv("REDIS_URL"))
	if value == "" {
		if strings.EqualFold(getEnv("APP_ENV", "development"), "production") {
			return redisConfig{}, fmt.Errorf("REDIS_URL is required when APP_ENV=production")
		}
		value = "localhost:6379"
	}

	if !strings.Contains(value, "://") {
		if _, _, err := net.SplitHostPort(value); err != nil {
			return redisConfig{}, fmt.Errorf("invalid REDIS_URL host:port: %w", err)
		}
		return redisConfig{
			addr:     value,
			password: os.Getenv("REDIS_PASSWORD"),
			db:       getEnvAsInt("REDIS_DB", 0),
		}, nil
	}

	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "redis" && parsed.Scheme != "rediss") || parsed.Host == "" {
		return redisConfig{}, fmt.Errorf("invalid REDIS_URL: expected redis:// or rediss:// URL")
	}

	db := getEnvAsInt("REDIS_DB", 0)
	if parsed.Path != "" && parsed.Path != "/" {
		if _, err := fmt.Sscanf(strings.TrimPrefix(parsed.Path, "/"), "%d", &db); err != nil || db < 0 {
			return redisConfig{}, fmt.Errorf("invalid Redis database in REDIS_URL")
		}
	}

	password := os.Getenv("REDIS_PASSWORD")
	if parsed.User != nil {
		if parsed.User.Username() != "" && parsed.User.Username() != "default" {
			return redisConfig{}, fmt.Errorf("Redis URL must use the default user")
		}
		if parsedPassword, ok := parsed.User.Password(); ok {
			password = parsedPassword
		}
	}

	return redisConfig{
		addr:     parsed.Host,
		password: password,
		db:       db,
		tls:      parsed.Scheme == "rediss",
	}, nil
}

func New() Service {
	if cacheInstance != nil {
		return cacheInstance
	}

	config, err := loadRedisConfig()
	if err != nil {
		log.Printf("[CACHE] Redis configuration error: %v", err)
		return nil
	}

	options := &redis.Options{
		Addr:         config.addr,
		Password:     config.password,
		DB:           config.db,
		PoolSize:     100,
		MinIdleConns: 10,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}
	if config.tls {
		options.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	client := redis.NewClient(options)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := client.Ping(ctx).Result(); err != nil {
		log.Printf("[CACHE] Redis connection failed: %v", err)
		log.Println("[CACHE] Running without Redis cache")
		return nil
	}

	log.Println("[CACHE] Redis connected successfully")

	cacheInstance = &service{
		client: client,
	}

	return cacheInstance
}

func (s *service) GetClient() *redis.Client {
	return s.client
}

func (s *service) Health() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	stats := make(map[string]string)

	_, err := s.client.Ping(ctx).Result()
	if err != nil {
		stats["status"] = "down"
		stats["error"] = fmt.Sprintf("redis down: %v", err)
		return stats
	}

	stats["status"] = "up"
	stats["message"] = "Redis is healthy"

	poolStats := s.client.PoolStats()
	stats["hits"] = strconv.FormatUint(uint64(poolStats.Hits), 10)
	stats["misses"] = strconv.FormatUint(uint64(poolStats.Misses), 10)
	stats["timeouts"] = strconv.FormatUint(uint64(poolStats.Timeouts), 10)
	stats["total_conns"] = strconv.FormatUint(uint64(poolStats.TotalConns), 10)
	stats["idle_conns"] = strconv.FormatUint(uint64(poolStats.IdleConns), 10)
	stats["stale_conns"] = strconv.FormatUint(uint64(poolStats.StaleConns), 10)

	return stats
}

func (s *service) Close() error {
	log.Println("[CACHE] Disconnecting from Redis")
	return s.client.Close()
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}
