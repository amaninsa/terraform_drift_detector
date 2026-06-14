package diff_test

import (
	"testing"

	"github.com/terraform-drift-detector/terraform_drift_detector/internal/diff"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/models"
)

func TestCompareResourcesMissing(t *testing.T) {
	expected := &models.Resource{
		ID:      "aws_instance.web",
		Address: "aws_instance.web",
		Type:    "aws_instance",
		CloudID: "i-123",
	}
	item := diff.CompareResources(expected, nil, nil)
	if item.DriftType != models.DriftTypeMissing {
		t.Fatalf("expected missing, got %s", item.DriftType)
	}
}

func TestCompareResourcesTagOnly(t *testing.T) {
	expected := &models.Resource{
		Address:    "aws_instance.web",
		Attributes: map[string]any{"instance_type": "t3.micro"},
		Tags:       map[string]string{"Environment": "prod"},
	}
	actual := &models.Resource{
		Address:    "aws_instance.web",
		Attributes: map[string]any{"instance_type": "t3.micro"},
		Tags:       map[string]string{"Environment": "staging"},
	}
	item := diff.CompareResources(expected, actual, nil)
	if item.DriftType != models.DriftTypeTagOnly {
		t.Fatalf("expected tag_only, got %s", item.DriftType)
	}
}

func TestCompareResourcesModified(t *testing.T) {
	expected := &models.Resource{
		Address:    "aws_instance.web",
		Attributes: map[string]any{"instance_type": "t3.micro"},
		Tags:       map[string]string{"Environment": "prod"},
	}
	actual := &models.Resource{
		Address:    "aws_instance.web",
		Attributes: map[string]any{"instance_type": "t3.small"},
		Tags:       map[string]string{"Environment": "prod"},
	}
	item := diff.CompareResources(expected, actual, nil)
	if item.DriftType != models.DriftTypeModified {
		t.Fatalf("expected modified, got %s", item.DriftType)
	}
}

func TestBuildSummary(t *testing.T) {
	items := []models.DriftItem{
		{DriftType: models.DriftTypeInSync},
		{DriftType: models.DriftTypeMissing},
		{DriftType: models.DriftTypeTagOnly},
	}
	summary := diff.BuildSummary(items)
	if summary.TotalResources != 3 || summary.Drifted != 2 || summary.InSync != 1 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
}
