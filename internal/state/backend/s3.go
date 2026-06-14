package backend

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/models"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/state"
)

func loadS3(ctx context.Context, src models.StateSource) (*state.TerraformState, string, error) {
	if src.Bucket == "" || src.Key == "" {
		return nil, "", fmt.Errorf("s3 state requires bucket and key")
	}
	region := src.Region
	if region == "" {
		region = "us-east-1"
	}
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, "", fmt.Errorf("load AWS config: %w", err)
	}
	client := s3.NewFromConfig(cfg)
	out, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(src.Bucket),
		Key:    aws.String(src.Key),
	})
	if err != nil {
		return nil, "", fmt.Errorf("get s3 object s3://%s/%s: %w", src.Bucket, src.Key, err)
	}
	defer out.Body.Close()
	st, err := state.Load(out.Body)
	if err != nil {
		return nil, "", err
	}
	source := fmt.Sprintf("s3://%s/%s", src.Bucket, src.Key)
	return st, source, nil
}
