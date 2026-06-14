package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/terraform-drift-detector/terraform_drift_detector/internal/models"
	_ "modernc.org/sqlite"
)

// Store persists drift scan reports.
type Store struct {
	db *sql.DB
}

// Open opens or creates a SQLite database.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS scans (
			scan_id TEXT PRIMARY KEY,
			workspace TEXT NOT NULL,
			state_source TEXT NOT NULL,
			provider TEXT NOT NULL,
			region TEXT,
			started_at TEXT NOT NULL,
			finished_at TEXT NOT NULL,
			duration TEXT NOT NULL,
			summary_json TEXT NOT NULL,
			report_json TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_scans_workspace ON scans(workspace);
		CREATE INDEX IF NOT EXISTS idx_scans_started_at ON scans(started_at);
	`)
	return err
}

// SaveReport persists a drift report.
func (s *Store) SaveReport(report *models.DriftReport) error {
	summaryJSON, err := json.Marshal(report.Summary)
	if err != nil {
		return err
	}
	reportJSON, err := json.Marshal(report)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
		INSERT INTO scans (scan_id, workspace, state_source, provider, region, started_at, finished_at, duration, summary_json, report_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		report.ScanID,
		report.Workspace,
		report.StateSource,
		report.Provider,
		report.Region,
		report.StartedAt.Format(time.RFC3339),
		report.FinishedAt.Format(time.RFC3339),
		report.Duration,
		string(summaryJSON),
		string(reportJSON),
	)
	return err
}

// ScanMeta is lightweight scan metadata for listings.
type ScanMeta struct {
	ScanID    string                `json:"scan_id"`
	Workspace string                `json:"workspace"`
	Provider  string                `json:"provider"`
	Region    string                `json:"region,omitempty"`
	StartedAt time.Time             `json:"started_at"`
	Duration  string                `json:"duration"`
	Summary   models.DriftSummary   `json:"summary"`
}

// ListScans returns recent scan metadata.
func (s *Store) ListScans(limit int) ([]ScanMeta, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`
		SELECT scan_id, workspace, provider, region, started_at, duration, summary_json
		FROM scans ORDER BY started_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ScanMeta
	for rows.Next() {
		var meta ScanMeta
		var started string
		var summaryJSON string
		var region sql.NullString
		if err := rows.Scan(&meta.ScanID, &meta.Workspace, &meta.Provider, &region, &started, &meta.Duration, &summaryJSON); err != nil {
			return nil, err
		}
		if region.Valid {
			meta.Region = region.String
		}
		meta.StartedAt, _ = time.Parse(time.RFC3339, started)
		_ = json.Unmarshal([]byte(summaryJSON), &meta.Summary)
		out = append(out, meta)
	}
	return out, rows.Err()
}

// GetReport loads a full drift report by scan ID.
func (s *Store) GetReport(scanID string) (*models.DriftReport, error) {
	var reportJSON string
	err := s.db.QueryRow(`SELECT report_json FROM scans WHERE scan_id = ?`, scanID).Scan(&reportJSON)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("scan %s not found", scanID)
	}
	if err != nil {
		return nil, err
	}
	var report models.DriftReport
	if err := json.Unmarshal([]byte(reportJSON), &report); err != nil {
		return nil, err
	}
	return &report, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
