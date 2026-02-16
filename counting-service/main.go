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

	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "80"
	}
	portWithColon := fmt.Sprintf(":%s", port)

	// Redis Connection
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "localhost:6379"
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: redisURL,
	})

	// Ping Redis to ensure connection
	ctx := context.Background()
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Printf("Warning: Could not connect to Redis at %s: %v", redisURL, err)
	} else {
		fmt.Printf("Connected to Redis at %s\n", redisURL)
	}

	router := mux.NewRouter()
	router.HandleFunc("/health", HealthHandler)

	router.PathPrefix("/").Handler(CountHandler{client: rdb})

	// Serve!
	fmt.Printf("Serving at http://localhost:%s\n(Pass as PORT environment variable)\n", port)
	log.Fatal(http.ListenAndServe(portWithColon, router))
}

// HealthHandler returns a succesful status and a message.
// For use by Consul or other processes that need to verify service health.
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
	client *redis.Client
}

func (h CountHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	hostname, _ := os.Hostname()

	// Increment the count in Redis
	newCount, err := h.client.Incr(ctx, "count").Result()
	if err != nil {
		// Graceful degradation
		count := Count{
			Count:    -1,
			Hostname: hostname,
			Message:  fmt.Sprintf("Redis Error: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(count)
		return
	}

	// Try to get Redis Run ID (Unique ID of the Redis instance)
	// We use "INFO SERVER" command and look for "run_id"
	redisInfo, err := h.client.Info(ctx, "server").Result()
	redisRunID := "Unknown"
	if err == nil {
		lines := strings.Split(redisInfo, "\r\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "run_id:") {
				redisRunID = strings.TrimPrefix(line, "run_id:")
				break
			}
		}
	}

	count := Count{
		Count:     newCount,
		Hostname:  hostname,
		RedisHost: redisRunID,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(count)
}
