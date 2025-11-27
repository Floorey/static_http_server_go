package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// ------------------------------------------------------------
// Config / Helpers
// ------------------------------------------------------------

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// ------------------------------------------------------------
// Security
// ------------------------------------------------------------

func tooManyRequests() bool {
	// simple global rate-liming
	// later token bucket / leaky bucket
	now := time.Now()

	// 1 sec "Windows"
	if now.Sub(rateWindowsStart.Load().(time.Time)) >= time.Second {
		rateWindowsStart.Store(now)
		atomic.StoreInt64(&rateCount, 0)
	}
	if atomic.AddInt64(&rateCount, 1) > 200 {
		return true
	}
	return false
}

var rateCount int64
var rateWindowsStart atomic.Value

func init() {
	rateWindowsStart.Store(time.Now())
}

func hasSQLAttack(s string) bool {
	patterns := []string{
		"SELECT ", "INSERT ", "UPDATE ", "DELETE ", "DROP ",
		"ALTER ", "EXEC ", "--", "/*", "*/",
		" OR ", " AND ", "1=1", "UNION", "XP_",
	}
	u := strings.ToUpper(s)
	for _, p := range patterns {
		if strings.Contains(u, p) {
			return true
		}
	}
	return false
}

func isPathSafe(p string) bool {
	if strings.Contains(p, "..") {
		return false
	}
	if strings.Contains(p, "\\") {
		return false
	}
	if strings.Contains(p, "%2e") {
		return false
	}
	return true
}

func hasAllowedExt(p string) bool {
	if p == "/" {
		return true // index.html
	}
	allowed := []string{
		".html", ".css", ".js", ".png",
		".jpg", ".jpeg", ".svg", ".pdf",
		".ttf", ".woff", ".woff2",
	}
	for _, e := range allowed {
		if strings.HasSuffix(p, e) {
			return true
		}
	}
	return false
}

func securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if tooManyRequests() {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusTooManyRequests)
			return
		}
		if len(r.RequestURI) > 4096 {
			http.Error(w, "Forbidden", http.StatusRequestEntityTooLarge)
			return
		}
		if !isPathSafe(r.URL.Path) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		if hasSQLAttack(r.URL.Path) || hasSQLAttack(r.RequestURI) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		if !hasAllowedExt(r.URL.Path) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ------------------------------------------------------------
// Logging + Metriken
// ------------------------------------------------------------
type accessLogEntry struct {
	Time       time.Time
	Method     string
	Path       string
	RemoteIP   string
	UserAgent  string
	StatusCode int
	Duration   time.Duration
	Bytes      int64
}

var (
	logCh           = make(chan accessLogEntry, 1024)
	totalRequests   int64
	total2xx        int64
	total4xx        int64
	total5xx        int64
	serverStartTime = time.Now()
)

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += int64(n)
	return n, err
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}

		next.ServeHTTP(rec, r)

		dur := time.Since(start)

		atomic.AddInt64(&totalRequests, 1)

		switch {
		case rec.status >= 200 && rec.status < 300:
			atomic.AddInt64(&total2xx, 1)
		case rec.status >= 400 && rec.status < 500:
			atomic.AddInt64(&total4xx, 1)
		case rec.status >= 500:
			atomic.AddInt64(&total5xx, 1)
		}
		remoteIP := r.RemoteAddr
		if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
			remoteIP = strings.Split(xf, ",")[0]
		}
		entry := accessLogEntry{
			Time:       start,
			Method:     r.Method,
			Path:       r.URL.Path,
			RemoteIP:   remoteIP,
			UserAgent:  r.UserAgent(),
			StatusCode: rec.status,
			Duration:   dur,
			Bytes:      rec.bytes,
		}

		select {
		case logCh <- entry:
		default:
			// drop
		}
	})
}

func startAccessLogWoker() {
	logger := log.New(os.Stdout, "[ACCESS]", log.LstdFlags|log.Lmicroseconds)
	go func() {
		for e := range logCh {
			logger.Printf("%s %s %s %d %dB %s UA=%q",
				e.RemoteIP,
				e.Method,
				e.Path,
				e.StatusCode,
				e.Bytes,
				e.Duration,
				e.UserAgent)
		}
	}()
}

// ------------------------------------------------------------
// Handlers für Health / Stats
// ------------------------------------------------------------

func healthHandle(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(serverStartTime).Round(time.Second)
	tr := atomic.LoadInt64(&totalRequests)
	t2 := atomic.LoadInt64(&total2xx)
	t4 := atomic.LoadInt64(&total4xx)
	t5 := atomic.LoadInt64(&total5xx)

	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w,
		`{"uptime":"%s","total":%d,"2xx":%d,"4xx":%d,"5xx":%d}`,
		uptime, tr, t2, t4, t5,
	)
}

func main() {
	addr := ":" + envOr("PORT", strconv.Itoa(8080))

	public := http.FileServer(http.Dir("public"))

	mux := http.NewServeMux()
	mux.Handle("/", public)
	mux.HandleFunc("/healthz", healthHandle)
	mux.HandleFunc("/stats", statsHandler)

	handler := securityMiddleware(loggingMiddleware(mux))

	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}
	startAccessLogWoker()

	log.Printf("Go-Server listening on %s", addr)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
