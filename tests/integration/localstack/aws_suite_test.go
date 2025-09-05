/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package aws_test

import (
	"context"
	"os"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53resolver"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	// +kubebuilder:scaffold:imports

	"github.com/aws-resolver-rules-operator/pkg/aws"
	"github.com/aws-resolver-rules-operator/pkg/resolver"
	"github.com/aws-resolver-rules-operator/tests"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AWS Suite")
}

const (
	AwsIamArn              = "arn:aws:iam::1234567890:role/IamRole"
	Region                 = "eu-central-1"
	LocalstackAWSAccountId = "000000000000"
)

var (
	additionalTags    map[string]string
	awsClients        resolver.AWSClients
	ctx               context.Context
	ec2Client         resolver.EC2Client
	logger            logr.Logger
	resolverClient    resolver.ResolverClient
	route53Client     resolver.Route53Client
	err               error
	rawEC2Client      *ec2.Client
	rawResolverClient *route53resolver.Client
	rawRoute53Client  *route53.Client
	subnets           []string
	MCVPCId           string
	VPCId             string
)

var _ = BeforeSuite(func() {
	logger = GinkgoLogr
	logf.SetLogger(logger)
	tests.GetEnvOrSkip("AWS_ENDPOINT")
	additionalTags = map[string]string{
		"test": "test-tag",
	}

	awsClients = aws.NewClients(os.Getenv("AWS_ENDPOINT"))

	rawEC2Client, err = NewEC2Client(Region, AwsIamArn, "")
	Expect(err).NotTo(HaveOccurred())

	rawResolverClient, err = NewResolverClient(Region, AwsIamArn, "")
	Expect(err).NotTo(HaveOccurred())

	rawRoute53Client, err = NewRoute53Client(Region, AwsIamArn, "")
	Expect(err).NotTo(HaveOccurred())

	createMCVPCResponse, err := rawEC2Client.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock: awssdk.String("10.0.0.0/16"),
	})
	Expect(err).NotTo(HaveOccurred())
	MCVPCId = *createMCVPCResponse.Vpc.VpcId

	createVPCResponse, err := rawEC2Client.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock: awssdk.String("10.0.0.0/16"),
	})
	Expect(err).NotTo(HaveOccurred())
	VPCId = *createVPCResponse.Vpc.VpcId

	createSubnetResponse1, err := rawEC2Client.CreateSubnet(ctx, &ec2.CreateSubnetInput{
		CidrBlock: awssdk.String("10.0.0.0/24"),
		VpcId:     awssdk.String(VPCId),
	})
	Expect(err).NotTo(HaveOccurred())

	createSubnetResponse2, err := rawEC2Client.CreateSubnet(ctx, &ec2.CreateSubnetInput{
		CidrBlock: awssdk.String("10.0.1.0/24"),
		VpcId:     awssdk.String(VPCId),
	})
	Expect(err).NotTo(HaveOccurred())
	subnets = []string{*createSubnetResponse1.Subnet.SubnetId, *createSubnetResponse2.Subnet.SubnetId}
})

func NewEC2Client(region, arn, externalId string) (*ec2.Client, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	stsClient := sts.NewFromConfig(cfg)
	client := ec2.NewFromConfig(cfg, func(o *ec2.Options) {
		o.BaseEndpoint = awssdk.String(os.Getenv("AWS_ENDPOINT"))
		o.Credentials = stscreds.NewAssumeRoleProvider(stsClient, arn, func(aro *stscreds.AssumeRoleOptions) {
			if externalId != "" {
				aro.ExternalID = awssdk.String(externalId)
			}
		})
	})

	return client, nil
}

func NewResolverClient(region, arn, externalId string) (*route53resolver.Client, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	stsClient := sts.NewFromConfig(cfg)
	client := route53resolver.NewFromConfig(cfg, func(o *route53resolver.Options) {
		o.BaseEndpoint = awssdk.String(os.Getenv("AWS_ENDPOINT"))
		o.Credentials = stscreds.NewAssumeRoleProvider(stsClient, arn, func(aro *stscreds.AssumeRoleOptions) {
			if externalId != "" {
				aro.ExternalID = awssdk.String(externalId)
			}
		})
	})

	return client, nil
}

func NewRoute53Client(region, arn, externalId string) (*route53.Client, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	stsClient := sts.NewFromConfig(cfg)
	client := route53.NewFromConfig(cfg, func(o *route53.Options) {
		o.BaseEndpoint = awssdk.String(os.Getenv("AWS_ENDPOINT"))
		o.Credentials = stscreds.NewAssumeRoleProvider(stsClient, arn, func(aro *stscreds.AssumeRoleOptions) {
			if externalId != "" {
				aro.ExternalID = awssdk.String(externalId)
			}
		})
	})

	return client, nil
}
