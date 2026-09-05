package main

import "net/http"

func (s *Server) handleEvaluateFlag(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}
