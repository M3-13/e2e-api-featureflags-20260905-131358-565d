package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthz(t *testing.T) {
	srv := NewServer(NewStore())
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	srv.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /healthz: expected 200, got %d", rr.Code)
	}
}

func TestNewHTTPServer(t *testing.T) {
	srv := newHTTPServer()
	if srv.Addr == "" {
		t.Fatal("newHTTPServer: expected non-empty Addr")
	}
	host, _, err := net.SplitHostPort(srv.Addr)
	if err != nil {
		t.Fatalf("newHTTPServer: invalid Addr %q: %v", srv.Addr, err)
	}
	if host != "127.0.0.1" {
		t.Fatalf("newHTTPServer: expected host 127.0.0.1, got %q", host)
	}
	if srv.ReadHeaderTimeout != 5*time.Second {
		t.Fatalf("ReadHeaderTimeout = %v, want 5s", srv.ReadHeaderTimeout)
	}
	if srv.ReadTimeout != 15*time.Second {
		t.Fatalf("ReadTimeout = %v, want 15s", srv.ReadTimeout)
	}
	if srv.WriteTimeout != 15*time.Second {
		t.Fatalf("WriteTimeout = %v, want 15s", srv.WriteTimeout)
	}
	if srv.IdleTimeout != 60*time.Second {
		t.Fatalf("IdleTimeout = %v, want 60s", srv.IdleTimeout)
	}
}
