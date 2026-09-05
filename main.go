package main

import (
	"log"
	"net/http"
	"os"
	"time"
)

type Server struct {
	store *Store
}

func NewServer(store *Store) *Server {
	return &Server{store: store}
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("POST /flags", s.handleCreateFlag)
	mux.HandleFunc("GET /flags", s.handleListFlags)
	mux.HandleFunc("GET /flags/{key}", s.handleGetFlag)
	mux.HandleFunc("PUT /flags/{key}", s.handleUpdateFlag)
	mux.HandleFunc("DELETE /flags/{key}", s.handleDeleteFlag)
	mux.HandleFunc("GET /flags/{key}/evaluate", s.handleEvaluateFlag)
	return Logging(mux)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func newHTTPServer() *http.Server {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return &http.Server{
		Addr:              "127.0.0.1:" + port,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

func main() {
	server := NewServer(NewStore())
	httpServer := newHTTPServer()
	httpServer.Handler = server.routes()
	if err := httpServer.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
