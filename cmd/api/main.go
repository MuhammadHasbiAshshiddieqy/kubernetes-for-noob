// Tokoku API: aplikasi kecil untuk latihan Kubernetes.
//
// Hanya memakai standard library supaya mudah dibaca dan tanpa dependensi.
// Endpoint /debug/* sengaja dibuat untuk latihan "rusak dan perbaiki"
// (crash, kebocoran memori, beban CPU, probe gagal). JANGAN dipakai di production.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// version diisi saat build: go build -ldflags "-X main.version=v0.1.0"
var version = "dev"

// ---------------------------------------------------------------------------
// Konfigurasi (dibaca dari environment variable, cara standar di container)
// ---------------------------------------------------------------------------

type config struct {
	port            string
	env             string
	greeting        string
	enableDebug     bool
	startupDelay    time.Duration // jeda sebelum /readyz menjadi 200
	shutdownDelay   time.Duration // jeda setelah SIGTERM sebelum berhenti menerima koneksi
	shutdownTimeout time.Duration // batas waktu menyelesaikan request yang sedang berjalan
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		slog.Warn("nilai durasi tidak valid, memakai default", "key", key, "value", v, "default", def.String())
		return def
	}
	return d
}

func loadConfig() config {
	return config{
		port:            getenv("PORT", "8080"),
		env:             getenv("APP_ENV", "local"),
		greeting:        getenv("GREETING", "Halo dari Tokoku API"),
		enableDebug:     getenv("ENABLE_DEBUG", "true") == "true",
		startupDelay:    getDuration("STARTUP_DELAY", 0),
		shutdownDelay:   getDuration("SHUTDOWN_DELAY", 0),
		shutdownTimeout: getDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
	}
}

// ---------------------------------------------------------------------------
// Metrik gaya Prometheus (ditulis manual agar tanpa dependensi).
// Di Modul 9 ganti dengan github.com/prometheus/client_golang.
// ---------------------------------------------------------------------------

var buckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

type reqKey struct {
	method string
	path   string
	code   int
}

type histogram struct {
	counts []uint64 // jumlah per bucket (tidak kumulatif); elemen terakhir = +Inf
	sum    float64
	n      uint64
}

type metrics struct {
	mu       sync.Mutex
	requests map[reqKey]uint64
	hist     map[string]*histogram
}

func newMetrics() *metrics {
	return &metrics{requests: map[reqKey]uint64{}, hist: map[string]*histogram{}}
}

func (m *metrics) observe(method, path string, code int, d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests[reqKey{method, path, code}]++
	h := m.hist[path]
	if h == nil {
		h = &histogram{counts: make([]uint64, len(buckets)+1)}
		m.hist[path] = h
	}
	sec := d.Seconds()
	i := sort.SearchFloat64s(buckets, sec) // bucket pertama dengan batas >= sec
	h.counts[i]++
	h.sum += sec
	h.n++
}

func (m *metrics) write(w io.Writer) {
	m.mu.Lock()
	defer m.mu.Unlock()

	fmt.Fprintln(w, "# HELP http_requests_total Jumlah request HTTP.")
	fmt.Fprintln(w, "# TYPE http_requests_total counter")
	keys := make([]reqKey, 0, len(m.requests))
	for k := range m.requests {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].path != keys[j].path {
			return keys[i].path < keys[j].path
		}
		if keys[i].method != keys[j].method {
			return keys[i].method < keys[j].method
		}
		return keys[i].code < keys[j].code
	})
	for _, k := range keys {
		fmt.Fprintf(w, "http_requests_total{method=%q,path=%q,code=\"%d\"} %d\n", k.method, k.path, k.code, m.requests[k])
	}

	fmt.Fprintln(w, "# HELP http_request_duration_seconds Durasi request HTTP.")
	fmt.Fprintln(w, "# TYPE http_request_duration_seconds histogram")
	paths := make([]string, 0, len(m.hist))
	for p := range m.hist {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		h := m.hist[p]
		var cum uint64
		for i, b := range buckets {
			cum += h.counts[i]
			fmt.Fprintf(w, "http_request_duration_seconds_bucket{path=%q,le=\"%g\"} %d\n", p, b, cum)
		}
		cum += h.counts[len(buckets)]
		fmt.Fprintf(w, "http_request_duration_seconds_bucket{path=%q,le=\"+Inf\"} %d\n", p, cum)
		fmt.Fprintf(w, "http_request_duration_seconds_sum{path=%q} %g\n", p, h.sum)
		fmt.Fprintf(w, "http_request_duration_seconds_count{path=%q} %d\n", p, h.n)
	}

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	fmt.Fprintln(w, "# HELP go_goroutines Jumlah goroutine.")
	fmt.Fprintln(w, "# TYPE go_goroutines gauge")
	fmt.Fprintf(w, "go_goroutines %d\n", runtime.NumGoroutine())
	fmt.Fprintln(w, "# HELP go_memstats_alloc_bytes Memori heap yang sedang dipakai.")
	fmt.Fprintln(w, "# TYPE go_memstats_alloc_bytes gauge")
	fmt.Fprintf(w, "go_memstats_alloc_bytes %d\n", ms.Alloc)
}

