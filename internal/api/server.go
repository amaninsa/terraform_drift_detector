package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/terraform-drift-detector/terraform_drift_detector/internal/store"
)

// Server exposes drift scan data over HTTP.
type Server struct {
	Store    *store.Store
	StaticFS http.FileSystem
}

// NewMux returns the HTTP handler for the API and dashboard.
func (s *Server) NewMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/scans", s.handleScans)
	mux.HandleFunc("/api/v1/scans/", s.handleScanByID)
	if s.StaticFS != nil {
		mux.Handle("/", http.FileServer(s.StaticFS))
	}
	return mux
}

func (s *Server) handleScans(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	scans, err := s.Store.ListScans(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, scans)
}

func (s *Server) handleScanByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/scans/")
	id = strings.TrimSuffix(id, "/json")
	id = strings.Trim(id, "/")
	if id == "" {
		http.Error(w, "scan id required", http.StatusBadRequest)
		return
	}
	report, err := s.Store.GetReport(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if strings.HasSuffix(r.URL.Path, "/json") {
		w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(id)+".json")
	}
	writeJSON(w, report)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
