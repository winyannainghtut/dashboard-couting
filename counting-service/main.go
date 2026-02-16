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
	Count    int64  `json:"count"`
	Hostname string `json:"hostname"`
}

// CountHandler serves a JSON feed that contains a number that increments each time
// the API is called.
type CountHandler struct {
	client *redis.Client
}

func (h CountHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// Increment the count in Redis
	newCount, err := h.client.Incr(ctx, "count").Result()
	if err != nil {
		http.Error(w, fmt.Sprintf("Redis Error: %v", err), http.StatusInternalServerError)
		return
	}

	hostname, _ := os.Hostname()
	count := Count{Count: newCount, Hostname: hostname}

	responseJSON, _ := json.Marshal(count)
	fmt.Fprintf(w, string(responseJSON))
}
