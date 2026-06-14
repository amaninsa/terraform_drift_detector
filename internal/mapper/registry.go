package mapper

import (
	"fmt"
	"strings"

	"github.com/terraform-drift-detector/terraform_drift_detector/internal/providers"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/state"
)

// Mapping describes how a Terraform resource type maps to cloud fetch logic.
type Mapping struct {
	Provider    string   `yaml:"provider"`
	IDAttribute string   `yaml:"id_attribute"`
	TagSource   string   `yaml:"tag_source"`
	CompareKeys []string `yaml:"compare_keys"`
}

// Registry maps Terraform resource types to fetch strategies.
type Registry struct {
	mappings map[string]Mapping
}

// DefaultRegistry returns built-in mappings for supported resource types.
func DefaultRegistry() *Registry {
	return &Registry{
		mappings: map[string]Mapping{
			"aws_instance": {
				Provider:    "aws",
				IDAttribute: "id",
				TagSource:   "tags",
				CompareKeys: []string{"instance_type", "ami", "subnet_id", "vpc_security_group_ids", "availability_zone"},
			},
			"aws_s3_bucket": {
				Provider:    "aws",
				IDAttribute: "bucket",
				TagSource:   "tags",
				CompareKeys: []string{"bucket", "region"},
			},
			"aws_security_group": {
				Provider:    "aws",
				IDAttribute: "id",
				TagSource:   "tags",
				CompareKeys: []string{"name", "description", "vpc_id"},
			},
			"aws_vpc": {
				Provider:    "aws",
				IDAttribute: "id",
				TagSource:   "tags",
				CompareKeys: []string{"cidr_block", "enable_dns_hostnames", "enable_dns_support"},
			},
			"aws_subnet": {
				Provider:    "aws",
				IDAttribute: "id",
				TagSource:   "tags",
				CompareKeys: []string{"cidr_block", "vpc_id", "availability_zone", "map_public_ip_on_launch"},
			},
		},
	}
}

// Get returns the mapping for a resource type.
func (r *Registry) Get(resourceType string) (Mapping, bool) {
	m, ok := r.mappings[resourceType]
	return m, ok
}

// SupportedTypes returns all mapped resource types for a provider.
func (r *Registry) SupportedTypes(provider string) []string {
	var types []string
	for typ, m := range r.mappings {
		if m.Provider == provider {
			types = append(types, typ)
		}
	}
	return types
}

// IsSupported checks if a resource type has a mapping.
func (r *Registry) IsSupported(resourceType string) bool {
	_, ok := r.mappings[resourceType]
	return ok
}

// ToResourceRef converts a state resource to a cloud fetch reference.
func ToResourceRef(res state.ManagedResource, mapping Mapping, region string) providers.ResourceRef {
	cloudID := state.CloudID(res)
	if mapping.IDAttribute != "" {
		if v, ok := res.Attributes[mapping.IDAttribute]; ok {
			if s, ok := v.(string); ok && s != "" {
				cloudID = s
			}
		}
	}
	return providers.ResourceRef{
		Address: res.Address,
		Type:    res.Type,
		CloudID: cloudID,
		Region:  region,
		Attrs:   res.Attributes,
	}
}

// ProviderFromType infers provider name from Terraform resource type prefix.
func ProviderFromType(resourceType string) string {
	if idx := strings.Index(resourceType, "_"); idx > 0 {
		return resourceType[:idx]
	}
	return ""
}

// ValidateProvider ensures the resource belongs to the expected provider.
func ValidateProvider(res state.ManagedResource, expectedProvider string) error {
	prefix := ProviderFromType(res.Type)
	if expectedProvider != "" && prefix != expectedProvider {
		return fmt.Errorf("resource %s provider %s does not match scan provider %s", res.Address, prefix, expectedProvider)
	}
	return nil
}
