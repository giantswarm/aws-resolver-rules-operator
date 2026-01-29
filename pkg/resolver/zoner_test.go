package resolver

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestZoner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Zoner Suite")
}

var _ = Describe("Zoner", func() {
	const workloadClusterBaseDomain = "test.gigantic.io"

	var (
		zoner Zoner
	)

	BeforeEach(func() {
		fakeAWSClients := &FakeClients{}
		var err error
		zoner, err = NewDnsZone(fakeAWSClients, workloadClusterBaseDomain)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("getHostedZoneName", func() {
		When("no custom hosted zone name is set", func() {
			It("returns the default hosted zone name", func() {
				cluster := Cluster{
					Name: "my-cluster",
				}
				result := zoner.getHostedZoneName(cluster)
				Expect(result).To(Equal("my-cluster.test.gigantic.io"))
			})
		})

		When("a custom hosted zone name is set", func() {
			It("returns the custom hosted zone name", func() {
				cluster := Cluster{
					Name:                 "my-cluster",
					CustomHostedZoneName: "custom.other.domain.com",
				}
				result := zoner.getHostedZoneName(cluster)
				Expect(result).To(Equal("custom.other.domain.com"))
			})
		})
	})

	Describe("getParentHostedZoneName", func() {
		When("no custom hosted zone name is set", func() {
			It("returns the workload cluster base domain", func() {
				cluster := Cluster{
					Name: "my-cluster",
				}
				result := zoner.getParentHostedZoneName(cluster)
				Expect(result).To(Equal("test.gigantic.io"))
			})
		})

		When("a custom hosted zone name with valid parent is set", func() {
			It("derives the parent from the custom hosted zone name", func() {
				cluster := Cluster{
					Name:                 "my-cluster",
					CustomHostedZoneName: "foo.bar.com",
				}
				result := zoner.getParentHostedZoneName(cluster)
				Expect(result).To(Equal("bar.com"))
			})
		})

		When("a custom hosted zone name with deeper nesting is set", func() {
			It("derives the parent correctly", func() {
				cluster := Cluster{
					Name:                 "my-cluster",
					CustomHostedZoneName: "deep.nested.sub.domain.com",
				}
				result := zoner.getParentHostedZoneName(cluster)
				Expect(result).To(Equal("nested.sub.domain.com"))
			})
		})

		When("a custom hosted zone name without valid parent is set (single label)", func() {
			It("returns empty string", func() {
				cluster := Cluster{
					Name:                 "my-cluster",
					CustomHostedZoneName: "invalid",
				}
				result := zoner.getParentHostedZoneName(cluster)
				Expect(result).To(Equal(""))
			})
		})

		When("a custom hosted zone name with only two labels is set", func() {
			It("returns empty string because parent would be TLD only", func() {
				cluster := Cluster{
					Name:                 "my-cluster",
					CustomHostedZoneName: "foo.com",
				}
				result := zoner.getParentHostedZoneName(cluster)
				Expect(result).To(Equal(""))
			})
		})
	})

	Describe("getDelegationRoleARN", func() {
		When("no custom delegation role ARN is set", func() {
			It("returns the MC IAM role ARN", func() {
				cluster := Cluster{
					MCIAMRoleARN: "arn:aws:iam::111111111111:role/MCRole",
				}
				result := zoner.getDelegationRoleARN(cluster)
				Expect(result).To(Equal("arn:aws:iam::111111111111:role/MCRole"))
			})
		})

		When("a custom delegation role ARN is set", func() {
			It("returns the custom delegation role ARN", func() {
				cluster := Cluster{
					MCIAMRoleARN:         "arn:aws:iam::111111111111:role/MCRole",
					DelegationIAMRoleARN: "arn:aws:iam::222222222222:role/CustomRole",
				}
				result := zoner.getDelegationRoleARN(cluster)
				Expect(result).To(Equal("arn:aws:iam::222222222222:role/CustomRole"))
			})
		})

		When("both delegation role ARN and MC IAM role ARN are empty", func() {
			It("returns empty string", func() {
				cluster := Cluster{}
				result := zoner.getDelegationRoleARN(cluster)
				Expect(result).To(Equal(""))
			})
		})
	})
})
