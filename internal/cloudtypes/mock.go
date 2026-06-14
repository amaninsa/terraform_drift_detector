package cloudtypes

import (
	"context"
	"fmt"

	"github.com/terraform-drift-detector/terraform_drift_detector/internal/models"
)

// MockAdapter is an in-memory cloud adapter for tests.
type MockAdapter struct {
	Resources map[string]*models.Resource
}

func (m *MockAdapter) Name() string { return "mock" }

func (m *MockAdapter) FetchResource(ctx context.Context, ref ResourceRef) (*models.Resource, error) {
	results, err := m.FetchResources(ctx, []ResourceRef{ref})
	if err != nil {
		return nil, err
	}
	res, ok := results[ref.Address]
	if !ok {
		return nil, fmt.Errorf("resource %s not found", ref.Address)
	}
	return res, nil
}

func (m *MockAdapter) FetchResources(ctx context.Context, refs []ResourceRef) (map[string]*models.Resource, error) {
	out := make(map[string]*models.Resource, len(refs))
	for _, ref := range refs {
		if res, ok := m.Resources[ref.Address]; ok {
			cp := *res
			out[ref.Address] = &cp
		}
	}
	return out, nil
}

func (m *MockAdapter) ListResources(ctx context.Context, resourceTypes []string, opts ListOptions) ([]*models.Resource, error) {
	typeSet := map[string]bool{}
	for _, t := range resourceTypes {
		typeSet[t] = true
	}
	var out []*models.Resource
	seen := map[string]bool{}
	for _, res := range m.Resources {
		if !typeSet[res.Type] {
			continue
		}
		key := res.Type + ":" + res.CloudID
		if seen[key] {
			continue
		}
		seen[key] = true
		cp := *res
		out = append(out, &cp)
	}
	return out, nil
}
