package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testServer(t *testing.T, debug bool) (*server, http.Handler) {
	t.Helper()
	s := newServer(config{greeting: "halo", env: "test", enableDebug: debug})
	return s, s.routes()
}

func do(h http.Handler, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func TestRoot(t *testing.T) {
	_, h := testServer(t, true)
	rec := do(h, "GET", "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, mau 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"message":"halo"`) {
		t.Fatalf("body tidak berisi pesan: %s", rec.Body.String())
	}
}

func TestUnknownPathIs404(t *testing.T) {
	_, h := testServer(t, true)
	if rec := do(h, "GET", "/tidak-ada"); rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, mau 404", rec.Code)
	}
}

func TestHealthz(t *testing.T) {
	s, h := testServer(t, true)
	if rec := do(h, "GET", "/healthz"); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, mau 200", rec.Code)
	}
	s.healthy.Store(false)
	if rec := do(h, "GET", "/healthz"); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, mau 503", rec.Code)
	}
}

func TestReadyz(t *testing.T) {
	s, h := testServer(t, true)
	if rec := do(h, "GET", "/readyz"); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("sebelum ready: status = %d, mau 503", rec.Code)
	}
	s.ready.Store(true)
	if rec := do(h, "GET", "/readyz"); rec.Code != http.StatusOK {
		t.Fatalf("setelah ready: status = %d, mau 200", rec.Code)
	}
}

func TestDebugToggleReady(t *testing.T) {
	s, h := testServer(t, true)
	s.ready.Store(true)
	if rec := do(h, "POST", "/debug/ready?state=false"); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, mau 200", rec.Code)
	}
	if s.ready.Load() {
		t.Fatal("ready seharusnya false")
	}
	if rec := do(h, "POST", "/debug/ready?state=ngawur"); rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, mau 400", rec.Code)
	}
}

func TestDebugMemory(t *testing.T) {
	s, h := testServer(t, true)
	if rec := do(h, "POST", "/debug/mem?mb=2"); rec.Code != http.StatusOK {
		t.Fatalf("alokasi: status = %d, mau 200", rec.Code)
	}
	if s.leakedMB != 2 {
		t.Fatalf("leakedMB = %d, mau 2", s.leakedMB)
	}
	if rec := do(h, "DELETE", "/debug/mem"); rec.Code != http.StatusOK {
		t.Fatalf("bebaskan: status = %d, mau 200", rec.Code)
	}
	if s.leakedMB != 0 {
		t.Fatalf("leakedMB = %d, mau 0", s.leakedMB)
	}
}

func TestDebugDisabled(t *testing.T) {
	_, h := testServer(t, false)
	if rec := do(h, "GET", "/debug/cpu"); rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, mau 404 saat debug nonaktif", rec.Code)
	}
}

func TestMetricsCountRequests(t *testing.T) {
	_, h := testServer(t, true)
	do(h, "GET", "/")
	do(h, "GET", "/")
	body := do(h, "GET", "/metrics").Body.String()
	want := `http_requests_total{method="GET",path="/",code="200"} 2`
	if !strings.Contains(body, want) {
		t.Fatalf("metrics tidak berisi %q:\n%s", want, body)
	}
	if !strings.Contains(body, `http_request_duration_seconds_count{path="/"} 2`) {
		t.Fatalf("histogram tidak mencatat 2 request:\n%s", body)
	}
}

func TestProbesNotCounted(t *testing.T) {
	_, h := testServer(t, true)
	do(h, "GET", "/healthz")
	do(h, "GET", "/readyz")
	body := do(h, "GET", "/metrics").Body.String()
	if strings.Contains(body, `path="/healthz"`) || strings.Contains(body, `path="/readyz"`) {
		t.Fatalf("probe tidak boleh masuk metrik:\n%s", body)
	}
}
