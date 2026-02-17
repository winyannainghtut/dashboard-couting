// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const defaultDBRequestTimeout = 1 * time.Second
const defaultDNSNetwork = "udp"
const defaultDNSPort = "53"
const defaultDNSTimeout = 1500 * time.Millisecond

// CounterStore describes storage operations for the counter.
type CounterStore interface {
	Incr(ctx context.Context) (int64, error)
	GetDBNode(ctx context.Context) (string, error)
}

// InMemoryStore implements an in-memory counter.
type InMemoryStore struct {
	mu    sync.Mutex
	count int64
}

func (m *InMemoryStore) Incr(ctx context.Context) (int64, error) {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	m.count++
	return m.count, nil
}

func (m *InMemoryStore) GetDBNode(ctx context.Context) (string, error) {
	_ = ctx
	return "", nil
}

// CockroachStore uses CockroachDB for persistence.
type CockroachStore struct {
	db *sql.DB
}

func NewCockroachStore(pgURL string) (*CockroachStore, error) {
	db, err := sql.Open("pgx", pgURL)
	if err != nil {
		return nil, err
	}
	return &CockroachStore{db: db}, nil
}

func (c *CockroachStore) ensureSchema(ctx context.Context) error {
	_, err := c.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS counts (
		id INT PRIMARY KEY,
		count BIGINT NOT NULL
	)`)
	if err != nil {
		return err
	}

	_, err = c.db.ExecContext(ctx, `INSERT INTO counts (id, count) VALUES (1, 0)
		ON CONFLICT (id) DO NOTHING`)
	return err
}

func (c *CockroachStore) Incr(ctx context.Context) (int64, error) {
	if err := c.ensureSchema(ctx); err != nil {
		return 0, err
	}

	var count int64
	err := c.db.QueryRowContext(ctx, `UPDATE counts SET count = count + 1 WHERE id = 1 RETURNING count`).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (c *CockroachStore) GetDBNode(ctx context.Context) (string, error) {
	var nodeID int64
	err := c.db.QueryRowContext(ctx, `SELECT crdb_internal.node_id()`).Scan(&nodeID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Node %d", nodeID), nil
}

func writeJSON(w http.ResponseWriter, payload Count) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func getCustomDNSServer() string {
	dnsServer := strings.TrimSpace(os.Getenv("DNS_SERVER"))
	if dnsServer != "" {
		return dnsServer
	}
	return strings.TrimSpace(os.Getenv("CONSUL_DNS_ADDR"))
}

func getCustomDNSNetwork() string {
	dnsNetwork := strings.TrimSpace(os.Getenv("DNS_NETWORK"))
	if dnsNetwork == "" {
		return defaultDNSNetwork
	}
	return dnsNetwork
}

func getCustomDNSTimeout() time.Duration {
	raw := strings.TrimSpace(os.Getenv("DNS_TIMEOUT_MS"))
	if raw == "" {
		return defaultDNSTimeout
	}

	ms, err := strconv.Atoi(raw)
	if err != nil || ms <= 0 {
		log.Printf("Invalid DNS_TIMEOUT_MS=%q. Using default %s.", raw, defaultDNSTimeout)
		return defaultDNSTimeout
	}

	return time.Duration(ms) * time.Millisecond
}

func getDBRequestTimeout() time.Duration {
	raw := strings.TrimSpace(os.Getenv("DB_REQUEST_TIMEOUT_MS"))
	if raw == "" {
		return defaultDBRequestTimeout
	}

	ms, err := strconv.Atoi(raw)
	if err != nil || ms <= 0 {
		log.Printf("Invalid DB_REQUEST_TIMEOUT_MS=%q. Using default %s.", raw, defaultDBRequestTimeout)
		return defaultDBRequestTimeout
	}

	return time.Duration(ms) * time.Millisecond
}

func normalizeDNSServerAddr(dnsServer string) string {
	if _, _, err := net.SplitHostPort(dnsServer); err == nil {
		return dnsServer
	}
	return net.JoinHostPort(dnsServer, defaultDNSPort)
}

func configureCustomDNSResolver() {
	dnsServer := getCustomDNSServer()
	if dnsServer == "" {
		return
	}

	dnsServer = normalizeDNSServerAddr(dnsServer)
	dnsNetwork := getCustomDNSNetwork()
	dnsTimeout := getCustomDNSTimeout()

	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			dialer := &net.Dialer{Timeout: dnsTimeout}
			return dialer.DialContext(ctx, dnsNetwork, dnsServer)
		},
	}

	log.Printf("Custom DNS resolver enabled: %s://%s", dnsNetwork, dnsServer)
}

func main() {
	configureCustomDNSResolver()

	port := os.Getenv("PORT")
	if port == "" {
		port = "9001"
	}
	portWithColon := fmt.Sprintf(":%s", port)

	var store CounterStore
	storageMode := os.Getenv("STORAGE_MODE")

	switch storageMode {
	case "", "memory":
		fmt.Println("Starting in Standalone Mode (In-Memory)")
		store = &InMemoryStore{}
	case "cockroach":
		pgURL := os.Getenv("PG_URL")
		if pgURL == "" {
			log.Fatal("PG_URL must be set when STORAGE_MODE=cockroach")
		}

		fmt.Printf("Connecting to CockroachDB at %s\n", pgURL)
		cockroachStore, err := NewCockroachStore(pgURL)
		if err != nil {
			log.Fatalf("Failed to initialize CockroachDB store: %v", err)
		}
		store = cockroachStore
	default:
		fmt.Printf("Warning: STORAGE_MODE=%s is not supported. Defaulting to 'memory'.\n", storageMode)
		store = &InMemoryStore{}
	}

	router := mux.NewRouter()
	router.HandleFunc("/health", HealthHandler)
	router.Handle("/", CountHandler{store: store, dbRequestTimeout: getDBRequestTimeout()})

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
	DBNode   string `json:"db_node,omitempty"`
	Message  string `json:"message,omitempty"`
}

// CountHandler serves a JSON feed that contains a number that increments each time
// the API is called.
type CountHandler struct {
	store            CounterStore
	dbRequestTimeout time.Duration
}

func (h CountHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()

	ctx, cancel := context.WithTimeout(r.Context(), h.dbRequestTimeout)
	defer cancel()

	newCount, err := h.store.Incr(ctx)
	if err != nil {
		count := Count{
			Count:    -1,
			Hostname: hostname,
			Message:  fmt.Sprintf("DB Error: %v", err),
		}
		writeJSON(w, count)
		return
	}

	count := Count{
		Count:    newCount,
		Hostname: hostname,
	}

	dbNode, dbErr := h.store.GetDBNode(ctx)
	if dbErr == nil {
		count.DBNode = dbNode
	}

	writeJSON(w, count)
}
