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

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/route53resolver"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

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
	AWS_IAM_ARN = "arn:aws:iam::1234567890:role/IamRole"
	Region      = "eu-central-1"
)

var (
	awsClients        resolver.AWSClients
	ctx               context.Context
	ec2Client         resolver.EC2Client
	resolverClient    resolver.ResolverClient
	err               error
	rawEC2Client      *ec2.EC2
	rawResolverClient *route53resolver.Route53Resolver
	subnets           []string
	VPCId             string
)

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	tests.GetEnvOrSkip("AWS_ENDPOINT")

	awsClients = aws.NewClients(os.Getenv("AWS_ENDPOINT"))

	rawEC2Client, err = NewEC2Client(Region, AWS_IAM_ARN, "")
	Expect(err).NotTo(HaveOccurred())

	rawResolverClient, err = NewResolverClient(Region, AWS_IAM_ARN, "")
	Expect(err).NotTo(HaveOccurred())

	createVPCResponse, err := rawEC2Client.CreateVpc(&ec2.CreateVpcInput{
		CidrBlock: awssdk.String("10.0.0.0/16"),
	})
	Expect(err).NotTo(HaveOccurred())
	VPCId = *createVPCResponse.Vpc.VpcId

	createSubnetResponse1, err := rawEC2Client.CreateSubnet(&ec2.CreateSubnetInput{
		CidrBlock: awssdk.String("10.0.0.0/24"),
		VpcId:     awssdk.String(VPCId),
	})
	Expect(err).NotTo(HaveOccurred())

	createSubnetResponse2, err := rawEC2Client.CreateSubnet(&ec2.CreateSubnetInput{
		CidrBlock: awssdk.String("10.0.1.0/24"),
		VpcId:     awssdk.String(VPCId),
	})
	Expect(err).NotTo(HaveOccurred())
	subnets = []string{*createSubnetResponse1.Subnet.SubnetId, *createSubnetResponse2.Subnet.SubnetId}
})

func NewEC2Client(region, arn, externalId string) (*ec2.EC2, error) {
	session, err := awssession.NewSession(&awssdk.Config{
		Region:   awssdk.String(region),
		Endpoint: awssdk.String(os.Getenv("AWS_ENDPOINT")),
	})
	if err != nil {
		return nil, err
	}

	client := ec2.New(session, &awssdk.Config{Credentials: stscreds.NewCredentials(session, arn, func(assumeRoleProvider *stscreds.AssumeRoleProvider) {
		if externalId != "" {
			assumeRoleProvider.ExternalID = awssdk.String(externalId)
		}
	})})

	return client, nil
}

func NewResolverClient(region, arn, externalId string) (*route53resolver.Route53Resolver, error) {
	session, err := awssession.NewSession(&awssdk.Config{
		Region:   awssdk.String(region),
		Endpoint: awssdk.String(os.Getenv("AWS_ENDPOINT")),
	})
	if err != nil {
		return nil, err
	}

	client := route53resolver.New(session, &awssdk.Config{Credentials: stscreds.NewCredentials(session, arn, func(assumeRoleProvider *stscreds.AssumeRoleProvider) {
		if externalId != "" {
			assumeRoleProvider.ExternalID = awssdk.String(externalId)
		}
	})})

	return client, nil
}
