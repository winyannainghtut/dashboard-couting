// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
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
)

const defaultDNSNetwork = "udp"
const defaultDNSPort = "53"
const defaultDNSTimeout = 1500 * time.Millisecond

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
	configureCustomDNSResolver()

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

func normalizeDNSServerAddr(dnsServer string) string {
	if _, _, err := net.SplitHostPort(dnsServer); err == nil {
		return dnsServer
	}
	return net.JoinHostPort(dnsServer, defaultDNSPort)
}

func resolveDNSServerHostToIP(dnsServer string) string {
	host, port, err := net.SplitHostPort(dnsServer)
	if err != nil {
		return dnsServer
	}

	if ip := net.ParseIP(host); ip != nil {
		return dnsServer
	}

	ips, lookupErr := net.LookupIP(host)
	if lookupErr != nil || len(ips) == 0 {
		log.Printf("Unable to resolve DNS server host %q: %v. Using as-is.", host, lookupErr)
		return dnsServer
	}

	selectedIP := ips[0]
	for _, ip := range ips {
		if ip.To4() != nil {
			selectedIP = ip
			break
		}
	}

	return net.JoinHostPort(selectedIP.String(), port)
}

func configureCustomDNSResolver() {
	dnsServer := getCustomDNSServer()
	if dnsServer == "" {
		return
	}

	dnsServer = normalizeDNSServerAddr(dnsServer)
	dnsServer = resolveDNSServerHostToIP(dnsServer)
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
