package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func doRequest(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestCreateAndGetFlag(t *testing.T) {
	srv := NewServer(NewStore())
	h := srv.routes()

	rr := doRequest(t, h, http.MethodPost, "/flags", `{"key":"feature-x","enabled":true,"description":"my flag","rollout_percent":50}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("POST /flags = %d, want 201", rr.Code)
	}

	var created Flag
	if err := json.NewDecoder(rr.Body).Decode(&created); err != nil {
		t.Fatalf("decode created flag: %v", err)
	}
	if created.Key != "feature-x" || !created.Enabled || created.Description != "my flag" || created.RolloutPercent != 50 {
		t.Fatalf("created flag = %+v", created)
	}

	rr = doRequest(t, h, http.MethodGet, "/flags", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /flags = %d, want 200", rr.Code)
	}
	var list []Flag
	if err := json.NewDecoder(rr.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 || list[0].Key != "feature-x" {
		t.Fatalf("list = %+v, want single flag feature-x", list)
	}

	rr = doRequest(t, h, http.MethodGet, "/flags/feature-x", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /flags/feature-x = %d, want 200", rr.Code)
	}
	var got Flag
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode single flag: %v", err)
	}
	if got.Key != "feature-x" {
		t.Fatalf("got = %+v", got)
	}
}

func TestCreateInvalidInputs(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"empty key", `{"key":""}`},
		{"missing key", `{"enabled":true}`},
		{"invalid json", `{"key":`},
		{"rollout negative", `{"key":"a","rollout_percent":-1}`},
		{"rollout too high", `{"key":"a","rollout_percent":101}`},
		{"key too long", `{"key":"` + strings.Repeat("a", 201) + `"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := NewServer(NewStore())
			h := srv.routes()
			rr := doRequest(t, h, http.MethodPost, "/flags", tt.body)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("POST /flags = %d, want 400", rr.Code)
			}
			var body map[string]string
			if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
				t.Fatalf("decode error body: %v", err)
			}
			if body["error"] == "" {
				t.Fatalf("error body = %+v, want non-empty error", body)
			}
			if len(body) != 1 {
				t.Fatalf("error body has %d fields, want only 'error'", len(body))
			}

			// Store must be unchanged.
			rr = doRequest(t, h, http.MethodGet, "/flags", "")
			var list []Flag
			_ = json.NewDecoder(rr.Body).Decode(&list)
			if len(list) != 0 {
				t.Fatalf("store changed after invalid create: %+v", list)
			}
		})
	}
}

func TestCreateBodyTooLarge(t *testing.T) {
	srv := NewServer(NewStore())
	h := srv.routes()
	body := `{"key":"big","description":"` + strings.Repeat("x", maxBodyBytes) + `"}`
	rr := doRequest(t, h, http.MethodPost, "/flags", body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("POST /flags with oversized body = %d, want 400", rr.Code)
	}
	var errBody map[string]string
	_ = json.NewDecoder(rr.Body).Decode(&errBody)
	if errBody["error"] == "" {
		t.Fatalf("error body = %+v, want error", errBody)
	}
}

func TestGetFlagNotFound(t *testing.T) {
	srv := NewServer(NewStore())
	h := srv.routes()
	rr := doRequest(t, h, http.MethodGet, "/flags/unknown", "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("GET /flags/unknown = %d, want 404", rr.Code)
	}
	var body map[string]string
	_ = json.NewDecoder(rr.Body).Decode(&body)
	if body["error"] == "" {
		t.Fatalf("error body = %+v, want error", body)
	}
}

func TestUpdateFlag(t *testing.T) {
	srv := NewServer(NewStore())
	h := srv.routes()

	doRequest(t, h, http.MethodPost, "/flags", `{"key":"k","enabled":false,"description":"old","rollout_percent":10}`)

	rr := doRequest(t, h, http.MethodPut, "/flags/k", `{"enabled":true,"description":"new","rollout_percent":90}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT /flags/k = %d, want 200", rr.Code)
	}
	var updated Flag
	if err := json.NewDecoder(rr.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated flag: %v", err)
	}
	if !updated.Enabled || updated.Description != "new" || updated.RolloutPercent != 90 {
		t.Fatalf("updated = %+v", updated)
	}

	// Persisted in store.
	rr = doRequest(t, h, http.MethodGet, "/flags/k", "")
	var got Flag
	_ = json.NewDecoder(rr.Body).Decode(&got)
	if !got.Enabled || got.Description != "new" || got.RolloutPercent != 90 {
		t.Fatalf("persisted = %+v", got)
	}
}

func TestUpdatePartialFields(t *testing.T) {
	srv := NewServer(NewStore())
	h := srv.routes()

	doRequest(t, h, http.MethodPost, "/flags", `{"key":"k","enabled":false,"description":"keep","rollout_percent":10}`)

	// Only update description; enabled and rollout must be preserved.
	rr := doRequest(t, h, http.MethodPut, "/flags/k", `{"description":"changed"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT /flags/k = %d, want 200", rr.Code)
	}
	var got Flag
	_ = json.NewDecoder(rr.Body).Decode(&got)
	if got.Enabled != false || got.Description != "changed" || got.RolloutPercent != 10 {
		t.Fatalf("partial update = %+v, want enabled=false description=changed rollout=10", got)
	}
}

func TestUpdateFlagErrors(t *testing.T) {
	srv := NewServer(NewStore())
	h := srv.routes()

	// Unknown key.
	rr := doRequest(t, h, http.MethodPut, "/flags/unknown", `{"enabled":true}`)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("PUT /flags/unknown = %d, want 404", rr.Code)
	}

	doRequest(t, h, http.MethodPost, "/flags", `{"key":"k","rollout_percent":50}`)

	// Invalid rollout.
	rr = doRequest(t, h, http.MethodPut, "/flags/k", `{"rollout_percent":200}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("PUT /flags/k invalid rollout = %d, want 400", rr.Code)
	}

	// Invalid JSON.
	rr = doRequest(t, h, http.MethodPut, "/flags/k", `{"enabled":`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("PUT /flags/k invalid json = %d, want 400", rr.Code)
	}
}

func TestDeleteFlag(t *testing.T) {
	srv := NewServer(NewStore())
	h := srv.routes()

	doRequest(t, h, http.MethodPost, "/flags", `{"key":"k"}`)

	rr := doRequest(t, h, http.MethodDelete, "/flags/k", "")
	if rr.Code != http.StatusNoContent {
		t.Fatalf("DELETE /flags/k = %d, want 204", rr.Code)
	}

	rr = doRequest(t, h, http.MethodGet, "/flags/k", "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("GET after delete = %d, want 404", rr.Code)
	}

	// Deleting again.
	rr = doRequest(t, h, http.MethodDelete, "/flags/k", "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("DELETE again = %d, want 404", rr.Code)
	}
}
