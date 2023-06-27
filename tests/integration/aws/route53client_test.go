package aws_test

import (
	"context"
	"fmt"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/route53"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Route53 Resolver client", func() {
	BeforeEach(func() {
		ctx = context.Background()

		route53Client, err = awsClients.NewRoute53Client(Region, AwsIamArn)
		Expect(err).NotTo(HaveOccurred())
	})

	When("creating hosted zones", func() {
		When("there is no hosted zone", func() {
			When("we want a public hosted zone", func() {
				var publicHostedZoneId string

				AfterEach(func() {
					_, err = rawRoute53Client.DeleteHostedZoneWithContext(ctx, &route53.DeleteHostedZoneInput{Id: awssdk.String(publicHostedZoneId)})
					Expect(err).NotTo(HaveOccurred())
				})

				It("creates a public hosted zone successfully", func() {
					tags := map[string]string{
						"Name":      "jose",
						"something": "else",
					}
					publicHostedZoneId, err = route53Client.CreatePublicHostedZone(ctx, logger, "josepublic.test.example.com", tags)
					Expect(err).NotTo(HaveOccurred())

					var publicListHostedZoneResponse *route53.ListHostedZonesByNameOutput
					Eventually(func() (int, error) {
						publicListHostedZoneResponse, err = rawRoute53Client.ListHostedZonesByNameWithContext(ctx, &route53.ListHostedZonesByNameInput{
							DNSName:  awssdk.String("josepublic.test.example.com"),
							MaxItems: awssdk.String("1"),
						})
						return len(publicListHostedZoneResponse.HostedZones), err
					}, "2s", "100ms").Should(Equal(1))

					actualTags, err := rawRoute53Client.ListTagsForResourceWithContext(ctx, &route53.ListTagsForResourceInput{
						ResourceId:   publicListHostedZoneResponse.HostedZones[0].Id,
						ResourceType: awssdk.String("hostedzone"),
					})
					Expect(err).NotTo(HaveOccurred())

					Expect(actualTags.ResourceTagSet.Tags).To(ContainElement(&route53.Tag{
						Key:   awssdk.String("Name"),
						Value: awssdk.String("jose"),
					}))
					Expect(actualTags.ResourceTagSet.Tags).To(ContainElement(&route53.Tag{
						Key:   awssdk.String("something"),
						Value: awssdk.String("else"),
					}))
				})
			})

			When("we want a private hosted zone", func() {
				var privateHostedZoneId string

				AfterEach(func() {
					_, err = rawRoute53Client.DeleteHostedZoneWithContext(ctx, &route53.DeleteHostedZoneInput{Id: awssdk.String(privateHostedZoneId)})
					Expect(err).NotTo(HaveOccurred())
				})

				It("creates a private hosted zone successfully", func() {
					tags := map[string]string{
						"Name":      "jose",
						"something": "else",
					}
					privateHostedZoneId, err = route53Client.CreatePrivateHostedZone(ctx, logger, "joseprivate.test.example.com", VPCId, Region, tags, []string{MCVPCId})
					Expect(err).NotTo(HaveOccurred())

					var privateHostedZoneResponse *route53.ListHostedZonesByNameOutput
					Eventually(func() (int, error) {
						privateHostedZoneResponse, err = rawRoute53Client.ListHostedZonesByNameWithContext(ctx, &route53.ListHostedZonesByNameInput{
							DNSName:  awssdk.String("joseprivate.test.example.com"),
							MaxItems: awssdk.String("1"),
						})
						return len(privateHostedZoneResponse.HostedZones), err
					}, "2s", "100ms").Should(Equal(1))

					actualTags, err := rawRoute53Client.ListTagsForResourceWithContext(ctx, &route53.ListTagsForResourceInput{
						ResourceId:   privateHostedZoneResponse.HostedZones[0].Id,
						ResourceType: awssdk.String("hostedzone"),
					})
					Expect(err).NotTo(HaveOccurred())

					Expect(actualTags.ResourceTagSet.Tags).To(ContainElement(&route53.Tag{
						Key:   awssdk.String("Name"),
						Value: awssdk.String("jose"),
					}))

					associatedHostedZones, err := rawRoute53Client.ListHostedZonesByVPCWithContext(ctx, &route53.ListHostedZonesByVPCInput{
						VPCId:     awssdk.String(MCVPCId),
						VPCRegion: awssdk.String(Region),
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(associatedHostedZones.HostedZoneSummaries).To(ContainElement(&route53.HostedZoneSummary{
						HostedZoneId: awssdk.String(strings.TrimPrefix(*privateHostedZoneResponse.HostedZones[0].Id, "/hostedzone/")),
						Name:         awssdk.String("joseprivate.test.example.com."),
						Owner: &route53.HostedZoneOwner{
							OwningAccount: awssdk.String("000000000000"),
						},
					}))
				})
			})
		})

		When("the hosted zone already exists", func() {
			var alreadyExistingHostedZone *route53.CreateHostedZoneOutput
			BeforeEach(func() {
				now := time.Now()
				alreadyExistingHostedZone, err = rawRoute53Client.CreateHostedZone(&route53.CreateHostedZoneInput{
					CallerReference: awssdk.String(fmt.Sprintf("1%d", now.UnixNano())),
					Name:            awssdk.String("already.exists.test.example.com"),
				})
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				_, err = rawRoute53Client.DeleteHostedZoneWithContext(ctx, &route53.DeleteHostedZoneInput{Id: alreadyExistingHostedZone.HostedZone.Id})
				Expect(err).NotTo(HaveOccurred())
			})

			It("doesn't return error", func() {
				_, err = route53Client.CreatePublicHostedZone(ctx, logger, "already.exists.test.example.com", nil)
				Expect(err).NotTo(HaveOccurred())

				hostedZoneResponse, err := rawRoute53Client.ListHostedZonesByNameWithContext(ctx, &route53.ListHostedZonesByNameInput{
					DNSName:  awssdk.String("already.exists.test.example.com"),
					MaxItems: awssdk.String("1"),
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(len(hostedZoneResponse.HostedZones)).To(Equal(1))
			})
		})
	})

	When("fetching hosted zone id by name", func() {
		var hostedZoneToFind, differentHostedZoneToFind *route53.CreateHostedZoneOutput
		BeforeEach(func() {
			now := time.Now()
			hostedZoneToFind, err = rawRoute53Client.CreateHostedZone(&route53.CreateHostedZoneInput{
				CallerReference: awssdk.String(fmt.Sprintf("1%d", now.UnixNano())),
				Name:            awssdk.String("findid.test.example.com"),
			})
			Expect(err).NotTo(HaveOccurred())

			differentHostedZoneToFind, err = rawRoute53Client.CreateHostedZone(&route53.CreateHostedZoneInput{
				CallerReference: awssdk.String(fmt.Sprintf("1%d", now.UnixNano())),
				Name:            awssdk.String("different.test.example.com"),
			})
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_, err = rawRoute53Client.DeleteHostedZoneWithContext(ctx, &route53.DeleteHostedZoneInput{Id: hostedZoneToFind.HostedZone.Id})
			Expect(err).NotTo(HaveOccurred())
			_, err = rawRoute53Client.DeleteHostedZoneWithContext(ctx, &route53.DeleteHostedZoneInput{Id: differentHostedZoneToFind.HostedZone.Id})
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the id", func() {
			hostedZoneId, err := route53Client.GetHostedZoneIdByName(ctx, logger, "findid.test.example.com")
			Expect(err).NotTo(HaveOccurred())
			Expect(hostedZoneId).To(Equal(*hostedZoneToFind.HostedZone.Id))
		})

		It("returns error when hosted zone does not exist", func() {
			_, err = route53Client.GetHostedZoneIdByName(ctx, logger, "nonexisting.test.example.com.")
			Expect(err).To(HaveOccurred())
		})
	})

	When("adding delegation to parent hosted zone", func() {
		var parentHostedZoneToFind *route53.CreateHostedZoneOutput
		var hostedZoneToFind *route53.CreateHostedZoneOutput

		BeforeEach(func() {
			now := time.Now()
			parentHostedZoneToFind, err = rawRoute53Client.CreateHostedZone(&route53.CreateHostedZoneInput{
				CallerReference: awssdk.String(fmt.Sprintf("1%d", now.UnixNano())),
				Name:            awssdk.String("test.example.com"),
			})
			Expect(err).NotTo(HaveOccurred())

			hostedZoneToFind, err = rawRoute53Client.CreateHostedZone(&route53.CreateHostedZoneInput{
				CallerReference: awssdk.String(fmt.Sprintf("1%d", now.UnixNano())),
				Name:            awssdk.String("different.test.example.com"),
			})
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_, err = rawRoute53Client.DeleteHostedZoneWithContext(ctx, &route53.DeleteHostedZoneInput{Id: parentHostedZoneToFind.HostedZone.Id})
			Expect(err).NotTo(HaveOccurred())
			_, err = rawRoute53Client.DeleteHostedZoneWithContext(ctx, &route53.DeleteHostedZoneInput{Id: hostedZoneToFind.HostedZone.Id})
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates the dns records", func() {
			listRecordSets, err := rawRoute53Client.ListResourceRecordSets(&route53.ListResourceRecordSetsInput{
				HostedZoneId: hostedZoneToFind.HostedZone.Id,
				MaxItems:     awssdk.String("1"), // First entry is always NS record
			})
			Expect(err).NotTo(HaveOccurred())

			err = route53Client.AddDelegationToParentZone(ctx, logger, *parentHostedZoneToFind.HostedZone.Id, *hostedZoneToFind.HostedZone.Id)
			Expect(err).NotTo(HaveOccurred())

			listParentRecordSets, err := rawRoute53Client.ListResourceRecordSets(&route53.ListResourceRecordSetsInput{
				HostedZoneId: parentHostedZoneToFind.HostedZone.Id,
			})
			Expect(err).NotTo(HaveOccurred())

			found := false
			for _, recordSet := range listParentRecordSets.ResourceRecordSets {
				if *recordSet.Name == *hostedZoneToFind.HostedZone.Name {
					found = true
					Expect(recordSet.ResourceRecords).To(Equal(listRecordSets.ResourceRecordSets[0].ResourceRecords))
				}
			}
			Expect(found).To(BeTrue())
		})
	})

	When("deleting a hosted zone", func() {
		When("the zone exists", func() {
			var hostedZoneToFind *route53.CreateHostedZoneOutput

			BeforeEach(func() {
				now := time.Now()

				hostedZoneToFind, err = rawRoute53Client.CreateHostedZone(&route53.CreateHostedZoneInput{
					CallerReference: awssdk.String(fmt.Sprintf("1%d", now.UnixNano())),
					Name:            awssdk.String("deleting.test.example.com"),
				})
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				// We don't check error here because we actually expect an error since we just removed the hosted zone.
				_, err = rawRoute53Client.DeleteHostedZoneWithContext(ctx, &route53.DeleteHostedZoneInput{Id: hostedZoneToFind.HostedZone.Id})
			})

			It("deletes the zone", func() {
				err = route53Client.DeleteHostedZone(ctx, logger, *hostedZoneToFind.HostedZone.Id)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() (int, error) {
					listHostedZoneResponse, err := rawRoute53Client.ListHostedZonesByName(&route53.ListHostedZonesByNameInput{
						DNSName:  awssdk.String("deleting.test.example.com"),
						MaxItems: awssdk.String("1"),
					})
					return len(listHostedZoneResponse.HostedZones), err
				}, "3s", "500ms").Should(BeZero())
			})
		})

		When("the zone doesn't exist", func() {
			It("deletes the zone", func() {
				err = route53Client.DeleteHostedZone(ctx, logger, "non-existing-zone.example.com")
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	When("deleting delegation to parent hosted zone", func() {
		var parentHostedZoneToFind *route53.CreateHostedZoneOutput
		var hostedZoneToFind *route53.CreateHostedZoneOutput

		BeforeEach(func() {
			now := time.Now()
			parentHostedZoneToFind, err = rawRoute53Client.CreateHostedZone(&route53.CreateHostedZoneInput{
				CallerReference: awssdk.String(fmt.Sprintf("1%d", now.UnixNano())),
				Name:            awssdk.String("test.example.com"),
			})
			Expect(err).NotTo(HaveOccurred())

			hostedZoneToFind, err = rawRoute53Client.CreateHostedZone(&route53.CreateHostedZoneInput{
				CallerReference: awssdk.String(fmt.Sprintf("1%d", now.UnixNano())),
				Name:            awssdk.String("different.test.example.com"),
			})
			Expect(err).NotTo(HaveOccurred())

			err = route53Client.AddDelegationToParentZone(ctx, logger, *parentHostedZoneToFind.HostedZone.Id, *hostedZoneToFind.HostedZone.Id)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_, err = rawRoute53Client.DeleteHostedZoneWithContext(ctx, &route53.DeleteHostedZoneInput{Id: parentHostedZoneToFind.HostedZone.Id})
			Expect(err).NotTo(HaveOccurred())
			_, err = rawRoute53Client.DeleteHostedZoneWithContext(ctx, &route53.DeleteHostedZoneInput{Id: hostedZoneToFind.HostedZone.Id})
			Expect(err).NotTo(HaveOccurred())
		})

		It("deletes the delegation from the parent zone", func() {
			err = route53Client.DeleteDelegationFromParentZone(ctx, logger, *parentHostedZoneToFind.HostedZone.Id, *hostedZoneToFind.HostedZone.Id)
			Expect(err).NotTo(HaveOccurred())

			listParentRecordSets, err := rawRoute53Client.ListResourceRecordSets(&route53.ListResourceRecordSetsInput{
				HostedZoneId: parentHostedZoneToFind.HostedZone.Id,
			})
			Expect(err).NotTo(HaveOccurred())

			found := false
			for _, recordSet := range listParentRecordSets.ResourceRecordSets {
				if *recordSet.Name == *hostedZoneToFind.HostedZone.Name {
					found = true
				}
			}
			Expect(found).To(BeFalse())
		})
	})
})
