package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"regexp"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

var db *DB

var (
	runSem     = make(chan struct{}, 5)
	snippetRe  = regexp.MustCompile(`^[a-zA-Z0-9]{8}$`)
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8090", "listen address")
	dbPath := flag.String("db", "playground.db", "SQLite database path")
	flag.Parse()

	var err error
	db, err = NewDB(*dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", handleHealth)
	mux.HandleFunc("POST /api/run", handleRun)
	mux.HandleFunc("POST /api/share", handleShare)
	mux.HandleFunc("GET /api/snippet/{id}", handleSnippet)

	handler := corsMiddleware(rateLimitMiddleware(mux))

	log.Printf("playground server listening on %s", *addr)
	srv := &http.Server{
		Addr:         *addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "https://sans.dev")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type ipLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
}

func newIPLimiter() *ipLimiter {
	l := &ipLimiter{limiters: make(map[string]*rate.Limiter)}
	go l.cleanup()
	return l
}

func (l *ipLimiter) get(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	if lim, ok := l.limiters[ip]; ok {
		return lim
	}
	// 10 requests per minute, burst of 5
	lim := rate.NewLimiter(rate.Every(6*time.Second), 5)
	l.limiters[ip] = lim
	return lim
}

func (l *ipLimiter) cleanup() {
	for {
		time.Sleep(10 * time.Minute)
		l.mu.Lock()
		l.limiters = make(map[string]*rate.Limiter)
		l.mu.Unlock()
	}
}

var limiter = newIPLimiter()

func rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only rate limit mutation endpoints
		if r.URL.Path == "/api/run" || r.URL.Path == "/api/share" {
			ip := r.RemoteAddr
			if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
				ip = fwd
			}
			if !limiter.get(ip).Allow() {
				http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		log.Printf("failed to encode health response: %v", err)
	}
}

func handleRun(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	if req.Code == "" {
		http.Error(w, `{"error":"code is required"}`, http.StatusBadRequest)
		return
	}

	result := runCode(r.Context(), req.Code)
	db.LogCompile(len(req.Code), result.CompileSuccess)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Printf("failed to encode run response: %v", err)
	}
}

func handleShare(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	if req.Code == "" {
		http.Error(w, `{"error":"code is required"}`, http.StatusBadRequest)
		return
	}
	id, err := db.SaveSnippet(req.Code)
	if err != nil {
		log.Printf("save snippet error: %v", err)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"id": id}); err != nil {
		log.Printf("failed to encode share response: %v", err)
	}
}

func handleSnippet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !snippetRe.MatchString(id) {
		http.Error(w, `{"error":"invalid snippet id"}`, http.StatusBadRequest)
		return
	}
	code, err := db.GetSnippet(id)
	if err != nil {
		http.Error(w, `{"error":"snippet not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"id": id, "code": code}); err != nil {
		log.Printf("failed to encode snippet response: %v", err)
	}
}
