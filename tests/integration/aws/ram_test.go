package aws_test

import (
	"context"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ram"
	ramtypes "github.com/aws/aws-sdk-go-v2/service/ram/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws-resolver-rules-operator/pkg/aws"
	"github.com/aws-resolver-rules-operator/pkg/resolver"
	"github.com/aws-resolver-rules-operator/tests"
)

var _ = Describe("RAM client", func() {
	var (
		ctx context.Context

		name       string
		share      resolver.ResourceShare
		prefixList *ec2types.ManagedPrefixList

		ramClient resolver.RAMClient
	)

	waitForResourceShareAvailability := func() {
		// The resource share can only be deleted when the resource and
		// principal are in a final state like Associated. If we don't
		// wait for this state the tests will flake
		Eventually(getResourceAssociationStatus(name, prefixList)).
			Should(PointTo(Equal(ramtypes.ResourceShareAssociationStatusAssociated)))

		Eventually(getPrincipalAssociationStatus(name)).
			Should(PointTo(Equal(ramtypes.ResourceShareAssociationStatusAssociated)))
	}

	BeforeEach(func() {
		SetDefaultEventuallyTimeout(10 * time.Second)
		SetDefaultConsistentlyPollingInterval(100 * time.Millisecond)

		ctx = log.IntoContext(context.Background(), logger)
		name = tests.GenerateGUID("test")

		prefixList = createManagedPrefixList(rawEC2Client, name)

		share = resolver.ResourceShare{
			Name:              name,
			ResourceArns:      []string{*prefixList.PrefixListArn},
			ExternalAccountID: wcAccount,
		}

		var err error
		awsClients := aws.NewClients("")
		ramClient, err = awsClients.NewRAMClient(awsRegion, mcIAMRoleARN)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		getResourceShareOutput, err := rawRamClient.GetResourceShares(context.TODO(), &ram.GetResourceSharesInput{
			Name:          awssdk.String(name),
			ResourceOwner: ramtypes.ResourceOwnerSelf,
		})
		Expect(err).NotTo(HaveOccurred())
		for _, resourceShare := range getResourceShareOutput.ResourceShares {
			if isResourceShareDeleted(resourceShare) {
				continue
			}

			_, err = rawRamClient.DeleteResourceShare(context.TODO(), &ram.DeleteResourceShareInput{
				ResourceShareArn: resourceShare.ResourceShareArn,
			})
			Expect(err).NotTo(HaveOccurred())
		}

		_, err = rawEC2Client.DeleteManagedPrefixList(context.TODO(), &ec2.DeleteManagedPrefixListInput{
			PrefixListId: prefixList.PrefixListId,
		})
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("ApplyResourceShare", func() {
		It("creates the share resource", func() {
			err := ramClient.ApplyResourceShare(ctx, share)
			Expect(err).NotTo(HaveOccurred())
			waitForResourceShareAvailability()

			Eventually(getSharedResources(rawRamClient, prefixList)).Should(HaveLen(1))
		})

		When("the resource is in the same account", func() {
			It("does not create a resource share", func() {
				share.ExternalAccountID = mcAccount
				err := ramClient.ApplyResourceShare(ctx, share)
				Expect(err).NotTo(HaveOccurred())
				Consistently(getResourceShares(rawRamClient, share.Name)).Should(HaveLen(0))
				Consistently(getSharedResources(rawRamClient, prefixList)).Should(HaveLen(0))
			})
		})

		When("the resource has already been shared", func() {
			BeforeEach(func() {
				err := ramClient.ApplyResourceShare(ctx, share)
				Expect(err).NotTo(HaveOccurred())
				waitForResourceShareAvailability()

				Eventually(getSharedResources(rawRamClient, prefixList)).Should(HaveLen(1))
			})

			It("does not return an error", func() {
				err := ramClient.ApplyResourceShare(ctx, share)
				Expect(err).NotTo(HaveOccurred())

				Consistently(getSharedResources(rawRamClient, prefixList)).Should(HaveLen(1))
			})
		})

		When("when the resource is not owned by the MC account", func() {
			BeforeEach(func() {
				share.ResourceArns = []string{
					"arn:aws:ec2:eu-north-1:012345678901:prefix-list/pl-0123456789abcdeff",
				}
			})

			It("returns an error", func() {
				err := ramClient.ApplyResourceShare(ctx, share)
				Expect(err).To(HaveOccurred())
			})
		})

		When("when the resource arn is invalid", func() {
			BeforeEach(func() {
				share.ResourceArns = []string{
					"this:is:not::an/arn",
				}
			})

			It("returns an error", func() {
				err := ramClient.ApplyResourceShare(ctx, share)
				Expect(err).To(HaveOccurred())
			})
		})

		When("the destination account is invalid", func() {
			BeforeEach(func() {
				share.ExternalAccountID = "notavalidaccount"
			})

			It("returns an error", func() {
				err := ramClient.ApplyResourceShare(ctx, share)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("DeleteResourceShare", func() {
		BeforeEach(func() {
			err := ramClient.ApplyResourceShare(ctx, share)
			Expect(err).NotTo(HaveOccurred())
			Eventually(getSharedResources(rawRamClient, prefixList)).Should(HaveLen(1))

			waitForResourceShareAvailability()
		})

		It("deletes the resource share", func() {
			err := ramClient.DeleteResourceShare(ctx, name)
			Expect(err).NotTo(HaveOccurred())

			Eventually(getSharedResources(rawRamClient, prefixList)).Should(HaveLen(0))
		})

		When("the resource is already deleted", func() {
			BeforeEach(func() {
				err := ramClient.DeleteResourceShare(ctx, name)
				Expect(err).NotTo(HaveOccurred())

				Eventually(getSharedResources(rawRamClient, prefixList)).Should(HaveLen(0))
			})

			It("does not return an error", func() {
				err := ramClient.DeleteResourceShare(ctx, name)
				Expect(err).NotTo(HaveOccurred())
			})

			When("creating a resource share with the same name", func() {
				It("creates the share resource", func() {
					err := ramClient.ApplyResourceShare(ctx, share)
					Expect(err).NotTo(HaveOccurred())

					Eventually(getSharedResources(rawRamClient, prefixList)).Should(HaveLen(1))
					waitForResourceShareAvailability()
				})

				When("the resource has already been recreated", func() {
					BeforeEach(func() {
						err := ramClient.ApplyResourceShare(ctx, share)
						Expect(err).NotTo(HaveOccurred())
						waitForResourceShareAvailability()
					})

					It("does not return an error", func() {
						err := ramClient.ApplyResourceShare(ctx, share)
						Expect(err).NotTo(HaveOccurred())
					})
				})
			})
		})
	})
})
