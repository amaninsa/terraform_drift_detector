package aws

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/models"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/cloudtypes"
)

// Adapter fetches AWS resources via the AWS SDK.
type Adapter struct {
	region string
	ec2    *ec2.Client
	s3     *s3.Client
}

// NewAdapter creates an AWS cloud adapter for the given region.
func NewAdapter(ctx context.Context, region string) (*Adapter, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}
	return &Adapter{
		region: region,
		ec2:    ec2.NewFromConfig(cfg),
		s3:     s3.NewFromConfig(cfg),
	}, nil
}

// NewAdapterWithClients creates an adapter with injected clients (for testing).
func NewAdapterWithClients(region string, ec2Client *ec2.Client, s3Client *s3.Client) *Adapter {
	return &Adapter{
		region: region,
		ec2:    ec2Client,
		s3:     s3Client,
	}
}

func (a *Adapter) Name() string { return "aws" }

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
	case "aws_instance":
		return a.fetchInstance(ctx, ref)
	case "aws_security_group":
		return a.fetchSecurityGroup(ctx, ref)
	case "aws_vpc":
		return a.fetchVPC(ctx, ref)
	case "aws_subnet":
		return a.fetchSubnet(ctx, ref)
	case "aws_s3_bucket":
		return a.fetchS3Bucket(ctx, ref)
	default:
		return nil, fmt.Errorf("unsupported resource type %s", ref.Type)
	}
}

func (a *Adapter) fetchInstance(ctx context.Context, ref cloudtypes.ResourceRef) (*models.Resource, error) {
	out, err := a.ec2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{ref.CloudID},
	})
	if err != nil {
		return nil, err
	}
	for _, res := range out.Reservations {
		for _, inst := range res.Instances {
			return instanceToResource(ref, inst, a.region), nil
		}
	}
	return nil, fmt.Errorf("instance %s not found", ref.CloudID)
}

func instanceToResource(ref cloudtypes.ResourceRef, inst ec2types.Instance, region string) *models.Resource {
	sgIDs := make([]string, 0, len(inst.SecurityGroups))
	for _, sg := range inst.SecurityGroups {
		if sg.GroupId != nil {
			sgIDs = append(sgIDs, *sg.GroupId)
		}
	}
	attrs := map[string]any{
		"instance_type":          string(inst.InstanceType),
		"ami":                    aws.ToString(inst.ImageId),
		"subnet_id":              aws.ToString(inst.SubnetId),
		"vpc_security_group_ids": sgIDs,
		"availability_zone":      aws.ToString(inst.Placement.AvailabilityZone),
	}
	return &models.Resource{
		ID:         ref.Address,
		Address:    ref.Address,
		Provider:   "aws",
		Type:       ref.Type,
		CloudID:    ref.CloudID,
		Region:     region,
		Attributes: attrs,
		Tags:       ec2Tags(inst.Tags),
		Source:     "cloud",
	}
}

func (a *Adapter) fetchSecurityGroup(ctx context.Context, ref cloudtypes.ResourceRef) (*models.Resource, error) {
	out, err := a.ec2.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		GroupIds: []string{ref.CloudID},
	})
	if err != nil {
		return nil, err
	}
	if len(out.SecurityGroups) == 0 {
		return nil, fmt.Errorf("security group %s not found", ref.CloudID)
	}
	sg := out.SecurityGroups[0]
	attrs := map[string]any{
		"name":        aws.ToString(sg.GroupName),
		"description": aws.ToString(sg.Description),
		"vpc_id":      aws.ToString(sg.VpcId),
	}
	return &models.Resource{
		ID:         ref.Address,
		Address:    ref.Address,
		Provider:   "aws",
		Type:       ref.Type,
		CloudID:    ref.CloudID,
		Region:     a.region,
		Attributes: attrs,
		Tags:       ec2Tags(sg.Tags),
		Source:     "cloud",
	}, nil
}

func (a *Adapter) fetchVPC(ctx context.Context, ref cloudtypes.ResourceRef) (*models.Resource, error) {
	out, err := a.ec2.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
		VpcIds: []string{ref.CloudID},
	})
	if err != nil {
		return nil, err
	}
	if len(out.Vpcs) == 0 {
		return nil, fmt.Errorf("vpc %s not found", ref.CloudID)
	}
	vpc := out.Vpcs[0]
	attrs := map[string]any{
		"cidr_block":           aws.ToString(vpc.CidrBlock),
		"enable_dns_hostnames": nil,
		"enable_dns_support":   nil,
	}
	return &models.Resource{
		ID:         ref.Address,
		Address:    ref.Address,
		Provider:   "aws",
		Type:       ref.Type,
		CloudID:    ref.CloudID,
		Region:     a.region,
		Attributes: attrs,
		Tags:       ec2Tags(vpc.Tags),
		Source:     "cloud",
	}, nil
}

func (a *Adapter) fetchSubnet(ctx context.Context, ref cloudtypes.ResourceRef) (*models.Resource, error) {
	out, err := a.ec2.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
		SubnetIds: []string{ref.CloudID},
	})
	if err != nil {
		return nil, err
	}
	if len(out.Subnets) == 0 {
		return nil, fmt.Errorf("subnet %s not found", ref.CloudID)
	}
	subnet := out.Subnets[0]
	attrs := map[string]any{
		"cidr_block":              aws.ToString(subnet.CidrBlock),
		"vpc_id":                  aws.ToString(subnet.VpcId),
		"availability_zone":       aws.ToString(subnet.AvailabilityZone),
		"map_public_ip_on_launch": aws.ToBool(subnet.MapPublicIpOnLaunch),
	}
	return &models.Resource{
		ID:         ref.Address,
		Address:    ref.Address,
		Provider:   "aws",
		Type:       ref.Type,
		CloudID:    ref.CloudID,
		Region:     a.region,
		Attributes: attrs,
		Tags:       ec2Tags(subnet.Tags),
		Source:     "cloud",
	}, nil
}

