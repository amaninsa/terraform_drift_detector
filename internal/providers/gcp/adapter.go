package gcp

import (
	"context"
	"fmt"
	"strings"
	"sync"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"cloud.google.com/go/storage"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/models"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/cloudtypes"
	"google.golang.org/api/iterator"
)

// Adapter fetches GCP resources via Google Cloud APIs.
type Adapter struct {
	projectID      string
	instances      *compute.InstancesClient
	networks       *compute.NetworksClient
	storage        *storage.Client
}

// NewAdapter creates a GCP cloud adapter.
func NewAdapter(ctx context.Context, projectID string) (*Adapter, error) {
	instClient, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("compute instances client: %w", err)
	}
	netClient, err := compute.NewNetworksRESTClient(ctx)
	if err != nil {
		instClient.Close()
		return nil, fmt.Errorf("compute networks client: %w", err)
	}
	stClient, err := storage.NewClient(ctx)
	if err != nil {
		instClient.Close()
		netClient.Close()
		return nil, fmt.Errorf("storage client: %w", err)
	}
	return &Adapter{
		projectID: projectID,
		instances: instClient,
		networks:  netClient,
		storage:   stClient,
	}, nil
}

func (a *Adapter) Name() string { return "gcp" }

func (a *Adapter) Close() error {
	var errs []error
	if a.instances != nil {
		errs = append(errs, a.instances.Close())
	}
	if a.networks != nil {
		errs = append(errs, a.networks.Close())
	}
	if a.storage != nil {
		errs = append(errs, a.storage.Close())
	}
	return errorsJoin(errs)
}

func (a *Adapter) FetchResource(ctx context.Context, ref cloudtypes.ResourceRef) (*models.Resource, error) {
	results, err := a.FetchResources(ctx, []cloudtypes.ResourceRef{ref})
	if err != nil {
		return nil, err
	}
	res, ok := results[ref.Address]
	if !ok {
		return nil, fmt.Errorf("resource %s not found", ref.Address)
	}
	return res, nil
}

func (a *Adapter) FetchResources(ctx context.Context, refs []cloudtypes.ResourceRef) (map[string]*models.Resource, error) {
	out := make(map[string]*models.Resource, len(refs))
	var mu sync.Mutex
	var wg sync.WaitGroup
	errCh := make(chan error, len(refs))

	for _, ref := range refs {
		wg.Add(1)
		go func(ref cloudtypes.ResourceRef) {
			defer wg.Done()
			res, err := a.fetchOne(ctx, ref)
			if err != nil {
				errCh <- fmt.Errorf("%s: %w", ref.Address, err)
				return
			}
			mu.Lock()
			out[ref.Address] = res
			mu.Unlock()
		}(ref)
	}
	wg.Wait()
	close(errCh)

	var errs []string
	for err := range errCh {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return out, fmt.Errorf("fetch errors: %s", strings.Join(errs, "; "))
	}
	return out, nil
}

func (a *Adapter) fetchOne(ctx context.Context, ref cloudtypes.ResourceRef) (*models.Resource, error) {
	switch ref.Type {
	case "google_compute_instance":
		return a.fetchInstance(ctx, ref)
	case "google_storage_bucket":
		return a.fetchBucket(ctx, ref)
	case "google_compute_network":
		return a.fetchNetwork(ctx, ref)
	default:
		return nil, fmt.Errorf("unsupported resource type %s", ref.Type)
	}
}

func (a *Adapter) fetchInstance(ctx context.Context, ref cloudtypes.ResourceRef) (*models.Resource, error) {
	zone := stringAttr(ref.Attrs, "zone")
	name := stringAttr(ref.Attrs, "name")
	if zone == "" || name == "" {
		if ref.CloudID != "" {
			parts := strings.Split(ref.CloudID, "/")
			if len(parts) >= 2 {
				zone = parts[len(parts)-3]
				name = parts[len(parts)-1]
			}
		}
	}
	if zone == "" || name == "" {
		return nil, fmt.Errorf("instance requires zone and name")
	}
	inst, err := a.instances.Get(ctx, &computepb.GetInstanceRequest{
		Project:  a.projectID,
		Zone:     zone,
		Instance: name,
	})
	if err != nil {
		return nil, err
	}
	return &models.Resource{
		ID:      ref.Address,
		Address: ref.Address,
		Provider: "gcp",
		Type:    ref.Type,
		CloudID: ref.CloudID,
		Region:  zone,
		Attributes: map[string]any{
			"name":         inst.GetName(),
			"zone":         zone,
			"machine_type": machineTypeName(inst.GetMachineType()),
		},
		Tags:   gcpLabels(inst.GetLabels()),
		Source: "cloud",
	}, nil
}

