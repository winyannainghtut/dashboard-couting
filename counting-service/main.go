// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/gorilla/mux"
)

// InMemoryStore implements an in-memory counter.
type InMemoryStore struct {
	mu    sync.Mutex
	count int64
}

func (m *InMemoryStore) Incr() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.count++
	return m.count
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9001"
	}
	portWithColon := fmt.Sprintf(":%s", port)

	// Enforce Memory Mode
	storageMode := os.Getenv("STORAGE_MODE")
	if storageMode != "memory" && storageMode != "" {
		fmt.Printf("Warning: STORAGE_MODE=%s is not supported in this version. Defaulting to 'memory'.\n", storageMode)
	}
	fmt.Println("Starting in Standalone Mode (In-Memory)")
	store := &InMemoryStore{}

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
	Count    int64  `json:"count"`
	Hostname string `json:"hostname"`
}

// CountHandler serves a JSON feed that contains a number that increments each time
// the API is called.
type CountHandler struct {
	store *InMemoryStore
}

func (h CountHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()

	newCount := h.store.Incr()
	count := Count{
		Count:    newCount,
		Hostname: hostname,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(count)
}