func (a *Adapter) fetchS3Bucket(ctx context.Context, ref cloudtypes.ResourceRef) (*models.Resource, error) {
	bucket := ref.CloudID
	_, err := a.s3.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(bucket)})
	if err != nil {
		return nil, err
	}

	locOut, err := a.s3.GetBucketLocation(ctx, &s3.GetBucketLocationInput{Bucket: aws.String(bucket)})
	if err != nil {
		return nil, err
	}
	region := string(locOut.LocationConstraint)
	if region == "" {
		region = "us-east-1"
	}

	tags := map[string]string{}
	tagOut, err := a.s3.GetBucketTagging(ctx, &s3.GetBucketTaggingInput{Bucket: aws.String(bucket)})
	if err == nil {
		for _, t := range tagOut.TagSet {
			tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
		}
	}

	attrs := map[string]any{
		"bucket": bucket,
		"region": region,
	}
	return &models.Resource{
		ID:         ref.Address,
		Address:    ref.Address,
		Provider:   "aws",
		Type:       ref.Type,
		CloudID:    bucket,
		Region:     region,
		Attributes: attrs,
		Tags:       tags,
		Source:     "cloud",
	}, nil
}

func ec2Tags(tags []ec2types.Tag) map[string]string {
	out := map[string]string{}
	for _, t := range tags {
		out[aws.ToString(t.Key)] = aws.ToString(t.Value)
	}
	return out
}

func (a *Adapter) ListResources(ctx context.Context, resourceTypes []string, opts cloudtypes.ListOptions) ([]*models.Resource, error) {
	typeSet := map[string]bool{}
	for _, t := range resourceTypes {
		typeSet[t] = true
	}
	var out []*models.Resource
	for _, typ := range resourceTypes {
		if !typeSet[typ] {
			continue
		}
		switch typ {
		case "aws_instance":
			items, err := a.listInstances(ctx)
			if err != nil {
				return nil, err
			}
			out = append(out, items...)
		case "aws_security_group":
			items, err := a.listSecurityGroups(ctx)
			if err != nil {
				return nil, err
			}
			out = append(out, items...)
		case "aws_s3_bucket":
			items, err := a.listBuckets(ctx)
			if err != nil {
				return nil, err
			}
			out = append(out, items...)
		case "aws_vpc":
			items, err := a.listVPCs(ctx)
			if err != nil {
				return nil, err
			}
			out = append(out, items...)
		case "aws_subnet":
			items, err := a.listSubnets(ctx)
			if err != nil {
				return nil, err
			}
			out = append(out, items...)
		}
	}
	return out, nil
}

func (a *Adapter) listInstances(ctx context.Context) ([]*models.Resource, error) {
	out, err := a.ec2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{})
	if err != nil {
		return nil, err
	}
	var res []*models.Resource
	for _, r := range out.Reservations {
		for _, inst := range r.Instances {
			if inst.InstanceId == nil || string(inst.State.Name) == "terminated" {
				continue
			}
			ref := cloudtypes.ResourceRef{Address: *inst.InstanceId, Type: "aws_instance", CloudID: *inst.InstanceId}
			res = append(res, instanceToResource(ref, inst, a.region))
		}
	}
	return res, nil
}

func (a *Adapter) listSecurityGroups(ctx context.Context) ([]*models.Resource, error) {
	out, err := a.ec2.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{})
	if err != nil {
		return nil, err
	}
	var res []*models.Resource
	for _, sg := range out.SecurityGroups {
		if sg.GroupId == nil {
			continue
		}
		ref := cloudtypes.ResourceRef{Address: *sg.GroupId, Type: "aws_security_group", CloudID: *sg.GroupId}
		r, err := a.fetchSecurityGroup(ctx, ref)
		if err == nil {
			res = append(res, r)
		}
	}
	return res, nil
}

func (a *Adapter) listBuckets(ctx context.Context) ([]*models.Resource, error) {
	out, err := a.s3.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, err
	}
	var res []*models.Resource
	for _, b := range out.Buckets {
		if b.Name == nil {
			continue
		}
		ref := cloudtypes.ResourceRef{Address: *b.Name, Type: "aws_s3_bucket", CloudID: *b.Name}
		r, err := a.fetchS3Bucket(ctx, ref)
		if err == nil {
			res = append(res, r)
		}
	}
	return res, nil
}

func (a *Adapter) listVPCs(ctx context.Context) ([]*models.Resource, error) {
	out, err := a.ec2.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{})
	if err != nil {
		return nil, err
	}
	var res []*models.Resource
	for _, vpc := range out.Vpcs {
		if vpc.VpcId == nil {
			continue
		}
		ref := cloudtypes.ResourceRef{Address: *vpc.VpcId, Type: "aws_vpc", CloudID: *vpc.VpcId}
		r, err := a.fetchVPC(ctx, ref)
		if err == nil {
			res = append(res, r)
		}
	}
	return res, nil
}

func (a *Adapter) listSubnets(ctx context.Context) ([]*models.Resource, error) {
	out, err := a.ec2.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{})
	if err != nil {
		return nil, err
	}
	var res []*models.Resource
	for _, subnet := range out.Subnets {
		if subnet.SubnetId == nil {
			continue
		}
		ref := cloudtypes.ResourceRef{Address: *subnet.SubnetId, Type: "aws_subnet", CloudID: *subnet.SubnetId}
		r, err := a.fetchSubnet(ctx, ref)
		if err == nil {
			res = append(res, r)
		}
	}
	return res, nil
}
