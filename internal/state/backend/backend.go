package backend

import (
	"context"
	"fmt"
	"io"

	"github.com/terraform-drift-detector/terraform_drift_detector/internal/models"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/state"
)

// Loader loads Terraform state from various backends.
type Loader struct{}

// NewLoader creates a state loader.
func NewLoader() *Loader { return &Loader{} }

// Load fetches and parses Terraform state based on the source configuration.
func (l *Loader) Load(ctx context.Context, src models.StateSource, localPath string) (*state.TerraformState, string, error) {
	switch src.Type {
	case "", "local":
		path := src.Path
		if path == "" {
			path = localPath
		}
		if path == "" {
			return nil, "", fmt.Errorf("state path is required for local backend")
		}
		st, err := state.LoadFromFile(path)
		return st, path, err
	case "s3":
		return loadS3(ctx, src)
	default:
		return nil, "", fmt.Errorf("unsupported state backend %q", src.Type)
	}
}

// Describe returns a human-readable state source description.
func Describe(src models.StateSource, localPath string) string {
	switch src.Type {
	case "", "local":
		if src.Path != "" {
			return src.Path
		}
		return localPath
	case "s3":
		return fmt.Sprintf("s3://%s/%s", src.Bucket, src.Key)
	default:
		return src.Type
	}
}

func loadFromReader(r io.Reader) (*state.TerraformState, error) {
	return state.Load(r)
}
