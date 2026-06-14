package config_test

import (
	"testing"

	"github.com/terraform-drift-detector/terraform_drift_detector/internal/config"
)

func TestLoadConfig(t *testing.T) {
	cfg, err := config.Load("../../configs/examples/drift.yaml")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(cfg.Workspaces) != 3 {
		t.Fatalf("expected 3 workspaces, got %d", len(cfg.Workspaces))
	}
	ws, err := cfg.GetWorkspace("prod")
	if err != nil {
		t.Fatalf("get workspace: %v", err)
	}
	if ws.State.Bucket != "my-tf-state" {
		t.Fatalf("unexpected bucket: %s", ws.State.Bucket)
	}
	if len(cfg.ActiveWebhooks()) != 1 {
		t.Fatalf("expected 1 webhook")
	}
}