// ---------------------------------------------------------------------------
// Server
// ---------------------------------------------------------------------------

type server struct {
	cfg      config
	hostname string
	metrics  *metrics

	ready   atomic.Bool // dipakai /readyz
	healthy atomic.Bool // dipakai /healthz

	mu       sync.Mutex
	leak     [][]byte // memori yang sengaja "bocor" untuk latihan OOMKilled
	leakedMB int
}

func newServer(cfg config) *server {
	host := os.Getenv("POD_NAME")
	if host == "" {
		host, _ = os.Hostname()
	}
	s := &server{cfg: cfg, hostname: host, metrics: newMetrics()}
	s.healthy.Store(true)
	return s
}

type statusRecorder struct {
	http.ResponseWriter
	code int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.code = code
	r.ResponseWriter.WriteHeader(code)
}

// instrument mencatat metrik dan log untuk endpoint aplikasi.
// Probe (/healthz, /readyz) dan /metrics sengaja tidak dicatat agar tidak
// mendominasi data dan log.
func (s *server) instrument(path string, next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, code: http.StatusOK}
		next(rec, r)
		d := time.Since(start)
		s.metrics.observe(r.Method, path, rec.code, d)
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "status", rec.code, "duration_ms", d.Milliseconds())
	})
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()

	// Endpoint untuk Kubernetes
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		s.metrics.write(w)
	})

	// Endpoint aplikasi. "/{$}" artinya hanya path "/" persis.
	mux.Handle("GET /{$}", s.instrument("/", s.handleRoot))

	if s.cfg.enableDebug {
		mux.Handle("GET /debug/slow", s.instrument("/debug/slow", s.handleSlow))
		mux.Handle("GET /debug/cpu", s.instrument("/debug/cpu", s.handleCPU))
		mux.Handle("POST /debug/mem", s.instrument("/debug/mem", s.handleMemAlloc))
		mux.Handle("DELETE /debug/mem", s.instrument("/debug/mem", s.handleMemFree))
		mux.Handle("POST /debug/ready", s.instrument("/debug/ready", s.handleSetReady))
		mux.Handle("POST /debug/health", s.instrument("/debug/health", s.handleSetHealth))
		mux.Handle("POST /debug/crash", s.instrument("/debug/crash", s.handleCrash))
	}
	return mux
}

// --- handler Kubernetes -----------------------------------------------------

// Liveness: "apakah proses ini macet?" Jika gagal, container di-restart.
func (s *server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	if !s.healthy.Load() {
		http.Error(w, "unhealthy", http.StatusServiceUnavailable)
		return
	}
	fmt.Fprintln(w, "ok")
}

// Readiness: "apakah siap menerima traffic?" Jika gagal, pod dikeluarkan dari Service.
func (s *server) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	if !s.ready.Load() {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	fmt.Fprintln(w, "ready")
}

// --- handler aplikasi -------------------------------------------------------

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *server) handleRoot(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"message":  s.cfg.greeting,
		"version":  version,
		"env":      s.cfg.env,
		"hostname": s.hostname, // nama pod; berguna melihat load balancing antar replika
		"time":     time.Now().Format(time.RFC3339),
	})
}

// --- handler debug (untuk latihan) -------------------------------------------

