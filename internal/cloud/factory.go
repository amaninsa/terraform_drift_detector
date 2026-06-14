package cloud

import (
	"context"
	"fmt"
	"os"

	awsprovider "github.com/terraform-drift-detector/terraform_drift_detector/internal/providers/aws"
	azureprovider "github.com/terraform-drift-detector/terraform_drift_detector/internal/providers/azure"
	gcpprovider "github.com/terraform-drift-detector/terraform_drift_detector/internal/providers/gcp"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/cloudtypes"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/models"
)

// Config holds cloud provider connection settings.
type Config struct {
	Provider       string
	Region         string
	SubscriptionID string
	ProjectID      string
}

// NewAdapter creates a cloud adapter for the given provider.
func NewAdapter(ctx context.Context, cfg Config) (cloudtypes.Adapter, error) {
	switch cfg.Provider {
	case "aws", "":
		region := cfg.Region
		if region == "" {
			region = os.Getenv("AWS_REGION")
		}
		if region == "" {
			region = "us-east-1"
		}
		return awsprovider.NewAdapter(ctx, region)
	case "azure":
		subID := cfg.SubscriptionID
		if subID == "" {
			subID = os.Getenv("AZURE_SUBSCRIPTION_ID")
		}
		if subID == "" {
			return nil, fmt.Errorf("azure subscription ID required (set subscription_id or AZURE_SUBSCRIPTION_ID)")
		}
		return azureprovider.NewAdapter(subID)
	case "gcp":
		projectID := cfg.ProjectID
		if projectID == "" {
			projectID = os.Getenv("GOOGLE_CLOUD_PROJECT")
		}
		if projectID == "" {
			projectID = os.Getenv("GCP_PROJECT")
		}
		if projectID == "" {
			return nil, fmt.Errorf("gcp project ID required (set project_id or GOOGLE_CLOUD_PROJECT)")
		}
		return gcpprovider.NewAdapter(ctx, projectID)
	default:
		return nil, fmt.Errorf("unsupported provider %q", cfg.Provider)
	}
}

// NewAdapterFromScanOptions creates an adapter from scan options.
func NewAdapterFromScanOptions(ctx context.Context, opts models.ScanOptions) (cloudtypes.Adapter, error) {
	return NewAdapter(ctx, Config{
		Provider:       opts.Provider,
		Region:         opts.Region,
		SubscriptionID: opts.SubscriptionID,
		ProjectID:      opts.ProjectID,
	})
}
