package state_test

import (
	"os"
	"testing"

	"github.com/terraform-drift-detector/terraform_drift_detector/internal/state"
)

func TestLoadFromFile(t *testing.T) {
	st, err := state.LoadFromFile("../../testdata/aws/terraform.tfstate")
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if st.Version != 4 {
		t.Fatalf("expected version 4, got %d", st.Version)
	}
}

func TestExtractManagedSkipsDataSources(t *testing.T) {
	st, err := state.LoadFromFile("../../testdata/aws/terraform.tfstate")
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	managed := st.ExtractManaged()
	if len(managed) != 3 {
		t.Fatalf("expected 3 managed resources, got %d", len(managed))
	}
	for _, res := range managed {
		if res.Type == "aws_availability_zones" {
			t.Fatalf("data source should be skipped")
		}
	}
}

func TestCloudIDAndTags(t *testing.T) {
	st, err := state.LoadFromFile("../../testdata/aws/terraform.tfstate")
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	managed := st.ExtractManaged()
	var instance state.ManagedResource
	for _, res := range managed {
		if res.Type == "aws_instance" {
			instance = res
			break
		}
	}
	if state.CloudID(instance) != "i-0abc123def456" {
		t.Fatalf("unexpected cloud id: %s", state.CloudID(instance))
	}
	tags := state.ExtractTags(instance.Attributes)
	if tags["Environment"] != "prod" {
		t.Fatalf("unexpected tags: %#v", tags)
	}
}

func TestUnsupportedVersion(t *testing.T) {
	f, err := os.CreateTemp("", "state-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	_, _ = f.WriteString(`{"version": 3, "resources": []}`)
	f.Close()

	_, err = state.LoadFromFile(f.Name())
	if err == nil {
		t.Fatal("expected error for version 3")
	}
}