func intParam(r *http.Request, name string, def, min, max int) int {
	v, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil {
		return def
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// GET /debug/slow?ms=500 : membalas setelah jeda (untuk latihan timeout dan 504).
func (s *server) handleSlow(w http.ResponseWriter, r *http.Request) {
	ms := intParam(r, "ms", 500, 0, 60000)
	time.Sleep(time.Duration(ms) * time.Millisecond)
	writeJSON(w, http.StatusOK, map[string]any{"slept_ms": ms, "hostname": s.hostname})
}

var sink int // mencegah compiler membuang loop di burn()

func burn(d time.Duration) {
	end := time.Now().Add(d)
	x := 0
	for time.Now().Before(end) {
		x++
	}
	sink = x
}

// GET /debug/cpu?ms=100 : memakai 1 core penuh selama sekian milidetik (untuk latihan HPA).
func (s *server) handleCPU(w http.ResponseWriter, r *http.Request) {
	ms := intParam(r, "ms", 100, 1, 5000)
	burn(time.Duration(ms) * time.Millisecond)
	writeJSON(w, http.StatusOK, map[string]any{"burned_ms": ms, "hostname": s.hostname})
}

// POST /debug/mem?mb=50 : menahan memori (untuk latihan OOMKilled). Maksimal 1024 MB total.
func (s *server) handleMemAlloc(w http.ResponseWriter, r *http.Request) {
	mb := intParam(r, "mb", 50, 1, 512)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.leakedMB+mb > 1024 {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "batas 1024 MB terlampaui", "leaked_mb": s.leakedMB})
		return
	}
	b := make([]byte, mb<<20)
	for i := 0; i < len(b); i += 4096 { // sentuh tiap halaman agar benar-benar terpakai
		b[i] = 1
	}
	s.leak = append(s.leak, b)
	s.leakedMB += mb
	writeJSON(w, http.StatusOK, map[string]any{"leaked_mb": s.leakedMB, "hostname": s.hostname})
}

// DELETE /debug/mem : melepas semua memori yang ditahan.
func (s *server) handleMemFree(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	s.leak = nil
	s.leakedMB = 0
	s.mu.Unlock()
	runtime.GC()
	writeJSON(w, http.StatusOK, map[string]any{"leaked_mb": 0, "hostname": s.hostname})
}

func parseState(r *http.Request) (bool, bool) {
	switch r.URL.Query().Get("state") {
	case "true":
		return true, true
	case "false":
		return false, true
	}
	return false, false
}

// POST /debug/ready?state=false : membuat /readyz gagal (pod keluar dari Service, tidak di-restart).
func (s *server) handleSetReady(w http.ResponseWriter, r *http.Request) {
	state, ok := parseState(r)
	if !ok {
		http.Error(w, "pakai ?state=true atau ?state=false", http.StatusBadRequest)
		return
	}
	s.ready.Store(state)
	writeJSON(w, http.StatusOK, map[string]any{"ready": state, "hostname": s.hostname})
}

// POST /debug/health?state=false : membuat /healthz gagal (liveness probe gagal, container di-restart).
func (s *server) handleSetHealth(w http.ResponseWriter, r *http.Request) {
	state, ok := parseState(r)
	if !ok {
		http.Error(w, "pakai ?state=true atau ?state=false", http.StatusBadRequest)
		return
	}
	s.healthy.Store(state)
	writeJSON(w, http.StatusOK, map[string]any{"healthy": state, "hostname": s.hostname})
}

// POST /debug/crash : proses keluar dengan kode 1 (untuk latihan CrashLoopBackOff dan restart).
func (s *server) handleCrash(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"crashing": true, "hostname": s.hostname})
	go func() {
		time.Sleep(100 * time.Millisecond) // beri waktu response terkirim
		slog.Error("crash sengaja dipicu lewat /debug/crash")
		os.Exit(1)
	}()
}

// ---------------------------------------------------------------------------
// main: menjalankan server dan berhenti dengan rapi saat menerima SIGTERM
// ---------------------------------------------------------------------------

func run() error {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg := loadConfig()
	s := newServer(cfg)

	srv := &http.Server{
		Addr:              ":" + cfg.port,
		Handler:           s.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// SIGTERM dikirim Kubernetes saat pod akan dimatikan; SIGINT = Ctrl+C di lokal.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		slog.Info("server dimulai", "port", cfg.port, "version", version, "env", cfg.env, "debug", cfg.enableDebug)
		errCh <- srv.ListenAndServe()
	}()

	// Server baru dianggap "ready" setelah startupDelay (simulasi aplikasi yang lambat start).
	go func() {
		time.Sleep(cfg.startupDelay)
		s.ready.Store(true)
		slog.Info("siap menerima traffic")
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
	}
	stop() // sinyal kedua langsung mematikan proses (perilaku bawaan)

	slog.Info("sinyal berhenti diterima, mulai graceful shutdown", "delay", cfg.shutdownDelay.String())
	s.ready.Store(false)          // keluar dari Service
	time.Sleep(cfg.shutdownDelay) // beri waktu Service/load balancer berhenti mengirim request baru

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	slog.Info("server berhenti dengan rapi")
	return nil
}

func main() {
	if err := run(); err != nil {
		slog.Error("server berhenti dengan error", "error", err)
		os.Exit(1)
	}
}
