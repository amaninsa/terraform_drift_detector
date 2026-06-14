package state

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// TerraformState represents a Terraform state file (version 4).
type TerraformState struct {
	Version          int            `json:"version"`
	TerraformVersion string         `json:"terraform_version"`
	Serial           int            `json:"serial"`
	Lineage          string         `json:"lineage"`
	Resources        []StateResource `json:"resources"`
}

// StateResource is a resource entry in Terraform state.
type StateResource struct {
	Mode      string         `json:"mode"`
	Type      string         `json:"type"`
	Name      string         `json:"name"`
	Provider  string         `json:"provider"`
	Module    string         `json:"module,omitempty"`
	Instances []StateInstance `json:"instances"`
}

// StateInstance is a single instance of a state resource.
type StateInstance struct {
	SchemaVersion int            `json:"schema_version"`
	Attributes    map[string]any `json:"attributes"`
}

// ManagedResource is a flattened managed resource from state.
type ManagedResource struct {
	Address    string
	Type       string
	Provider   string
	Module     string
	Attributes map[string]any
}

// LoadFromFile reads and parses a Terraform state file from disk.
func LoadFromFile(path string) (*TerraformState, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open state file: %w", err)
	}
	defer f.Close()
	return Load(f)
}

// Load parses Terraform state JSON from a reader.
func Load(r io.Reader) (*TerraformState, error) {
	var st TerraformState
	if err := json.NewDecoder(r).Decode(&st); err != nil {
		return nil, fmt.Errorf("decode state: %w", err)
	}
	if st.Version != 4 {
		return nil, fmt.Errorf("unsupported state version %d (only version 4 supported)", st.Version)
	}
	return &st, nil
}

// ExtractManaged returns managed resources, skipping data sources.
func (st *TerraformState) ExtractManaged() []ManagedResource {
	var out []ManagedResource
	for _, res := range st.Resources {
		if res.Mode != "managed" {
			continue
		}
		for _, inst := range res.Instances {
			if inst.Attributes == nil {
				continue
			}
			out = append(out, ManagedResource{
				Address:    buildAddress(res.Module, res.Type, res.Name),
				Type:       res.Type,
				Provider:   normalizeProvider(res.Provider),
				Module:     res.Module,
				Attributes: inst.Attributes,
			})
		}
	}
	return out
}

func buildAddress(module, typ, name string) string {
	prefix := ""
	if module != "" {
		prefix = strings.TrimPrefix(module, "module.") + "."
	}
	return prefix + typ + "." + name
}

func normalizeProvider(provider string) string {
	// provider["registry.terraform.io/hashicorp/aws"] -> aws
	provider = strings.TrimPrefix(provider, `provider["registry.terraform.io/hashicorp/`)
	provider = strings.TrimPrefix(provider, `provider["`)
	provider = strings.TrimSuffix(provider, `"]`)
	if idx := strings.LastIndex(provider, "/"); idx >= 0 {
		provider = provider[idx+1:]
	}
	return provider
}

// CloudID extracts the primary cloud identifier from resource attributes.
func CloudID(res ManagedResource) string {
	attrs := res.Attributes
	for _, key := range []string{"id", "bucket", "arn", "name"} {
		if v, ok := attrs[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

// ExtractTags pulls tags from common Terraform attribute shapes.
func ExtractTags(attrs map[string]any) map[string]string {
	tags := map[string]string{}
	if raw, ok := attrs["tags"]; ok {
		mergeTags(tags, raw)
	}
	if raw, ok := attrs["tags_all"]; ok {
		mergeTags(tags, raw)
	}
	return tags
}

func mergeTags(dst map[string]string, raw any) {
	m, ok := raw.(map[string]any)
	if !ok {
		return
	}
	for k, v := range m {
		if s, ok := v.(string); ok {
			dst[k] = s
		}
	}
}
