package notify_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/terraform-drift-detector/terraform_drift_detector/internal/models"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/notify"
)

func TestNotifyDrift(t *testing.T) {
	var received notify.Payload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	report := &models.DriftReport{
		ScanID:    "scan-1",
		Workspace: "prod",
		Provider:  "aws",
		Summary:   models.DriftSummary{Drifted: 1, TotalResources: 2},
		Items: []models.DriftItem{
			{DriftType: models.DriftTypeModified, Address: "aws_instance.web"},
			{DriftType: models.DriftTypeInSync, Address: "aws_s3_bucket.logs"},
		},
		StartedAt: time.Now(),
	}
	client := notify.NewClient()
	if err := client.NotifyDrift(context.Background(), []string{srv.URL}, report); err != nil {
		t.Fatalf("notify: %v", err)
	}
	if received.ScanID != "scan-1" || len(received.Drifted) != 1 {
		t.Fatalf("unexpected payload: %#v", received)
	}
}
