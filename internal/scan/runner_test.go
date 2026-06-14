package scan_test

import (
	"context"
	"testing"

	"github.com/terraform-drift-detector/terraform_drift_detector/internal/models"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/providers"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/scan"
)

func TestRunnerDetectsDrift(t *testing.T) {
	mock := &providers.MockAdapter{
		Resources: map[string]*models.Resource{
			"aws_instance.web": {
				ID:         "aws_instance.web",
				Address:    "aws_instance.web",
				Provider:   "aws",
				Type:       "aws_instance",
				CloudID:    "i-0abc123def456",
				Attributes: map[string]any{
					"instance_type":          "t3.small",
					"ami":                    "ami-0c55b159cbfafe1f0",
					"subnet_id":              "subnet-0abc123",
					"vpc_security_group_ids": []any{"sg-0abc123"},
					"availability_zone":      "us-east-1a",
				},
				Tags: map[string]string{
					"Name":        "web-server",
					"Environment": "prod",
				},
				Source: "cloud",
			},
			"aws_s3_bucket.logs": {
				ID:         "aws_s3_bucket.logs",
				Address:    "aws_s3_bucket.logs",
				Provider:   "aws",
				Type:       "aws_s3_bucket",
				CloudID:    "my-app-logs-bucket",
				Attributes: map[string]any{
					"bucket": "my-app-logs-bucket",
					"region": "us-east-1",
				},
				Tags:   map[string]string{"Environment": "prod"},
				Source: "cloud",
			},
			"aws_security_group.web": {
				ID:         "aws_security_group.web",
				Address:    "aws_security_group.web",
				Provider:   "aws",
				Type:       "aws_security_group",
				CloudID:    "sg-0abc123",
				Attributes: map[string]any{
					"name":        "web-sg",
					"description": "Web server security group",
					"vpc_id":      "vpc-0abc123",
				},
				Tags:   map[string]string{"Name": "web-sg"},
				Source: "cloud",
			},
		},
	}

	runner := scan.NewRunner(nil, mock)
	report, err := runner.Run(context.Background(), models.ScanOptions{
		StateFile: "../../testdata/aws/terraform.tfstate",
		Provider:  "aws",
		Region:    "us-east-1",
	})
	if err != nil {
		t.Fatalf("run scan: %v", err)
	}
	if report.Summary.TotalResources != 3 {
		t.Fatalf("expected 3 resources, got %d", report.Summary.TotalResources)
	}
	if report.Summary.Modified != 1 {
		t.Fatalf("expected 1 modified resource (instance type drift), got %d", report.Summary.Modified)
	}
	if report.Summary.InSync != 2 {
		t.Fatalf("expected 2 in sync, got %d", report.Summary.InSync)
	}
}

func TestRunnerDetectsMissingResource(t *testing.T) {
	mock := &providers.MockAdapter{Resources: map[string]*models.Resource{}}
	runner := scan.NewRunner(nil, mock)
	report, err := runner.Run(context.Background(), models.ScanOptions{
		StateFile: "../../testdata/aws/terraform.tfstate",
		Provider:  "aws",
		Region:    "us-east-1",
	})
	if err != nil {
		t.Fatalf("run scan: %v", err)
	}
	if report.Summary.Missing != 3 {
		t.Fatalf("expected 3 missing, got %d", report.Summary.Missing)
	}
}
