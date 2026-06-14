package cloudtypes

import (
	"context"

	"github.com/terraform-drift-detector/terraform_drift_detector/internal/models"
)

// ResourceRef identifies a resource to fetch from a cloud provider.
type ResourceRef struct {
	Address string
	Type    string
	CloudID string
	Region  string
	Attrs   map[string]any
}

// ListOptions configures resource listing for unmanaged detection.
type ListOptions struct {
	Region         string
	SubscriptionID string
	ProjectID      string
}

// Adapter fetches live resource metadata from a cloud provider.
type Adapter interface {
	Name() string
	FetchResource(ctx context.Context, ref ResourceRef) (*models.Resource, error)
	FetchResources(ctx context.Context, refs []ResourceRef) (map[string]*models.Resource, error)
	ListResources(ctx context.Context, resourceTypes []string, opts ListOptions) ([]*models.Resource, error)
}
