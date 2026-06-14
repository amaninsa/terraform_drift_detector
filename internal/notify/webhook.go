package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/terraform-drift-detector/terraform_drift_detector/internal/models"
)

// Client sends drift notifications to webhooks.
type Client struct {
	HTTPClient *http.Client
}

// NewClient creates a webhook notification client.
func NewClient() *Client {
	return &Client{HTTPClient: &http.Client{Timeout: 15 * time.Second}}
}

// Payload is the webhook notification body.
type Payload struct {
	Event     string              `json:"event"`
	ScanID    string              `json:"scan_id"`
	Workspace string              `json:"workspace"`
	Provider  string              `json:"provider"`
	Summary   models.DriftSummary `json:"summary"`
	Drifted   []models.DriftItem  `json:"drifted_items"`
	Timestamp time.Time           `json:"timestamp"`
}

// NotifyDrift sends drift alerts to all configured webhook URLs.
func (c *Client) NotifyDrift(ctx context.Context, urls []string, report *models.DriftReport) error {
	if len(urls) == 0 || report.Summary.Drifted == 0 {
		return nil
	}
	var drifted []models.DriftItem
	for _, item := range report.Items {
		if item.DriftType != models.DriftTypeInSync {
			drifted = append(drifted, item)
		}
	}
	payload := Payload{
		Event:     "drift.detected",
		ScanID:    report.ScanID,
		Workspace: report.Workspace,
		Provider:  report.Provider,
		Summary:   report.Summary,
		Drifted:   drifted,
		Timestamp: time.Now().UTC(),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	var errs []string
	for _, url := range urls {
		if err := c.post(ctx, url, body); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", url, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("webhook errors: %s", join(errs))
	}
	return nil
}

func (c *Client) post(ctx context.Context, url string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "terraform-drift-detector/1.0")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func join(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "; "
		}
		out += p
	}
	return out
}
