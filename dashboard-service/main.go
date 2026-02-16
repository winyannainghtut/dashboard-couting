// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"embed"
	"encoding/json"
	"expvar"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

//go:embed assets
var staticFiles embed.FS

var countingServiceURL string
var port string

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for this demo
	},
}

func main() {
	port = getEnvOrDefault("PORT", "80")
	portWithColon := fmt.Sprintf(":%s", port)

	countingServiceURL = getEnvOrDefault("COUNTING_SERVICE_URL", "http://localhost:9001")

	fmt.Printf("Starting server on http://0.0.0.0:%s\n", port)
	fmt.Println("(Pass as PORT environment variable)")
	fmt.Printf("Using counting service at %s\n", countingServiceURL)
	fmt.Println("(Pass as COUNTING_SERVICE_URL environment variable)")

	failTrack := new(failureTracker)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", wsHandler(failTrack))
	mux.HandleFunc("/health", HealthHandler)
	mux.HandleFunc("/health/api", HealthAPIHandler(failTrack))
	mux.Handle("/metrics", expvar.Handler())
	assetsFS, err := fs.Sub(staticFiles, "assets")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/", http.FileServer(http.FS(assetsFS)))

	log.Fatal(http.ListenAndServe(portWithColon, mux))
}

func getEnvOrDefault(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// HealthHandler returns a successful status and a message.
// For use by Consul or other processes that need to verify service health.
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Hello, you've hit %s\n", r.URL.Path)
}

type failureTracker struct {
	lock     sync.RWMutex
	latest   bool // indicates condition of most recent connection attempt
	failures int  // counts the number of consecutive connection failures
}

func (ft *failureTracker) Count(ok bool) {
	ft.lock.Lock()
	defer ft.lock.Unlock()

	if ft.latest = ok; ok {
		ft.failures = 0
	} else {
		ft.failures++
	}
}

func (ft *failureTracker) Status() (bool, int) {
	ft.lock.RLock()
	defer ft.lock.RUnlock()
	return ft.latest, ft.failures
}

// HealthAPIHandler returns the condition of the connectivity between the
// dashboard and the backend API server.
func HealthAPIHandler(ft *failureTracker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		statusOK, failures := ft.Status()
		if statusOK {
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "ok")
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, fmt.Sprintf(
				"failures: %d", failures,
			))
		}
	}
}

// wsHandler handles WebSocket connections from the dashboard frontend.
func wsHandler(ft *failureTracker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("WebSocket upgrade error:", err)
			return
		}
		defer conn.Close()

		fmt.Println("New WebSocket client connected")

		// Get local hostname once per connection
		localHostname, hostnameErr := os.Hostname()
		if hostnameErr != nil {
			localHostname = "Unknown"
		}

		// Read messages from client and respond with count
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					log.Println("WebSocket read error:", err)
				}
				break
			}

			count, fetchErr := getAndParseCount()
			if fetchErr != nil {
				count = Count{Count: -1, Message: fetchErr.Error(), Hostname: "[Unreachable]"}
				ft.Count(false)
			} else {
				ft.Count(true)
			}
			count.DashboardHostname = localHostname

			fmt.Println("Fetched count", count.Count)

			response, _ := json.Marshal(count)
			if writeErr := conn.WriteMessage(websocket.TextMessage, response); writeErr != nil {
				log.Println("WebSocket write error:", writeErr)
				break
			}
		}
	}
}

// Count stores a number that is being counted and other data to send to
// websocket clients.
type Count struct {
	Count             int    `json:"count"`
	Message           string `json:"message"`
	Hostname          string `json:"hostname"`
	DashboardHostname string `json:"dashboard_hostname"`
}

func getAndParseCount() (Count, error) {
	url := countingServiceURL

	tr := &http.Transport{
		IdleConnTimeout: time.Second * 1,
	}

	httpClient := http.Client{
		Timeout:   time.Second * 2,
		Transport: tr,
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return Count{}, err
	}

	req.Header.Set("User-Agent", "HashiCorp Training Lab")

	res, getErr := httpClient.Do(req)
	if getErr != nil {
		return Count{}, getErr
	}

	body, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		return Count{}, readErr
	}

	defer res.Body.Close()
	return parseCount(body)
}

func parseCount(body []byte) (Count, error) {
	count := Count{}
	err := json.Unmarshal(body, &count)
	if err != nil {
		fmt.Println(err)
		return count, err
	}
	return count, err
}
