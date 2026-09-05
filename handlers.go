package main

import (
	"encoding/json"
	"errors"
	"net/http"
)

const maxBodyBytes = 1 << 20

type updateFlagRequest struct {
	Enabled        *bool   `json:"enabled"`
	Description    *string `json:"description"`
	RolloutPercent *int    `json:"rollout_percent"`
}

func (s *Server) handleCreateFlag(w http.ResponseWriter, r *http.Request) {
	var f Flag
	if err := decodeJSONBody(w, r, &f); err != nil {
		return
	}

	if err := s.store.Create(f); err != nil {
		switch {
		case errors.Is(err, ErrEmptyKey):
			writeError(w, http.StatusBadRequest, "key must not be empty")
		case errors.Is(err, ErrKeyTooLong):
			writeError(w, http.StatusBadRequest, "key must be at most 200 characters")
		case errors.Is(err, ErrRolloutRange):
			writeError(w, http.StatusBadRequest, "rollout_percent must be between 0 and 100")
		case errors.Is(err, ErrFlagExists):
			writeError(w, http.StatusBadRequest, "flag already exists")
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusCreated, f)
}

func (s *Server) handleListFlags(w http.ResponseWriter, r *http.Request) {
	flags := s.store.List()
	writeJSON(w, http.StatusOK, flags)
}

func (s *Server) handleGetFlag(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	f, ok := s.store.Get(key)
	if !ok {
		writeError(w, http.StatusNotFound, "flag not found")
		return
	}
	writeJSON(w, http.StatusOK, f)
}

func (s *Server) handleUpdateFlag(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	var req updateFlagRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		return
	}

	if req.RolloutPercent != nil && (*req.RolloutPercent < 0 || *req.RolloutPercent > 100) {
		writeError(w, http.StatusBadRequest, "rollout_percent must be between 0 and 100")
		return
	}

	existing, ok := s.store.Get(key)
	if !ok {
		writeError(w, http.StatusNotFound, "flag not found")
		return
	}

	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}
	if req.RolloutPercent != nil {
		existing.RolloutPercent = *req.RolloutPercent
	}

	updated, ok := s.store.Update(key, existing)
	if !ok {
		writeError(w, http.StatusNotFound, "flag not found")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteFlag(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if !s.store.Delete(key) {
		writeError(w, http.StatusNotFound, "flag not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusBadRequest, "request body too large")
		} else {
			writeError(w, http.StatusBadRequest, "invalid JSON")
		}
		return err
	}
	return nil
}
