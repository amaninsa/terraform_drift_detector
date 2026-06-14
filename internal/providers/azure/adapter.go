package azure

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources/v3"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage/v2"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/models"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/cloudtypes"
)

// Adapter fetches Azure resources via the Azure SDK.
type Adapter struct {
	subscriptionID string
	rgClient       *armresources.ResourceGroupsClient
	vnetClient     *armnetwork.VirtualNetworksClient
	subnetClient   *armnetwork.SubnetsClient
	storageClient  *armstorage.AccountsClient
}

// NewAdapter creates an Azure cloud adapter.
func NewAdapter(subscriptionID string) (*Adapter, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("azure credentials: %w", err)
	}
	rgClient, err := armresources.NewResourceGroupsClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, err
	}
	vnetClient, err := armnetwork.NewVirtualNetworksClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, err
	}
	subnetClient, err := armnetwork.NewSubnetsClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, err
	}
	storageClient, err := armstorage.NewAccountsClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, err
	}
	return &Adapter{
		subscriptionID: subscriptionID,
		rgClient:       rgClient,
		vnetClient:     vnetClient,
		subnetClient:   subnetClient,
		storageClient:  storageClient,
	}, nil
}

func (a *Adapter) Name() string { return "azure" }

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
	case "azurerm_resource_group":
		return a.fetchResourceGroup(ctx, ref)
	case "azurerm_virtual_network":
		return a.fetchVirtualNetwork(ctx, ref)
	case "azurerm_subnet":
		return a.fetchSubnet(ctx, ref)
	case "azurerm_storage_account":
		return a.fetchStorageAccount(ctx, ref)
	default:
		return nil, fmt.Errorf("unsupported resource type %s", ref.Type)
	}
}

func (a *Adapter) fetchResourceGroup(ctx context.Context, ref cloudtypes.ResourceRef) (*models.Resource, error) {
	name := ref.CloudID
	if n, ok := ref.Attrs["name"].(string); ok && n != "" {
		name = n
	}
	rg, err := a.rgClient.Get(ctx, name, nil)
	if err != nil {
		return nil, err
	}
	return &models.Resource{
		ID:      ref.Address,
		Address: ref.Address,
		Provider: "azure",
		Type:    ref.Type,
		CloudID: ref.CloudID,
		Attributes: map[string]any{
			"name":     derefStr(rg.Name),
			"location": derefStr(rg.Location),
		},
		Tags:   azureTags(rg.Tags),
		Source: "cloud",
	}, nil
}

func (a *Adapter) fetchVirtualNetwork(ctx context.Context, ref cloudtypes.ResourceRef) (*models.Resource, error) {
	rg, name, err := parseAzureID(ref)
	if err != nil {
		return nil, err
	}
	vnet, err := a.vnetClient.Get(ctx, rg, name, nil)
	if err != nil {
		return nil, err
	}
	var spaces []string
	for _, s := range vnet.Properties.AddressSpace.AddressPrefixes {
		spaces = append(spaces, derefStr(s))
	}
	return &models.Resource{
		ID:      ref.Address,
		Address: ref.Address,
		Provider: "azure",
		Type:    ref.Type,
		CloudID: ref.CloudID,
		Attributes: map[string]any{
			"name":          derefStr(vnet.Name),
			"location":      derefStr(vnet.Location),
			"address_space": spaces,
		},
		Tags:   azureTags(vnet.Tags),
		Source: "cloud",
	}, nil
}

func (a *Adapter) fetchSubnet(ctx context.Context, ref cloudtypes.ResourceRef) (*models.Resource, error) {
	rg := stringAttr(ref.Attrs, "resource_group_name")
	vnet := stringAttr(ref.Attrs, "virtual_network_name")
	name := stringAttr(ref.Attrs, "name")
	if rg == "" || vnet == "" || name == "" {
		return nil, fmt.Errorf("subnet requires resource_group_name, virtual_network_name, and name in state")
	}
	subnet, err := a.subnetClient.Get(ctx, rg, vnet, name, nil)
	if err != nil {
		return nil, err
	}
	var prefixes []string
	for _, p := range subnet.Properties.AddressPrefixes {
		prefixes = append(prefixes, derefStr(p))
	}
	return &models.Resource{
		ID:      ref.Address,
		Address: ref.Address,
		Provider: "azure",
		Type:    ref.Type,
		CloudID: ref.CloudID,
		Attributes: map[string]any{
			"name":                 derefStr(subnet.Name),
			"address_prefixes":     prefixes,
			"virtual_network_name": vnet,
		},
		Tags:   map[string]string{},
		Source: "cloud",
	}, nil
}

