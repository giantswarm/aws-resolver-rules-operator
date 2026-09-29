package aws

import (
	"context"
	"io"
	"net/http"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
)

type fakeHTTPClient func(request *http.Request) *http.Response

func (f fakeHTTPClient) Do(request *http.Request) (*http.Response, error) {
	return f(request), nil
}

var _ = Describe("Route53", func() {
	When("deleting a delegation that is already gone from the parent zone", func() {
		var changeRequestCount int
		var route53Client *Route53

		BeforeEach(func() {
			changeRequestCount = 0

			httpClient := fakeHTTPClient(func(request *http.Request) *http.Response {
				response := &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"text/xml"}},
					Request:    request,
				}

				if request.Method == http.MethodGet {
					response.Body = io.NopCloser(strings.NewReader(`<?xml version="1.0" encoding="UTF-8"?>
<ListResourceRecordSetsResponse xmlns="https://route53.amazonaws.com/doc/2013-04-01/">
  <ResourceRecordSets>
    <ResourceRecordSet>
      <Name>other.test.example.com.</Name>
      <Type>NS</Type>
      <TTL>300</TTL>
      <ResourceRecords><ResourceRecord><Value>ns-1.example.com.</Value></ResourceRecord></ResourceRecords>
    </ResourceRecordSet>
  </ResourceRecordSets>
  <IsTruncated>false</IsTruncated>
  <MaxItems>1</MaxItems>
</ListResourceRecordSetsResponse>`))
					return response
				}

				changeRequestCount++
				response.StatusCode = http.StatusBadRequest
				response.Body = io.NopCloser(strings.NewReader(`<?xml version="1.0" encoding="UTF-8"?>
<InvalidChangeBatch xmlns="https://route53.amazonaws.com/doc/2013-04-01/">
  <Messages><Message>Tried to delete resource record set [name='cluster.test.example.com.', type='NS'] but it was not found</Message></Messages>
</InvalidChangeBatch>`))
				return response
			})

			route53Client = NewRoute53(route53.New(route53.Options{
				BaseEndpoint: awssdk.String("https://route53.example.com"),
				Credentials:  awssdk.AnonymousCredentials{},
				HTTPClient:   httpClient,
				Region:       "us-east-1",
			}))
		})

		It("succeeds without requesting a change", func() {
			err := route53Client.DeleteDelegationFromParentZone(context.Background(), logr.Discard(), "parent-zone-id", &resolver.DNSRecord{
				Name:   "cluster.test.example.com",
				Kind:   resolver.DnsRecordType("NS"),
				Values: []string{"ns-1.example.com."},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(changeRequestCount).To(BeZero())
		})
	})
})
