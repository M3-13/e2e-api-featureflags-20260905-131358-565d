package main

import (
	"hash/fnv"
	"net/http"
)

func (s *Server) handleEvaluateFlag(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if len(key) > 200 {
		writeError(w, http.StatusBadRequest, "key too long")
		return
	}

	user := r.URL.Query().Get("user")
	if user == "" || len(user) > 200 {
		writeError(w, http.StatusBadRequest, "user required and must be at most 200 characters")
		return
	}

	flag, ok := s.store.Get(key)
	if !ok {
		writeError(w, http.StatusNotFound, "flag not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"key":     flag.Key,
		"enabled": rolloutEnabled(flag.Key, user, flag.RolloutPercent),
	})
}

func rolloutEnabled(key, user string, rolloutPercent int) bool {
	if rolloutPercent <= 0 {
		return false
	}
	if rolloutPercent >= 100 {
		return true
	}

	h := fnv.New64a()
	h.Write([]byte(key))
	h.Write([]byte{0})
	h.Write([]byte(user))
	return (h.Sum64() % 100) < uint64(rolloutPercent)
}
