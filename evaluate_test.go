package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func seedFlag(t *testing.T, s *Server, key string, rolloutPercent int) {
	t.Helper()
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	s.store.flags[key] = Flag{Key: key, RolloutPercent: rolloutPercent}
}

func doEvaluate(t *testing.T, s *Server, key, user string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/flags/"+key+"/evaluate?user="+user, nil)
	rr := httptest.NewRecorder()
	s.routes().ServeHTTP(rr, req)
	return rr
}

func decodeEvaluate(t *testing.T, rr *httptest.ResponseRecorder) (string, bool) {
	t.Helper()
	var body struct {
		Key     string `json:"key"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return body.Key, body.Enabled
}

func snapshotStore(s *Store) map[string]Flag {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := make(map[string]Flag, len(s.flags))
	for k, v := range s.flags {
		m[k] = v
	}
	return m
}

func TestEvaluateDeterministic(t *testing.T) {
	s := NewServer(NewStore())
	seedFlag(t, s, "feature-a", 50)

	var first string
	for i := 0; i < 10; i++ {
		rr := doEvaluate(t, s, "feature-a", "user-42")
		if rr.Code != http.StatusOK {
			t.Fatalf("evaluate: expected 200, got %d", rr.Code)
		}
		key, enabled := decodeEvaluate(t, rr)
		if key != "feature-a" {
			t.Fatalf("evaluate: expected key feature-a, got %q", key)
		}
		got := "false"
		if enabled {
			got = "true"
		}
		if first == "" {
			first = got
		} else if got != first {
			t.Fatalf("evaluate: non-deterministic result for same key+user: %s vs %s", first, got)
		}
	}
}

func TestEvaluateRolloutZeroAlwaysFalse(t *testing.T) {
	s := NewServer(NewStore())
	seedFlag(t, s, "feature-a", 0)

	for _, user := range []string{"a", "b", "c", "user-42", "z"} {
		rr := doEvaluate(t, s, "feature-a", user)
		if rr.Code != http.StatusOK {
			t.Fatalf("evaluate: expected 200, got %d", rr.Code)
		}
		_, enabled := decodeEvaluate(t, rr)
		if enabled {
			t.Fatalf("rollout_percent=0 should always be disabled, got enabled for user %q", user)
		}
	}
}

func TestEvaluateRolloutHundredAlwaysTrue(t *testing.T) {
	s := NewServer(NewStore())
	seedFlag(t, s, "feature-a", 100)

	for _, user := range []string{"a", "b", "c", "user-42", "z"} {
		rr := doEvaluate(t, s, "feature-a", user)
		if rr.Code != http.StatusOK {
			t.Fatalf("evaluate: expected 200, got %d", rr.Code)
		}
		_, enabled := decodeEvaluate(t, rr)
		if !enabled {
			t.Fatalf("rollout_percent=100 should always be enabled, got disabled for user %q", user)
		}
	}
}

func TestEvaluateMissingUser(t *testing.T) {
	s := NewServer(NewStore())
	seedFlag(t, s, "feature-a", 50)

	req := httptest.NewRequest(http.MethodGet, "/flags/feature-a/evaluate", nil)
	rr := httptest.NewRecorder()
	s.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing user: expected 400, got %d", rr.Code)
	}
}

func TestEvaluateEmptyUser(t *testing.T) {
	s := NewServer(NewStore())
	seedFlag(t, s, "feature-a", 50)

	rr := doEvaluate(t, s, "feature-a", "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("empty user: expected 400, got %d", rr.Code)
	}
}

func TestEvaluateUserTooLong(t *testing.T) {
	s := NewServer(NewStore())
	seedFlag(t, s, "feature-a", 50)

	rr := doEvaluate(t, s, "feature-a", strings.Repeat("x", 201))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("too-long user: expected 400, got %d", rr.Code)
	}
}

func TestEvaluateUnknownKey(t *testing.T) {
	s := NewServer(NewStore())

	rr := doEvaluate(t, s, "does-not-exist", "user-42")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown key: expected 404, got %d", rr.Code)
	}
}

func TestEvaluateKeyTooLong(t *testing.T) {
	s := NewServer(NewStore())

	rr := doEvaluate(t, s, strings.Repeat("k", 201), "user-42")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("too-long key: expected 400, got %d", rr.Code)
	}
}

func TestEvaluateStoreUnchanged(t *testing.T) {
	s := NewServer(NewStore())
	seedFlag(t, s, "feature-a", 50)
	seedFlag(t, s, "feature-b", 100)

	before := snapshotStore(s.store)

	rr := doEvaluate(t, s, "feature-a", "user-42")
	if rr.Code != http.StatusOK {
		t.Fatalf("evaluate: expected 200, got %d", rr.Code)
	}

	after := snapshotStore(s.store)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("evaluate must not mutate the store: before=%v after=%v", before, after)
	}
}
