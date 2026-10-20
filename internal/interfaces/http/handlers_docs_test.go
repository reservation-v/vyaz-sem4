package http

import (
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"coworking/internal/application"
	"coworking/internal/infrastructure/memory"
)

func newDocsTestServer(t *testing.T) *Server {
	t.Helper()

	catalog := application.NewServiceCatalog(memory.NewServiceRepository())
	srv, err := NewServer(catalog)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	return srv
}

func TestDocsPage(t *testing.T) {
	t.Parallel()

	srv := newDocsTestServer(t)

	for _, target := range []string{"/docs", "/docs/"} {
		req := httptest.NewRequest(nethttp.MethodGet, target, nil)
		rec := httptest.NewRecorder()
		srv.Router().ServeHTTP(rec, req)

		if rec.Code != nethttp.StatusOK {
			t.Fatalf("GET %s code = %d, want 200", target, rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
			t.Fatalf("GET %s Content-Type = %q, want text/html", target, got)
		}
		if body := rec.Body.String(); !strings.Contains(body, "Общее описание технологии") {
			t.Fatalf("GET %s body does not contain documentation title", target)
		}
	}
}

func TestHomeLinksToDocs(t *testing.T) {
	t.Parallel()

	srv := newDocsTestServer(t)

	req := httptest.NewRequest(nethttp.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusOK {
		t.Fatalf("GET / code = %d, want 200", rec.Code)
	}

	body := rec.Body.String()
	for _, want := range []string{`href="/docs"`, "Документация"} {
		if !strings.Contains(body, want) {
			t.Fatalf("GET / body does not contain %q", want)
		}
	}
}
