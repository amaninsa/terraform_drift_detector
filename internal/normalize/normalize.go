package normalize

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/terraform-drift-detector/terraform_drift_detector/internal/models"
)

// DefaultIgnoreRules are attributes commonly computed by providers.
var DefaultIgnoreRules = []string{
	"arn",
	"tags_all",
	"owner_id",
	"requester_id",
	"self",
	"default_network_acl_id",
	"main_route_table_id",
	"ipv6_association_id",
	"ipv6_cidr_block_association_id",
	"primary_network_interface_id",
	"private_dns",
	"public_dns",
	"monitoring",
	"credit_specification",
	"capacity_reservation_specification",
	"cpu_options",
	"enclave_options",
	"hibernation_options",
	"instance_state",
	"maintenance_options",
	"metadata_options",
	"private_dns_name_options",
	"root_block_device",
	"ebs_block_device",
	"ephemeral_block_device",
	"network_interface",
	"placement_group",
	"placement_partition_number",
	"outpost_arn",
	"password_data",
	"source_dest_check",
	"tenancy",
	"host_id",
	"host_resource_group_arn",
	"iam_instance_profile",
	"instance_lifecycle",
	"spot_instance_request_id",
	"user_data_base64",
	"user_data_replace_on_change",
	"force_destroy",
	"grant",
	"server_side_encryption_configuration",
	"versioning",
	"website",
	"acceleration_status",
	"bucket_domain_name",
	"bucket_regional_domain_name",
	"hosted_zone_id",
	"region",
	"id",
}

// Config controls normalization behavior.
type Config struct {
	IgnoreRules []string
	CompareKeys []string
}

// NormalizeResource prepares a resource for comparison.
func NormalizeResource(res *models.Resource, cfg Config) *models.Resource {
	if res == nil {
		return nil
	}
	ignore := buildIgnoreSet(cfg.IgnoreRules)
	attrs := filterAttributes(res.Attributes, ignore, cfg.CompareKeys)
	tags := normalizeTags(res.Tags)
	return &models.Resource{
		ID:         res.ID,
		Address:    res.Address,
		Provider:   res.Provider,
		Type:       res.Type,
		CloudID:    res.CloudID,
		Region:     res.Region,
		Attributes: attrs,
		Tags:       tags,
		Source:     res.Source,
	}
}

func buildIgnoreSet(extra []string) map[string]bool {
	set := map[string]bool{}
	for _, rule := range DefaultIgnoreRules {
		set[rule] = true
	}
	for _, rule := range extra {
		set[rule] = true
	}
	return set
}

func filterAttributes(attrs map[string]any, ignore map[string]bool, compareKeys []string) map[string]any {
	if attrs == nil {
		return map[string]any{}
	}
	out := map[string]any{}
	if len(compareKeys) > 0 {
		for _, key := range compareKeys {
			if ignore[key] {
				continue
			}
			if v, ok := attrs[key]; ok {
				out[key] = normalizeValue(v)
			}
		}
		return out
	}
	for k, v := range attrs {
		if ignore[k] || k == "tags" || k == "tags_all" {
			continue
		}
		out[k] = normalizeValue(v)
	}
	return out
}

func normalizeValue(v any) any {
	switch val := v.(type) {
	case []any:
		strs := make([]string, 0, len(val))
		for _, item := range val {
			strs = append(strs, scalarString(item))
		}
		sort.Strings(strs)
		return strs
	case []string:
		cp := append([]string(nil), val...)
		sort.Strings(cp)
		return cp
	case map[string]any:
		out := map[string]any{}
		for k, vv := range val {
			out[k] = normalizeValue(vv)
		}
		return out
	case bool:
		return val
	case float64:
		if val == float64(int64(val)) {
			return int64(val)
		}
		return val
	default:
		return scalarString(val)
	}
}

func scalarString(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case float64:
		if s == float64(int64(s)) {
			return strconv.FormatInt(int64(s), 10)
		}
		return strconv.FormatFloat(s, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(s)
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

func normalizeTags(tags map[string]string) map[string]string {
	if len(tags) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(tags))
	for k, v := range tags {
		out[k] = v
	}
	return out
}
