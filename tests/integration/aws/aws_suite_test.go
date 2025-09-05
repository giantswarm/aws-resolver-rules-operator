package aws_test

import (
	"context"
	"fmt"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ram"
	ramtypes "github.com/aws/aws-sdk-go-v2/service/ram/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap/zapcore"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/aws-resolver-rules-operator/tests"
)

var (
	logger logr.Logger

	mcAccount string
	wcAccount string
	iamRoleId string
	awsRegion string

	mcIAMRoleARN string
	wcIAMRoleARN string

	rawEC2Client *ec2.Client
	rawRamClient *ram.Client
)

func TestAws(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Aws Suite")
}

var _ = BeforeSuite(func() {
	opts := zap.Options{
		DestWriter:  GinkgoWriter,
		Development: true,
		TimeEncoder: zapcore.RFC3339TimeEncoder,
	}

	logger = zap.New(zap.UseFlagOptions(&opts))

	mcAccount = tests.GetEnvOrSkip("MC_AWS_ACCOUNT")
	wcAccount = tests.GetEnvOrSkip("WC_AWS_ACCOUNT")
	iamRoleId = tests.GetEnvOrSkip("AWS_IAM_ROLE_ID")
	awsRegion = tests.GetEnvOrSkip("AWS_REGION")

	mcIAMRoleARN = fmt.Sprintf("arn:aws:iam::%s:role/%s", mcAccount, iamRoleId)
	wcIAMRoleARN = fmt.Sprintf("arn:aws:iam::%s:role/%s", wcAccount, iamRoleId)

	cfg, err := awsconfig.LoadDefaultConfig(context.TODO(),
		awsconfig.WithRegion(awsRegion),
	)
	Expect(err).NotTo(HaveOccurred())

	stsClient := sts.NewFromConfig(cfg)
	rawEC2Client = ec2.NewFromConfig(cfg, func(o *ec2.Options) {
		o.Credentials = stscreds.NewAssumeRoleProvider(stsClient, mcIAMRoleARN)
	})

	rawRamClient = ram.NewFromConfig(cfg, func(o *ram.Options) {
		o.Credentials = stscreds.NewAssumeRoleProvider(stsClient, mcIAMRoleARN)
	})
})

func getResourceAssociationStatus(resourceShareName string, prefixList *ec2types.ManagedPrefixList) func(g Gomega) ramtypes.ResourceShareAssociationStatus {
	return func(g Gomega) ramtypes.ResourceShareAssociationStatus {
		getResourceShareOutput, err := rawRamClient.GetResourceShares(context.TODO(), &ram.GetResourceSharesInput{
			Name:          awssdk.String(resourceShareName),
			ResourceOwner: ramtypes.ResourceOwnerSelf,
		})
		Expect(err).NotTo(HaveOccurred())
		resourceShares := []ramtypes.ResourceShare{}
		for _, share := range getResourceShareOutput.ResourceShares {
			if !isResourceShareDeleted(share) {
				resourceShares = append(resourceShares, share)
			}
		}
		Expect(resourceShares).To(HaveLen(1))

		resourceShare := resourceShares[0]

		listAssociationsOutput, err := rawRamClient.GetResourceShareAssociations(context.TODO(), &ram.GetResourceShareAssociationsInput{
			AssociationType:   ramtypes.ResourceShareAssociationTypeResource,
			ResourceArn:       prefixList.PrefixListArn,
			ResourceShareArns: []string{*resourceShare.ResourceShareArn},
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(listAssociationsOutput.ResourceShareAssociations).To(HaveLen(1))
		return listAssociationsOutput.ResourceShareAssociations[0].Status
	}
}

func getPrincipalAssociationStatus(resourceShareName string) func(g Gomega) ramtypes.ResourceShareAssociationStatus {
	return func(g Gomega) ramtypes.ResourceShareAssociationStatus {
		getResourceShareOutput, err := rawRamClient.GetResourceShares(context.TODO(), &ram.GetResourceSharesInput{
			Name:          awssdk.String(resourceShareName),
			ResourceOwner: ramtypes.ResourceOwnerSelf,
		})
		Expect(err).NotTo(HaveOccurred())
		resourceShares := []ramtypes.ResourceShare{}
		for _, share := range getResourceShareOutput.ResourceShares {
			if !isResourceShareDeleted(share) {
				resourceShares = append(resourceShares, share)
			}
		}
		Expect(resourceShares).To(HaveLen(1))

		resourceShare := resourceShares[0]
		listAssociationsOutput, err := rawRamClient.GetResourceShareAssociations(context.TODO(), &ram.GetResourceShareAssociationsInput{
			AssociationType:   ramtypes.ResourceShareAssociationTypePrincipal,
			Principal:         awssdk.String(wcAccount),
			ResourceShareArns: []string{*resourceShare.ResourceShareArn},
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(listAssociationsOutput.ResourceShareAssociations).To(HaveLen(1))
		return listAssociationsOutput.ResourceShareAssociations[0].Status
	}
}

func getResourceShares(ramClient *ram.Client, name string) func(g Gomega) []ramtypes.ResourceShare {
	return func(g Gomega) []ramtypes.ResourceShare {
		resourceShareOutput, err := ramClient.GetResourceShares(context.TODO(), &ram.GetResourceSharesInput{
			Name:          awssdk.String(name),
			ResourceOwner: ramtypes.ResourceOwnerSelf,
		})
		g.Expect(err).NotTo(HaveOccurred())
		return resourceShareOutput.ResourceShares
	}
}

func getSharedResources(ramClient *ram.Client, prefixList *ec2types.ManagedPrefixList) func(g Gomega) []ramtypes.Resource {
	return func(g Gomega) []ramtypes.Resource {
		listResourcesOutput, err := ramClient.ListResources(context.TODO(), &ram.ListResourcesInput{
			Principal:     awssdk.String(wcAccount),
			ResourceArns:  []string{*prefixList.PrefixListArn},
			ResourceOwner: ramtypes.ResourceOwnerSelf,
		})
		g.Expect(err).NotTo(HaveOccurred())
		return listResourcesOutput.Resources
	}
}

func createManagedPrefixList(ec2Client *ec2.Client, name string) *ec2types.ManagedPrefixList {
	createPrefixListOutput, err := ec2Client.CreateManagedPrefixList(context.TODO(), &ec2.CreateManagedPrefixListInput{
		AddressFamily:  awssdk.String("IPv4"),
		MaxEntries:     awssdk.Int32(2),
		PrefixListName: awssdk.String(name),
	})
	Expect(err).NotTo(HaveOccurred())
	prefixList := createPrefixListOutput.PrefixList
	Eventually(func() ec2types.PrefixListState {
		prefixListOutput, err := ec2Client.DescribeManagedPrefixLists(context.TODO(), &ec2.DescribeManagedPrefixListsInput{
			PrefixListIds: []string{*prefixList.PrefixListId},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(prefixListOutput.PrefixLists).To(HaveLen(1))
		return prefixListOutput.PrefixLists[0].State
	}).Should(Equal(ec2types.PrefixListStateCreateComplete))

	return prefixList
}

func isResourceShareDeleted(resourceShare ramtypes.ResourceShare) bool {
	if resourceShare.Status == "" {
		return false
	}

	status := resourceShare.Status
	return status == ramtypes.ResourceShareStatusDeleted || status == ramtypes.ResourceShareStatusDeleting
}
