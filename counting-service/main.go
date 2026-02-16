// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
)

// CounterStore defines the interface for counting storage operations.
type CounterStore interface {
	Incr(ctx context.Context) (int64, error)
	GetInfo(ctx context.Context) (string, error)
}

// RedisStore implements CounterStore using Redis.
type RedisStore struct {
	client *redis.Client
}

func (r *RedisStore) Incr(ctx context.Context) (int64, error) {
	return r.client.Incr(ctx, "count").Result()
}

func (r *RedisStore) GetInfo(ctx context.Context) (string, error) {
	redisInfo, err := r.client.Info(ctx, "server").Result()
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

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "80"
	}
	portWithColon := fmt.Sprintf(":%s", port)

	var store CounterStore
	storageMode := os.Getenv("STORAGE_MODE")
	if storageMode == "memory" {
		fmt.Println("Starting in Standalone Mode (In-Memory)")
		store = &InMemoryStore{}
	} else {
		// Default to Redis
		redisURL := os.Getenv("REDIS_URL")
		if redisURL == "" {
			redisURL = "localhost:6379"
		}

		rdb := redis.NewClient(&redis.Options{
			Addr:         redisURL,
			DialTimeout:  time.Second,
			ReadTimeout:  time.Second,
			WriteTimeout: time.Second,
		})

		// Ping Redis to ensure connection
		ctx := context.Background()
		_, err := rdb.Ping(ctx).Result()
		if err != nil {
			log.Printf("Warning: Could not connect to Redis at %s: %v", redisURL, err)
		} else {
			fmt.Printf("Connected to Redis at %s\n", redisURL)
		}
		store = &RedisStore{client: rdb}
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
