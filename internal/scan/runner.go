package scan

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/diff"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/mapper"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/models"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/normalize"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/cloudtypes"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/state"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/state/backend"
)

type scanTarget struct {
	managed  state.ManagedResource
	mapping  mapper.Mapping
	ref      cloudtypes.ResourceRef
	expected *models.Resource
}

// Runner executes drift scans.
type Runner struct {
	Registry *mapper.Registry
	Adapter  cloudtypes.Adapter
	Loader   *backend.Loader
}

// NewRunner creates a scan runner.
func NewRunner(registry *mapper.Registry, adapter cloudtypes.Adapter) *Runner {
	if registry == nil {
		registry = mapper.DefaultRegistry()
	}
	return &Runner{
		Registry: registry,
		Adapter:  adapter,
		Loader:   backend.NewLoader(),
	}
}

// Run executes a drift scan and returns a report.
func (r *Runner) Run(ctx context.Context, opts models.ScanOptions) (*models.DriftReport, error) {
	started := time.Now().UTC()
	if opts.Provider == "" {
		opts.Provider = "aws"
	}
	if opts.Workspace == "" {
		opts.Workspace = "default"
	}

	st, stateSource, err := r.Loader.Load(ctx, opts.State, opts.StateFile)
	if err != nil {
		return nil, err
	}

	managed := st.ExtractManaged()
	var targets []scanTarget
	var skipped []models.DriftItem
	managedCloudIDs := map[string]bool{}

	for _, res := range managed {
		if err := mapper.ValidateProvider(res, opts.Provider); err != nil {
			skipped = append(skipped, skipItem(res, err.Error()))
			continue
		}
		mapping, ok := r.Registry.Get(res.Type)
		if !ok {
			skipped = append(skipped, skipItem(res, fmt.Sprintf("unsupported resource type %s (no cloud mapping)", res.Type)))
			continue
		}
		ref := mapper.ToResourceRef(res, mapping, opts.Region)
		if ref.CloudID == "" {
			skipped = append(skipped, skipItem(res, "could not determine cloud resource ID from state attributes"))
			continue
		}
		managedCloudIDs[resourceKey(res.Type, ref.CloudID)] = true
		targets = append(targets, scanTarget{
			managed:  res,
			mapping:  mapping,
			ref:      ref,
			expected: stateToModel(res, ref.CloudID, opts.Region),
		})
	}

	refs := make([]cloudtypes.ResourceRef, len(targets))
	for i, t := range targets {
		refs[i] = t.ref
	}
	actualByAddress, fetchErr := r.Adapter.FetchResources(ctx, refs)

	var items []models.DriftItem
	items = append(items, skipped...)

	for _, target := range targets {
		normCfg := normalize.Config{
			IgnoreRules: opts.IgnoreRules,
			CompareKeys: target.mapping.CompareKeys,
		}
		normExpected := normalize.NormalizeResource(target.expected, normCfg)

		actual, ok := actualByAddress[target.expected.Address]
		if !ok {
			if fetchErr != nil {
				items = append(items, diff.FetchErrorItem(normExpected, fetchErr))
				continue
			}
			items = append(items, diff.CompareResources(normExpected, nil, opts.IgnoreRules))
			continue
		}
		normActual := normalize.NormalizeResource(actual, normCfg)
		items = append(items, diff.CompareResources(normExpected, normActual, opts.IgnoreRules))
	}

	if opts.DetectUnmanaged {
		unmanaged, err := r.detectUnmanaged(ctx, opts, managedCloudIDs)
		if err != nil {
			items = append(items, models.DriftItem{
				DriftType: models.DriftTypeFetchErr,
				Message:   fmt.Sprintf("unmanaged resource detection failed: %v", err),
			})
		} else {
			items = append(items, unmanaged...)
		}
	}

	finished := time.Now().UTC()
	return &models.DriftReport{
		ScanID:      uuid.New().String(),
		Workspace:   opts.Workspace,
		StateSource: stateSource,
		Provider:    opts.Provider,
		Region:      opts.Region,
		StartedAt:   started,
		FinishedAt:  finished,
		Duration:    finished.Sub(started).String(),
		Summary:     diff.BuildSummary(items),
		Items:       items,
	}, nil
}

func (r *Runner) detectUnmanaged(ctx context.Context, opts models.ScanOptions, managed map[string]bool) ([]models.DriftItem, error) {
	types := r.Registry.SupportedTypes(normalizeProviderName(opts.Provider))
	listed, err := r.Adapter.ListResources(ctx, types, cloudtypes.ListOptions{
		Region:         opts.Region,
		SubscriptionID: opts.SubscriptionID,
		ProjectID:      opts.ProjectID,
	})
	if err != nil {
		return nil, err
	}
	var items []models.DriftItem
	for _, res := range listed {
		key := resourceKey(res.Type, res.CloudID)
		if managed[key] {
			continue
		}
		items = append(items, diff.UnmanagedItem(res))
	}
	return items, nil
}

func normalizeProviderName(p string) string {
	if p == "gcp" {
		return "google"
	}
	return p
}

func resourceKey(typ, cloudID string) string {
	return typ + ":" + cloudID
}

func skipItem(res state.ManagedResource, msg string) models.DriftItem {
	return models.DriftItem{
		ResourceID: res.Address,
		Address:    res.Address,
		Type:       res.Type,
		Provider:   res.Provider,
		DriftType:  models.DriftTypeFetchErr,
		Message:    msg,
	}
}

func stateToModel(res state.ManagedResource, cloudID, region string) *models.Resource {
	return &models.Resource{
		ID:         res.Address,
		Address:    res.Address,
		Provider:   res.Provider,
		Type:       res.Type,
		CloudID:    cloudID,
		Region:     region,
		Attributes: res.Attributes,
		Tags:       state.ExtractTags(res.Attributes),
		Source:     "state",
	}
}
