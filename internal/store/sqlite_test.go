package store_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/terraform-drift-detector/terraform_drift_detector/internal/models"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/store"
)

func TestSaveAndLoadReport(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	now := time.Now().UTC()
	report := &models.DriftReport{
		ScanID:      "scan-123",
		Workspace:   "prod",
		StateSource: "terraform.tfstate",
		Provider:    "aws",
		StartedAt:   now,
		FinishedAt:  now,
		Duration:    "1s",
		Summary: models.DriftSummary{
			TotalResources: 1,
			Drifted:        1,
			Missing:        1,
		},
		Items: []models.DriftItem{
			{
				Address:   "aws_instance.web",
				Type:      "aws_instance",
				DriftType: models.DriftTypeMissing,
			},
		},
	}

	if err := s.SaveReport(report); err != nil {
		t.Fatalf("save report: %v", err)
	}

	list, err := s.ListScans(10)
	if err != nil {
		t.Fatalf("list scans: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 scan, got %d", len(list))
	}

	loaded, err := s.GetReport("scan-123")
	if err != nil {
		t.Fatalf("get report: %v", err)
	}
	if loaded.Items[0].Address != "aws_instance.web" {
		t.Fatalf("unexpected item: %#v", loaded.Items[0])
	}
}