func (a *Adapter) fetchBucket(ctx context.Context, ref cloudtypes.ResourceRef) (*models.Resource, error) {
	name := ref.CloudID
	if n := stringAttr(ref.Attrs, "name"); n != "" {
		name = n
	}
	attrs, err := a.storage.Bucket(name).Attrs(ctx)
	if err != nil {
		return nil, err
	}
	return &models.Resource{
		ID:      ref.Address,
		Address: ref.Address,
		Provider: "gcp",
		Type:    ref.Type,
		CloudID: name,
		Attributes: map[string]any{
			"name":     attrs.Name,
			"location": attrs.Location,
		},
		Tags:   attrs.Labels,
		Source: "cloud",
	}, nil
}

func (a *Adapter) fetchNetwork(ctx context.Context, ref cloudtypes.ResourceRef) (*models.Resource, error) {
	name := stringAttr(ref.Attrs, "name")
	if name == "" {
		name = ref.CloudID
	}
	net, err := a.networks.Get(ctx, &computepb.GetNetworkRequest{
		Project: a.projectID,
		Network: name,
	})
	if err != nil {
		return nil, err
	}
	return &models.Resource{
		ID:      ref.Address,
		Address: ref.Address,
		Provider: "gcp",
		Type:    ref.Type,
		CloudID: ref.CloudID,
		Attributes: map[string]any{
			"name":                    net.GetName(),
			"auto_create_subnetworks": net.GetAutoCreateSubnetworks(),
		},
		Tags:   map[string]string{},
		Source: "cloud",
	}, nil
}

func (a *Adapter) ListResources(ctx context.Context, resourceTypes []string, opts cloudtypes.ListOptions) ([]*models.Resource, error) {
	typeSet := map[string]bool{}
	for _, t := range resourceTypes {
		typeSet[t] = true
	}
	var out []*models.Resource
	if typeSet["google_storage_bucket"] {
		it := a.storage.Buckets(ctx, a.projectID)
		for {
			battrs, err := it.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return nil, err
			}
			ref := cloudtypes.ResourceRef{Address: battrs.Name, Type: "google_storage_bucket", CloudID: battrs.Name}
			res, err := a.fetchBucket(ctx, ref)
			if err == nil {
				out = append(out, res)
			}
		}
	}
	if typeSet["google_compute_network"] {
		it := a.networks.List(ctx, &computepb.ListNetworksRequest{Project: a.projectID})
		for {
			net, err := it.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return nil, err
			}
			ref := cloudtypes.ResourceRef{Address: net.GetName(), Type: "google_compute_network", CloudID: net.GetName(), Attrs: map[string]any{"name": net.GetName()}}
			res, err := a.fetchNetwork(ctx, ref)
			if err == nil {
				out = append(out, res)
			}
		}
	}
	return out, nil
}

func machineTypeName(selfLink string) string {
	if selfLink == "" {
		return ""
	}
	parts := strings.Split(selfLink, "/")
	return parts[len(parts)-1]
}

func gcpLabels(labels map[string]string) map[string]string {
	if labels == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(labels))
	for k, v := range labels {
		out[k] = v
	}
	return out
}

func stringAttr(attrs map[string]any, key string) string {
	if v, ok := attrs[key].(string); ok {
		return v
	}
	return ""
}

func errorsJoin(errs []error) error {
	var msgs []string
	for _, e := range errs {
		if e != nil {
			msgs = append(msgs, e.Error())
		}
	}
	if len(msgs) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(msgs, "; "))
}
