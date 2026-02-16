// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
)

// CounterStore defines the interface for counting storage operations.
type CounterStore interface {
	Incr(ctx context.Context) (int64, error)
	GetInfo(ctx context.Context) (string, error)
}

// RedisStore implements CounterStore using Redis.
type RedisStore struct {
	client redis.UniversalClient
}

func (r *RedisStore) Incr(ctx context.Context) (int64, error) {
	return r.client.Incr(ctx, "count").Result()
}

func (r *RedisStore) GetInfo(ctx context.Context) (string, error) {
	// UniversalClient doesn't have a direct Info method that returns a single string for "server".
	// We need to cast it to a specific client type or use a more generic approach.
	// For simplicity, we'll try to get info from the primary node if it's a failover/cluster client,
	// or directly from the client if it's a single node.
	// This might not be perfectly accurate for all UniversalClient implementations,
	// but it's a reasonable attempt for common Redis setups.
	var redisInfo string
	var err error

	switch c := r.client.(type) {
	case *redis.Client:
		redisInfo, err = c.Info(ctx, "server").Result()
	case *redis.ClusterClient:
		// ClusterClient.Info returns a string (potentially combined or from a random node).
		// We'll just use it directly.
		redisInfo, err = c.Info(ctx, "server").Result()
	default:
		return "Unknown (Unsupported Redis Client Type)", nil
	}

	if err != nil {
		return "", err
	}

	lines := strings.Split(redisInfo, "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "run_id:") {
			return strings.TrimPrefix(line, "run_id:"), nil
		}
	}
	return "Unknown", nil
}

// InMemoryStore implements CounterStore using an in-memory variable.
type InMemoryStore struct {
	mu    sync.Mutex
	count int64
}

func (m *InMemoryStore) Incr(ctx context.Context) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.count++
	return m.count, nil
}

func (m *InMemoryStore) GetInfo(ctx context.Context) (string, error) {
	hostname, _ := os.Hostname()
	return fmt.Sprintf("In-Memory (Host: %s)", hostname), nil
}

// PostgresStore implements CounterStore using PostgreSQL.
type PostgresStore struct {
	db *sql.DB
}

func (p *PostgresStore) Incr(ctx context.Context) (int64, error) {
	var count int64
	err := p.db.QueryRowContext(ctx,
		"UPDATE counters SET count = count + 1 WHERE id = 'default' RETURNING count",
	).Scan(&count)
	return count, err
}

func (p *PostgresStore) GetInfo(ctx context.Context) (string, error) {
	var version string
	err := p.db.QueryRowContext(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		return "", err
	}
	// Truncate long version strings
	if len(version) > 60 {
		version = version[:60] + "..."
	}
	return version, nil
}

func getPostgresDB() *sql.DB {
	pgURL := os.Getenv("PG_URL")
	if pgURL == "" {
		pgURL = "postgres://counting:counting@localhost:5432/counting?sslmode=disable"
	}

	fmt.Printf("Connecting to PostgreSQL: %s\n", pgURL)
	db, err := sql.Open("pgx", pgURL)
	if err != nil {
		log.Printf("Warning: Could not open PostgreSQL connection: %v", err)
		return db
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Ping
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Printf("Warning: Could not connect to PostgreSQL: %v", err)
	} else {
		fmt.Println("Connected to PostgreSQL")
	}
	return db
}

