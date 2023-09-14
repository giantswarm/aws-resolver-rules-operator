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

	"github.com/aws-resolver-rules-operator/pkg/resolver"
)

var _ = Describe("Route53 Resolver client", func() {
	BeforeEach(func() {
		ctx = context.Background()

		route53Client, err = awsClients.NewRoute53Client(Region, AwsIamArn)
		Expect(err).NotTo(HaveOccurred())
	})

	When("creating hosted zones", func() {
		tags := map[string]string{
			"Name":      "jose",
			"something": "else",
		}

		Context("we want a public hosted zone", func() {
			var hostedZoneId string

			AfterEach(func() {
				_, _ = rawRoute53Client.DeleteHostedZoneWithContext(ctx, &route53.DeleteHostedZoneInput{Id: awssdk.String(hostedZoneId)})
			})

			It("creates a public hosted zone successfully", func() {
				hostedZoneId, err = route53Client.CreateHostedZone(ctx, logger, resolver.BuildPublicHostedZone("apublic.test.example.com", tags))
				Expect(err).NotTo(HaveOccurred())

				var publicListHostedZoneResponse *route53.ListHostedZonesByNameOutput
				Eventually(func() (int, error) {
					publicListHostedZoneResponse, err = rawRoute53Client.ListHostedZonesByNameWithContext(ctx, &route53.ListHostedZonesByNameInput{
						DNSName:  awssdk.String("apublic.test.example.com"),
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

			When("the hosted zone already exists", func() {
				BeforeEach(func() {
					now := time.Now()
					_, err = rawRoute53Client.CreateHostedZone(&route53.CreateHostedZoneInput{
						CallerReference: awssdk.String(fmt.Sprintf("1%d", now.UnixNano())),
						Name:            awssdk.String("already.public.exists.test.example.com"),
					})
					Expect(err).NotTo(HaveOccurred())
				})

				It("doesn't return error", func() {
					hostedZoneId, err = route53Client.CreateHostedZone(ctx, logger, resolver.BuildPublicHostedZone("already.public.exists.test.example.com", tags))
					Expect(err).NotTo(HaveOccurred())

					hostedZoneResponse, err := rawRoute53Client.ListHostedZonesByNameWithContext(ctx, &route53.ListHostedZonesByNameInput{
						DNSName:  awssdk.String("already.public.exists.test.example.com"),
						MaxItems: awssdk.String("1"),
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(len(hostedZoneResponse.HostedZones)).To(Equal(1))
				})
			})
		})

		Context("we want a private hosted zone", func() {
			var hostedZoneId string

			AfterEach(func() {
				_, _ = rawRoute53Client.DeleteHostedZoneWithContext(ctx, &route53.DeleteHostedZoneInput{Id: awssdk.String(hostedZoneId)})
			})

			It("creates a private hosted zone successfully", func() {
				dnsZone := resolver.BuildPrivateHostedZone("aprivate.test.example.com", tags, VPCId, Region, []string{MCVPCId})
				hostedZoneId, err = route53Client.CreateHostedZone(ctx, logger, dnsZone)
				Expect(err).NotTo(HaveOccurred())

				var privateHostedZoneResponse *route53.ListHostedZonesByNameOutput
				Eventually(func() (string, error) {
					privateHostedZoneResponse, err = rawRoute53Client.ListHostedZonesByNameWithContext(ctx, &route53.ListHostedZonesByNameInput{
						DNSName:  awssdk.String("aprivate.test.example.com"),
						MaxItems: awssdk.String("1"),
					})
					if len(privateHostedZoneResponse.HostedZones) > 0 {
						return *privateHostedZoneResponse.HostedZones[0].Name, err
					}
					return "no_what_we_are_looking_for", nil
				}, "3s", "100ms").Should(Equal("aprivate.test.example.com."))

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
					Name:         awssdk.String("aprivate.test.example.com."),
					Owner: &route53.HostedZoneOwner{
						OwningAccount: awssdk.String("000000000000"),
					},
				}))
			})

			When("the hosted zone already exists", func() {
				BeforeEach(func() {
					now := time.Now()
					_, err = rawRoute53Client.CreateHostedZone(&route53.CreateHostedZoneInput{
						CallerReference: awssdk.String(fmt.Sprintf("1%d", now.UnixNano())),
						HostedZoneConfig: &route53.HostedZoneConfig{
							Comment:     awssdk.String("Zone for CAPI cluster"),
							PrivateZone: awssdk.Bool(true),
						},
						Name: awssdk.String("already.private.exists.test.example.com"),
						VPC: &route53.VPC{
							VPCId:     awssdk.String(VPCId),
							VPCRegion: awssdk.String(Region),
						},
					})
					Expect(err).NotTo(HaveOccurred())
				})

				It("doesn't return error", func() {
					dnsZone := resolver.BuildPrivateHostedZone("already.private.exists.test.example.com", tags, VPCId, Region, []string{MCVPCId})
					hostedZoneId, err = route53Client.CreateHostedZone(ctx, logger, dnsZone)
					Expect(err).NotTo(HaveOccurred())

					hostedZoneResponse, err := rawRoute53Client.ListHostedZonesByNameWithContext(ctx, &route53.ListHostedZonesByNameInput{
						DNSName:  awssdk.String("already.private.exists.test.example.com"),
						MaxItems: awssdk.String("1"),
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(*hostedZoneResponse.HostedZones[0].Name).To(Equal("already.private.exists.test.example.com."))
				})
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

			By("looking for a non existing zone, we expect an error")
			_, err = route53Client.GetHostedZoneIdByName(ctx, logger, "nonexisting.test.example.com.")
			Expect(err).To(HaveOccurred())
			Expect(err).Should(MatchError(&resolver.HostedZoneNotFoundError{}))
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

			err = route53Client.AddDelegationToParentZone(ctx, logger, *parentHostedZoneToFind.HostedZone.Id, listRecordSets.ResourceRecordSets[0])
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

			listRecordSets, err := rawRoute53Client.ListResourceRecordSets(&route53.ListResourceRecordSetsInput{
				HostedZoneId: hostedZoneToFind.HostedZone.Id,
				MaxItems:     awssdk.String("1"), // First entry is always NS record
			})
			Expect(err).NotTo(HaveOccurred())

			// We add the delegation so we can delete it later.
			err = route53Client.AddDelegationToParentZone(ctx, logger, *parentHostedZoneToFind.HostedZone.Id, listRecordSets.ResourceRecordSets[0])
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

	When("deleting a hosted zone", func() {
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
			}, "3s", "100ms").Should(BeZero())
		})
	})

	When("creating dns records", func() {
		var hostedZoneToFind *route53.CreateHostedZoneOutput

		BeforeEach(func() {
			now := time.Now()

			hostedZoneToFind, err = rawRoute53Client.CreateHostedZone(&route53.CreateHostedZoneInput{
				CallerReference: awssdk.String(fmt.Sprintf("1%d", now.UnixNano())),
				Name:            awssdk.String("dnsrecords.example.com"),
			})
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			foundDnsRecordsResponse, err := rawRoute53Client.ListResourceRecordSetsWithContext(ctx, &route53.ListResourceRecordSetsInput{
				HostedZoneId: hostedZoneToFind.HostedZone.Id,
			})
			Expect(err).NotTo(HaveOccurred())

			for _, recordSet := range foundDnsRecordsResponse.ResourceRecordSets {
				// We don't need to remove NS or SOA records.
				if *recordSet.Type == "CNAME" || *recordSet.Type == "A" {
					_, err = rawRoute53Client.ChangeResourceRecordSetsWithContext(ctx, &route53.ChangeResourceRecordSetsInput{
						ChangeBatch: &route53.ChangeBatch{
							Changes: []*route53.Change{
								{
									Action:            awssdk.String("DELETE"),
									ResourceRecordSet: recordSet,
								},
							},
						},
						HostedZoneId: hostedZoneToFind.HostedZone.Id,
					})
					Expect(err).NotTo(HaveOccurred())
				}
			}

			_, err = rawRoute53Client.DeleteHostedZoneWithContext(ctx, &route53.DeleteHostedZoneInput{Id: hostedZoneToFind.HostedZone.Id})
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates the records", func() {
			dnsRecordsToCreate := []resolver.DNSRecord{
				{
					Kind:  "CNAME",
					Name:  "a.dnsrecords.example.com",
					Value: "something",
				},
				{
					Kind:   "ALIAS",
					Name:   "z.dnsrecords.example.com",
					Value:  "alias-value",
					Region: "eu-central-1",
				},
			}
			err = route53Client.AddDnsRecordsToHostedZone(ctx, logger, *hostedZoneToFind.HostedZone.Id, dnsRecordsToCreate)
			Expect(err).NotTo(HaveOccurred())

			foundDnsRecordsResponse, err := rawRoute53Client.ListResourceRecordSetsWithContext(ctx, &route53.ListResourceRecordSetsInput{
				HostedZoneId: hostedZoneToFind.HostedZone.Id,
			})
			Expect(err).NotTo(HaveOccurred())

			cnameFound := false
			aliasFound := false
			for _, recordSet := range foundDnsRecordsResponse.ResourceRecordSets {
				if *recordSet.Name == "a.dnsrecords.example.com." && *recordSet.ResourceRecords[0].Value == "something" {
					cnameFound = true
				}
				if *recordSet.Name == "z.dnsrecords.example.com." && *recordSet.AliasTarget.DNSName == "alias-value" {
					aliasFound = true
				}
			}
			Expect(cnameFound).To(BeTrue())
			Expect(aliasFound).To(BeTrue())

			By("creating the exact same records again, it doesn't fail", func() {
				dnsRecordsToCreate := []resolver.DNSRecord{
					{
						Kind:  "CNAME",
						Name:  "a.dnsrecords.example.com",
						Value: "something",
					},
					{
						Kind:   "ALIAS",
						Name:   "z.dnsrecords.example.com",
						Value:  "alias-value",
						Region: "eu-central-1",
					},
				}
				err = route53Client.AddDnsRecordsToHostedZone(ctx, logger, *hostedZoneToFind.HostedZone.Id, dnsRecordsToCreate)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
	When("deleting all dns records from hosted zone", func() {
		var hostedZoneToFind *route53.CreateHostedZoneOutput

		BeforeEach(func() {
			now := time.Now()

			hostedZoneToFind, err = rawRoute53Client.CreateHostedZone(&route53.CreateHostedZoneInput{
				CallerReference: awssdk.String(fmt.Sprintf("1%d", now.UnixNano())),
				Name:            awssdk.String("dnsrecordstodelete.example.com"),
			})
			Expect(err).NotTo(HaveOccurred())

			dnsRecordsToCreate := []resolver.DNSRecord{
				{
					Kind:  "CNAME",
					Name:  "a.dnsrecordstodelete.example.com",
					Value: "something",
				},
				{
					Kind:   "ALIAS",
					Name:   "z.dnsrecordstodelete.example.com",
					Value:  "alias-value",
					Region: "eu-central-1",
				},
			}
			err = route53Client.AddDnsRecordsToHostedZone(ctx, logger, *hostedZoneToFind.HostedZone.Id, dnsRecordsToCreate)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			foundDnsRecordsResponse, err := rawRoute53Client.ListResourceRecordSetsWithContext(ctx, &route53.ListResourceRecordSetsInput{
				HostedZoneId: hostedZoneToFind.HostedZone.Id,
			})
			Expect(err).NotTo(HaveOccurred())

			for _, recordSet := range foundDnsRecordsResponse.ResourceRecordSets {
				// We don't need to remove NS or SOA records.
				if *recordSet.Type == "CNAME" || *recordSet.Type == "A" {
					_, err = rawRoute53Client.ChangeResourceRecordSetsWithContext(ctx, &route53.ChangeResourceRecordSetsInput{
						ChangeBatch: &route53.ChangeBatch{
							Changes: []*route53.Change{
								{
									Action:            awssdk.String("DELETE"),
									ResourceRecordSet: recordSet,
								},
							},
						},
						HostedZoneId: hostedZoneToFind.HostedZone.Id,
					})
					Expect(err).NotTo(HaveOccurred())
				}
			}

			_, err = rawRoute53Client.DeleteHostedZoneWithContext(ctx, &route53.DeleteHostedZoneInput{Id: hostedZoneToFind.HostedZone.Id})
			Expect(err).NotTo(HaveOccurred())
		})

		It("removes all dns records", func() {
			err = route53Client.DeleteDnsRecordsFromHostedZone(ctx, logger, *hostedZoneToFind.HostedZone.Id)
			Expect(err).NotTo(HaveOccurred())

			foundDnsRecordsResponse, err := rawRoute53Client.ListResourceRecordSetsWithContext(ctx, &route53.ListResourceRecordSetsInput{
				HostedZoneId: hostedZoneToFind.HostedZone.Id,
			})
			Expect(err).NotTo(HaveOccurred())
			// After removing the cluster dns records we expect 2 dns records: the NS and SOA records.
			Expect(len(foundDnsRecordsResponse.ResourceRecordSets)).Should(BeNumerically("<=", 2))
		})
	})
})
