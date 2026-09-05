package main

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoggingRecordsMethodPathStatusDuration(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(log.Default().Writer())

	dummy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	h := Logging(dummy)
	req := httptest.NewRequest(http.MethodPost, "/flags/myflag/evaluate?user=secret", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	out := buf.String()
	if !strings.Contains(out, http.MethodPost) {
		t.Errorf("log should contain method %q, got %q", http.MethodPost, out)
	}
	if !strings.Contains(out, "/flags/myflag/evaluate") {
		t.Errorf("log should contain path %q, got %q", "/flags/myflag/evaluate", out)
	}
	if !strings.Contains(out, "201") {
		t.Errorf("log should contain status code 201, got %q", out)
	}
	if !strings.Contains(out, "µs") && !strings.Contains(out, "ms") && !strings.Contains(out, "ns") && !strings.Contains(out, "s") {
		t.Errorf("log should contain a duration, got %q", out)
	}
}

func TestLoggingDoesNotLogQueryString(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(log.Default().Writer())

	dummy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h := Logging(dummy)
	req := httptest.NewRequest(http.MethodGet, "/flags/x/evaluate?user=secret", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	out := buf.String()
	if strings.Contains(out, "secret") {
		t.Errorf("log must not contain the query value, got %q", out)
	}
	if strings.Contains(out, "user=") {
		t.Errorf("log must not contain the query string, got %q", out)
	}
}