func getRedisClient() redis.UniversalClient {
	mode := os.Getenv("REDIS_MODE")
	if mode == "cluster" {
		// Cluster Mode
		clusterAddrsStr := os.Getenv("REDIS_CLUSTER_ADDRS")
		var clusterAddrs []string
		if clusterAddrsStr != "" {
			clusterAddrs = strings.Split(clusterAddrsStr, ",")
		} else {
			// Fallback
			clusterAddrs = []string{"localhost:6379"}
		}

		fmt.Printf("Connecting to Redis Cluster: Addrs=%v\n", clusterAddrs)
		rdb := redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:        clusterAddrs,
			DialTimeout:  time.Second,
			ReadTimeout:  time.Second,
			WriteTimeout: time.Second,
		})

		// Ping
		ctx := context.Background()
		_, err := rdb.Ping(ctx).Result()
		if err != nil {
			log.Printf("Warning: Could not connect to Redis Cluster: %v", err)
		} else {
			fmt.Printf("Connected to Redis Cluster\n")
		}
		return rdb
	} else if mode == "sentinel" {
		// Sentinel Mode
		masterName := os.Getenv("REDIS_MASTER_NAME")
		if masterName == "" {
			masterName = "mymaster" // Default
		}

		sentinelAddrsStr := os.Getenv("REDIS_SENTINEL_ADDRS")
		var sentinelAddrs []string
		if sentinelAddrsStr != "" {
			sentinelAddrs = strings.Split(sentinelAddrsStr, ",")
		} else {
			// Fallback/Default for demo
			sentinelAddrs = []string{"localhost:26379"}
		}

		fmt.Printf("Connecting to Redis Sentinel: Master=%s, Sentinels=%v\n", masterName, sentinelAddrs)
		rdb := redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:    masterName,
			SentinelAddrs: sentinelAddrs,
			DialTimeout:   time.Second,
			ReadTimeout:   time.Second,
			WriteTimeout:  time.Second,
		})

		// Ping
		ctx := context.Background()
		_, err := rdb.Ping(ctx).Result()
		if err != nil {
			log.Printf("Warning: Could not connect to Redis Sentinel: %v", err)
		} else {
			fmt.Printf("Connected to Redis Sentinel Master: %s\n", masterName)
		}
		return rdb

	} else {
		// Single Node Mode (Default)
		redisURL := os.Getenv("REDIS_URL")
		if redisURL == "" {
			redisURL = "localhost:6379"
		}

		fmt.Printf("Connecting to Single Redis Node: %s\n", redisURL)
		rdb := redis.NewClient(&redis.Options{
			Addr:         redisURL,
			DialTimeout:  time.Second,
			ReadTimeout:  time.Second,
			WriteTimeout: time.Second,
		})

		// Ping
		ctx := context.Background()
		_, err := rdb.Ping(ctx).Result()
		if err != nil {
			log.Printf("Warning: Could not connect to Redis at %s: %v", redisURL, err)
		} else {
			fmt.Printf("Connected to Redis at %s\n", redisURL)
		}
		return rdb
	}
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "80"
	}
	portWithColon := fmt.Sprintf(":%s", port)

	var store CounterStore
	storageMode := os.Getenv("STORAGE_MODE")
	switch storageMode {
	case "memory":
		fmt.Println("Starting in Standalone Mode (In-Memory)")
		store = &InMemoryStore{}
	case "postgres":
		fmt.Println("Starting in PostgreSQL Mode")
		store = &PostgresStore{db: getPostgresDB()}
	default:
		store = &RedisStore{client: getRedisClient()}
	}

	router := mux.NewRouter()
	router.HandleFunc("/health", HealthHandler)

	router.PathPrefix("/").Handler(CountHandler{store: store})

	// Serve!
	fmt.Printf("Serving at http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(portWithColon, router))
}

// HealthHandler returns a succesful status and a message.
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Hello, you've hit %s\n", r.URL.Path)
}

// Count stores a number that is being counted and other data to
// return as JSON in the API.
type Count struct {
	Count     int64  `json:"count"`
	Hostname  string `json:"hostname"`
	RedisHost string `json:"redis_host,omitempty"`
	Message   string `json:"message,omitempty"`
}

// CountHandler serves a JSON feed that contains a number that increments each time
// the API is called.
type CountHandler struct {
	store CounterStore
}

func (h CountHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	hostname, _ := os.Hostname()

	// Increment the count
	newCount, err := h.store.Incr(ctx)
	if err != nil {
		// Graceful degradation
		count := Count{
			Count:    -1,
			Hostname: hostname,
			Message:  fmt.Sprintf("Store Error: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(count)
		return
	}

	// Get Store Info (Redis Run ID or Memory Info)
	storeInfo, _ := h.store.GetInfo(ctx)

	count := Count{
		Count:     newCount,
		Hostname:  hostname,
		RedisHost: storeInfo,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(count)
}
