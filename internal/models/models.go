package models

import "time"

// Resource represents a cloud resource in the canonical model.
type Resource struct {
	ID         string            `json:"id"`
	Address    string            `json:"address"`
	Provider   string            `json:"provider"`
	Type       string            `json:"type"`
	CloudID    string            `json:"cloud_id"`
	Region     string            `json:"region,omitempty"`
	Attributes map[string]any    `json:"attributes"`
	Tags       map[string]string `json:"tags,omitempty"`
	Source     string            `json:"source"`
}

// DriftType classifies the kind of drift detected.
type DriftType string

const (
	DriftTypeMissing   DriftType = "missing"
	DriftTypeModified  DriftType = "modified"
	DriftTypeTagOnly   DriftType = "tag_only"
	DriftTypeUnmanaged DriftType = "unmanaged"
	DriftTypeFetchErr  DriftType = "fetch_error"
	DriftTypeInSync    DriftType = "in_sync"
)

// FieldDiff describes a single attribute difference.
type FieldDiff struct {
	Path     string `json:"path"`
	Expected any    `json:"expected,omitempty"`
	Actual   any    `json:"actual,omitempty"`
}

// DriftItem is a single resource drift result.
type DriftItem struct {
	ResourceID string            `json:"resource_id"`
	Address    string            `json:"address"`
	Type       string            `json:"type"`
	Provider   string            `json:"provider"`
	CloudID    string            `json:"cloud_id,omitempty"`
	DriftType  DriftType         `json:"drift_type"`
	Message    string            `json:"message,omitempty"`
	Expected   *Resource         `json:"expected,omitempty"`
	Actual     *Resource         `json:"actual,omitempty"`
	Diff       []FieldDiff       `json:"diff,omitempty"`
	TagsDiff   map[string]TagDiff `json:"tags_diff,omitempty"`
}

// TagDiff captures tag-level changes.
type TagDiff struct {
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
	Status   string `json:"status"` // added, removed, changed
}

// DriftSummary aggregates scan statistics.
type DriftSummary struct {
	TotalResources int `json:"total_resources"`
	InSync         int `json:"in_sync"`
	Drifted        int `json:"drifted"`
	Missing        int `json:"missing"`
	Modified       int `json:"modified"`
	TagOnly        int `json:"tag_only"`
	Unmanaged      int `json:"unmanaged"`
	FetchErrors    int `json:"fetch_errors"`
}

// DriftReport is the full output of a drift scan.
type DriftReport struct {
	ScanID      string       `json:"scan_id"`
	Workspace   string       `json:"workspace"`
	StateSource string       `json:"state_source"`
	Provider    string       `json:"provider"`
	Region      string       `json:"region,omitempty"`
	StartedAt   time.Time    `json:"started_at"`
	FinishedAt  time.Time    `json:"finished_at"`
	Duration    string       `json:"duration"`
	Summary     DriftSummary `json:"summary"`
	Items       []DriftItem  `json:"items"`
}

// ScanOptions configures a drift scan run.
type ScanOptions struct {
	Workspace      string
	StateFile      string
	Provider       string
	Region         string
	DetectUnmanaged bool
	Concurrency    int
	IgnoreRules    []string
}