func (a *Adapter) fetchStorageAccount(ctx context.Context, ref cloudtypes.ResourceRef) (*models.Resource, error) {
	rg, name, err := parseAzureID(ref)
	if err != nil {
		rg = stringAttr(ref.Attrs, "resource_group_name")
		name = stringAttr(ref.Attrs, "name")
	}
	if rg == "" || name == "" {
		return nil, fmt.Errorf("storage account requires resource group and name")
	}
	acct, err := a.storageClient.GetProperties(ctx, rg, name, nil)
	if err != nil {
		return nil, err
	}
	return &models.Resource{
		ID:      ref.Address,
		Address: ref.Address,
		Provider: "azure",
		Type:    ref.Type,
		CloudID: ref.CloudID,
		Attributes: map[string]any{
			"name":                       derefStr(acct.Name),
			"location":                   derefStr(acct.Location),
			"account_tier":               string(derefTier(acct.SKU)),
			"account_replication_type":   replicationType(acct.SKU),
		},
		Tags:   azureTags(acct.Tags),
		Source: "cloud",
	}, nil
}

func (a *Adapter) ListResources(ctx context.Context, resourceTypes []string, opts cloudtypes.ListOptions) ([]*models.Resource, error) {
	typeSet := map[string]bool{}
	for _, t := range resourceTypes {
		typeSet[t] = true
	}
	var out []*models.Resource
	if typeSet["azurerm_resource_group"] {
		pager := a.rgClient.NewListPager(nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return nil, err
			}
			for _, rg := range page.Value {
				if rg.ID == nil {
					continue
				}
				ref := cloudtypes.ResourceRef{Address: *rg.ID, Type: "azurerm_resource_group", CloudID: *rg.ID, Attrs: map[string]any{"name": derefStr(rg.Name)}}
				res, err := a.fetchResourceGroup(ctx, ref)
				if err == nil {
					out = append(out, res)
				}
			}
		}
	}
	if typeSet["azurerm_storage_account"] {
		pager := a.storageClient.NewListPager(nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return nil, err
			}
			for _, acct := range page.Value {
				if acct.ID == nil {
					continue
				}
				ref := cloudtypes.ResourceRef{Address: *acct.ID, Type: "azurerm_storage_account", CloudID: *acct.ID, Attrs: map[string]any{
					"resource_group_name": resourceGroupFromID(*acct.ID),
					"name":                derefStr(acct.Name),
				}}
				res, err := a.fetchStorageAccount(ctx, ref)
				if err == nil {
					out = append(out, res)
				}
			}
		}
	}
	return out, nil
}

func parseAzureID(ref cloudtypes.ResourceRef) (rg, name string, err error) {
	if n, ok := ref.Attrs["name"].(string); ok {
		name = n
	}
	if rgName, ok := ref.Attrs["resource_group_name"].(string); ok {
		rg = rgName
	}
	if rg != "" && name != "" {
		return rg, name, nil
	}
	return "", "", fmt.Errorf("could not parse azure resource identity from state")
}

func resourceGroupFromID(id string) string {
	parts := strings.Split(id, "/")
	for i, p := range parts {
		if strings.EqualFold(p, "resourceGroups") && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func azureTags(tags map[string]*string) map[string]string {
	out := map[string]string{}
	for k, v := range tags {
		out[k] = derefStr(v)
	}
	return out
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func stringAttr(attrs map[string]any, key string) string {
	if v, ok := attrs[key].(string); ok {
		return v
	}
	return ""
}

func derefTier(sku *armstorage.SKU) armstorage.SKUTier {
	if sku == nil || sku.Tier == nil {
		return ""
	}
	return *sku.Tier
}

func replicationType(sku *armstorage.SKU) string {
	if sku == nil || sku.Name == nil {
		return ""
	}
	return string(*sku.Name)
}
