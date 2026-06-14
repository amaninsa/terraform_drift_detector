package api

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/terraform-drift-detector/terraform_drift_detector/internal/config"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/notify"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/cloud"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/scan"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/store"
)

// Server exposes drift scan data over HTTP.
type Server struct {
	Store    *store.Store
	Config   *config.Config
	StaticFS http.FileSystem
}

// NewMux returns the HTTP handler for the API and dashboard.
func (s *Server) NewMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/scans", s.handleScans)
	mux.HandleFunc("/api/v1/scans/", s.handleScanByID)
	mux.HandleFunc("/api/v1/workspaces", s.handleWorkspaces)
	if s.StaticFS != nil {
		mux.Handle("/", http.FileServer(s.StaticFS))
	}
	return mux
}

func (s *Server) handleScans(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
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
	case http.MethodPost:
		s.triggerScan(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

type scanRequest struct {
	Workspace string `json:"workspace"`
}

func (s *Server) triggerScan(w http.ResponseWriter, r *http.Request) {
	if s.Config == nil {
		http.Error(w, "server config not loaded; start with --config", http.StatusBadRequest)
		return
	}
	var req scanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Workspace == "" && len(s.Config.Workspaces) == 1 {
		req.Workspace = s.Config.Workspaces[0].Name
	}
	ws, err := s.Config.GetWorkspace(req.Workspace)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	opts := ws.ToScanOptions(s.Config.Drift)
	ctx := r.Context()
	adapter, err := cloud.NewAdapterFromScanOptions(ctx, opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	report, err := scan.NewRunner(nil, adapter).Run(ctx, opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.Store.SaveReport(report); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = notify.NewClient().NotifyDrift(ctx, s.Config.ActiveWebhooks(), report)
	writeJSON(w, report)
}

func (s *Server) handleWorkspaces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.Config == nil {
		writeJSON(w, []config.WorkspaceConfig{})
		return
	}
	writeJSON(w, s.Config.Workspaces)
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
