package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/go-logr/logr"
	gocache "github.com/patrickmn/go-cache"
	"github.com/pkg/errors"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
)

var (
	// see: https://docs.aws.amazon.com/general/latest/gr/elb.html
	canonicalHostedZones = map[string]string{
		// Application Load Balancers and Classic Load Balancers
		"us-east-2":      "Z3AADJGX6KTTL2",
		"us-east-1":      "Z35SXDOTRQ7X7K",
		"us-west-1":      "Z368ELLRRE2KJ0",
		"us-west-2":      "Z1H1FL5HABSF5",
		"ca-central-1":   "ZQSVJUPU6J1EY",
		"ap-east-1":      "Z3DQVH9N71FHZ0",
		"ap-south-1":     "ZP97RAFLXTNZK",
		"ap-northeast-2": "ZWKZPGTI48KDX",
		"ap-northeast-3": "Z5LXEXXYW11ES",
		"ap-southeast-1": "Z1LMS91P8CMLE5",
		"ap-southeast-2": "Z1GM3OXH4ZPM65",
		"ap-northeast-1": "Z14GRHDCWA56QT",
		"eu-central-1":   "Z215JYRZR1TBD5",
		"eu-west-1":      "Z32O12XQLNTSW2",
		"eu-west-2":      "ZHURV8PSTC4K8",
		"eu-west-3":      "Z3Q77PNBQS71R4",
		"eu-north-1":     "Z23TAZ6LKFMNIO",
		"eu-south-1":     "Z3ULH7SSC9OV64",
		"sa-east-1":      "Z2P70J7HTTTPLU",
		"cn-north-1":     "Z1GDH35T77C1KE",
		"cn-northwest-1": "ZM7IZAIOVVDZF",
		"us-gov-west-1":  "Z33AYJ8TM3BH4J",
		"us-gov-east-1":  "Z166TLBEWOO7G0",
		"me-south-1":     "ZS929ML54UICD",
		"af-south-1":     "Z268VQBMOI5EKX",
	}
)

type Route53 struct {
	client *route53.Route53

	upsertCache           *gocache.Cache
	zoneIdToNSRecordCache *gocache.Cache
	zoneNameToIdCache     *gocache.Cache
}

func NewRoute53(client *route53.Route53) *Route53 {
	return &Route53{
		client: client,

		// By remembering that we recently upserted certain resource records, we can avoid making excess Route53
		// requests that lead to rate limiting
		upsertCache: gocache.New(120*time.Second, 15*time.Second),

		// Avoid Route53 rate limit by caching zone ID and NS for a short time. This is particularly
		// helpful if the same object gets reconciled multiple times in a row. We don't want
		// to repeat the same AWS API request each time.
		zoneIdToNSRecordCache: gocache.New(300*time.Second, 60*time.Second),
		zoneNameToIdCache:     gocache.New(90*time.Second, 60*time.Second),
	}
}

func (r *Route53) CreateHostedZone(ctx context.Context, logger logr.Logger, dnsZone resolver.DnsZone) (string, error) {
	hostedZoneId, err := r.GetHostedZoneIdByName(ctx, logger, dnsZone.DnsName)
	if err != nil && !errors.Is(err, &resolver.HostedZoneNotFoundError{}) {
		return "", errors.WithStack(err)
	}

	// We didn't find the hosted zone, so we need to create it.
	if err != nil {
		now := time.Now()
		createHostedZoneInput := &route53.CreateHostedZoneInput{
			CallerReference: awssdk.String(fmt.Sprintf("%d", now.UnixNano())),
			HostedZoneConfig: &route53.HostedZoneConfig{
				Comment: awssdk.String("Zone for CAPI cluster"),
			},
			Name: awssdk.String(dnsZone.DnsName),
		}

		if dnsZone.IsPrivate {
			createHostedZoneInput.HostedZoneConfig.PrivateZone = awssdk.Bool(true)
			createHostedZoneInput.VPC = &route53.VPC{
				VPCId:     awssdk.String(dnsZone.VPCId),
				VPCRegion: awssdk.String(dnsZone.Region),
			}
		}

		logger.Info("Creating hosted zone")
		createdHostedZone, err := r.client.CreateHostedZoneWithContext(ctx, createHostedZoneInput)
		if err != nil {
			return "", errors.WithStack(err)
		}
		hostedZoneId = *createdHostedZone.HostedZone.Id
		r.zoneNameToIdCache.SetDefault(dnsZone.DnsName, hostedZoneId)
	}

	logger = logger.WithValues("hostedZoneId", hostedZoneId)

	err = r.tagHostedZone(ctx, hostedZoneId, dnsZone.Tags)
	if err != nil {
		return "", errors.WithStack(err)
	}

	if dnsZone.IsPrivate {
		r.associateHostedZoneWithAdditionalVPCs(ctx, logger, hostedZoneId, dnsZone.Region, dnsZone.VPCsToAssociate)
	}
	logger.Info("Hosted zone created")

	return hostedZoneId, nil
}

func (r *Route53) tagHostedZone(ctx context.Context, hostedZoneId string, tags map[string]string) error {
	if len(tags) == 0 {
		return nil
	}

	var route53Tags []*route53.Tag
	for tagKey, tagValue := range tags {
		route53Tags = append(route53Tags, &route53.Tag{
			Key:   awssdk.String(tagKey),
			Value: awssdk.String(tagValue),
		})
	}
	tagsInput := &route53.ChangeTagsForResourceInput{
		AddTags:      route53Tags,
		ResourceId:   awssdk.String(hostedZoneId),
		ResourceType: awssdk.String("hostedzone"),
	}
	_, err := r.client.ChangeTagsForResourceWithContext(ctx, tagsInput)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// associateHostedZoneWithAdditionalVPCs will associate the hosted zone with all the VPCs passed as parameters.
// If there is an error while associating one of them, it will log the error and continue with the next VPC.
func (r *Route53) associateHostedZoneWithAdditionalVPCs(ctx context.Context, logger logr.Logger, hostedZoneId, region string, vpcsToAssociate []string) {
	logger.Info("Associating hosted zone with VPCs", "vpcsToAssociate", vpcsToAssociate)
	for _, vpcIdToAssociate := range vpcsToAssociate {
		_, err := r.client.AssociateVPCWithHostedZoneWithContext(ctx, &route53.AssociateVPCWithHostedZoneInput{
			HostedZoneId: awssdk.String(hostedZoneId),
			VPC: &route53.VPC{
				VPCId:     awssdk.String(vpcIdToAssociate),
				VPCRegion: awssdk.String(region),
			},
		})
		if err != nil {
			logger.Error(err, "error while associating VPC with hosted zone, skipping association", "vpcid", vpcIdToAssociate, "region", region)
		}
	}
}

func (r *Route53) DeleteHostedZone(ctx context.Context, logger logr.Logger, zoneId string) error {
	logger.Info("Deleting hosted zone")
	for zoneName, item := range r.zoneNameToIdCache.Items() {
		cachedZoneId := item.Object.(string)
		if cachedZoneId == zoneId {
			r.zoneNameToIdCache.Delete(zoneName)
			logger.Info("Removed hosted zone ID from cache")
			break
		}
	}

	_, err := r.client.DeleteHostedZoneWithContext(ctx, &route53.DeleteHostedZoneInput{Id: awssdk.String(zoneId)})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case route53.ErrCodeNoSuchHostedZone:
				return nil
			default:
				return errors.WithStack(err)
			}
		}

		return errors.WithStack(err)
	}
	logger.Info("Hosted zone deleted")

	return nil
}

func (r *Route53) GetHostedZoneNSRecord(ctx context.Context, logger logr.Logger, zoneId string, zoneName string) (*resolver.DNSRecord, error) {
	logger = logger.WithValues("zoneId", zoneId, "zoneName", zoneName)

	var resourceRecordSet *route53.ResourceRecordSet

	if cachedValue, ok := r.zoneIdToNSRecordCache.Get(zoneId); ok {
		logger.Info("Using NS record from cache")

		resourceRecordSet = cachedValue.(*route53.ResourceRecordSet)
	} else {
		logger.Info("Requesting NS record")

		listResourceRecordSetsOutput, err := r.client.ListResourceRecordSetsWithContext(ctx, &route53.ListResourceRecordSetsInput{
			HostedZoneId:    awssdk.String(zoneId),
			MaxItems:        awssdk.String("1"),
			StartRecordType: awssdk.String(route53.RRTypeNs),

			// `StartRecordType` must be specified together with `StartRecordName`, so we specify the zone domain
			StartRecordName: awssdk.String(zoneName),
		})
		if err != nil {
			return nil, errors.WithStack(err)
		}

		if len(listResourceRecordSetsOutput.ResourceRecordSets) != 1 {
			return nil, errors.New("logic error - did not receive exactly one resource record set")
		}
		if *listResourceRecordSetsOutput.ResourceRecordSets[0].Type != route53.RRTypeNs {
			return nil, errors.Errorf(
				"logic error - did not receive a resource record set of type NS (got name %q and type %q)",
				*listResourceRecordSetsOutput.ResourceRecordSets[0].Name,
				*listResourceRecordSetsOutput.ResourceRecordSets[0].Type)
		}
		if len(listResourceRecordSetsOutput.ResourceRecordSets[0].ResourceRecords) == 0 {
			return nil, errors.New("did not receive any NS resource record")
		}

		resourceRecordSet = listResourceRecordSetsOutput.ResourceRecordSets[0]

		r.zoneIdToNSRecordCache.SetDefault(zoneId, resourceRecordSet)
	}

	record := &resolver.DNSRecord{
		Name:   *resourceRecordSet.Name,
		Kind:   resolver.DnsRecordType(*resourceRecordSet.Type),
		Values: []string{},
	}

	for _, resourceRecord := range resourceRecordSet.ResourceRecords {
		record.Values = append(record.Values, *resourceRecord.Value)
	}

	return record, nil
}

func (r *Route53) GetHostedZoneIdByName(ctx context.Context, logger logr.Logger, zoneName string) (string, error) {
	if cachedValue, ok := r.zoneNameToIdCache.Get(zoneName); ok {
		zoneId := cachedValue.(string)
		logger.Info("Using hosted zone ID from cache", "zoneId", zoneId, "zoneName", zoneName)
		return zoneId, nil
	}

	// Request up to the allowed maximum of 100 items. This way, we can get and cache the IDs of other
	// zones as well and thereby avoid making one request per zone name which can easily lead to AWS throttling
	// (Route53: 5 req/sec rate limit!). Mind that `DNSName` acts like an alphabetical start marker, not as equality
	// comparison - if that exact zone name does not exist, AWS may still return other zones!
	//
	// See https://docs.aws.amazon.com/Route53/latest/APIReference/API_ListHostedZonesByName.html.
	listResponse, err := r.client.ListHostedZonesByNameWithContext(ctx, &route53.ListHostedZonesByNameInput{
		DNSName:  awssdk.String(zoneName),
		MaxItems: awssdk.String("100"),
	})
	if err != nil {
		return "", errors.WithStack(err)
	}

	// Validate response (no duplicates expected) and search for our zone name
	var foundZoneId string
	wantedAWSZoneName := strings.TrimSuffix(zoneName, ".") + "."
	for _, hostedZone := range listResponse.HostedZones {
		if *hostedZone.Name == wantedAWSZoneName {
			if foundZoneId != "" {
				// We found this zone name already
				return "", errors.New("found two identical zone names in ListHostedZonesByName response")
			}
			foundZoneId = *hostedZone.Id
		}
	}

	// If the response was considered fully valid above, cache all returned zones
	for _, hostedZone := range listResponse.HostedZones {
		cacheKey := strings.TrimSuffix(*hostedZone.Name, ".")
		r.zoneNameToIdCache.SetDefault(cacheKey, *hostedZone.Id)
	}

	if foundZoneId == "" {
		return "", &resolver.HostedZoneNotFoundError{}
	}

	logger.Info("Found hosted zone ID", "zoneId", foundZoneId, "zoneName", zoneName)
	return foundZoneId, nil
}

// AddDelegationToParentZone adds a NS record (or updates if it already exists) to the parent hosted zone with the NS records of the subdomain.
func (r *Route53) AddDelegationToParentZone(ctx context.Context, logger logr.Logger, parentZoneId string, resourceRecord *resolver.DNSRecord) error {
	logger = logger.WithValues("parentHostedZoneId", parentZoneId, "dnsRecordName", resourceRecord.Name)

	logger.Info("Adding delegation to parent hosted zone")

	if len(resourceRecord.Values) == 0 {
		return errors.New("logic error - no NS resource records")
	}

	var awsResourceRecords []*route53.ResourceRecord
	for _, value := range resourceRecord.Values {
		awsResourceRecords = append(awsResourceRecords, &route53.ResourceRecord{
			Value: awssdk.String(value),
		})
	}

	cacheKey := fmt.Sprintf("parentZoneId=%q/name=%q/values=%q", parentZoneId, resourceRecord.Name, strings.Join(resourceRecord.Values, ","))

	if _, ok := r.upsertCache.Get(cacheKey); ok {
		// Avoid making excess Route53 requests that lead to rate limiting if we recently
		// upserted those exact resource records
		logger.Info("NS record in parent zone was recently upserted, skipping")
		return nil
	}

	_, err := r.client.ChangeResourceRecordSetsWithContext(ctx, &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: awssdk.String(parentZoneId),
		ChangeBatch: &route53.ChangeBatch{
			Changes: []*route53.Change{
				{
					Action: awssdk.String("UPSERT"),
					ResourceRecordSet: &route53.ResourceRecordSet{
						Name:            awssdk.String(resourceRecord.Name),
						Type:            awssdk.String("NS"),
						TTL:             awssdk.Int64(300),
						ResourceRecords: awsResourceRecords,
					},
				},
			},
		},
	})
	if err != nil {
		return errors.WithStack(err)
	}

	r.upsertCache.SetDefault(cacheKey, true)

	logger.Info("Added delegation to parent hosted zone")

	return nil
}

func (r *Route53) AddDnsRecordsToHostedZone(ctx context.Context, logger logr.Logger, hostedZoneId string, dnsRecords []resolver.DNSRecord) error {
	logger = logger.WithValues("dnsRecords", dnsRecords, "zoneId", hostedZoneId)

	logger.Info("Creating DNS records")

	if len(dnsRecords) == 0 {
		return errors.New("logic error - no DNS records given")
	}

	var hash string
	for _, dnsRecord := range dnsRecords {
		hash += fmt.Sprintf("%q,%q,%q", dnsRecord.Kind, dnsRecord.Name, strings.Join(dnsRecord.Values, ","))
	}

	cacheKey := fmt.Sprintf("zoneId=%q/hash=%s", hostedZoneId, hash)

	if _, ok := r.upsertCache.Get(cacheKey); ok {
		// Avoid making excess Route53 requests that lead to rate limiting if we recently
		// upserted those exact resource records
		logger.Info("DNS records were recently upserted, skipping")
		return nil
	}

	_, err := r.client.ChangeResourceRecordSetsWithContext(ctx, &route53.ChangeResourceRecordSetsInput{
		ChangeBatch: &route53.ChangeBatch{
			Changes: getAWSSdkChangesFromDnsRecords(logger, dnsRecords),
		},
		HostedZoneId: awssdk.String(hostedZoneId),
	})
	if err != nil {
		return errors.WithStack(err)
	}

	r.upsertCache.SetDefault(cacheKey, true)

	logger.Info("DNS records created")

	return nil
}

// DeleteDnsRecordsFromHostedZone will delete all dns records from the zone, except for SOA and NS records.
func (r *Route53) DeleteDnsRecordsFromHostedZone(ctx context.Context, logger logr.Logger, hostedZoneId string) error {
	logger.Info("Deleting dns records from hosted zone")
	listRecordsResponse, err := r.client.ListResourceRecordSetsWithContext(ctx, &route53.ListResourceRecordSetsInput{
		HostedZoneId: awssdk.String(hostedZoneId),
	})
	if err != nil {
		return errors.WithStack(err)
	}

	changes := []*route53.Change{}
	for _, recordSet := range listRecordsResponse.ResourceRecordSets {
		if *recordSet.Type == route53.RRTypeSoa || *recordSet.Type == route53.RRTypeNs {
			continue
		}

		changes = append(changes, &route53.Change{
			Action:            awssdk.String("DELETE"),
			ResourceRecordSet: recordSet,
		})
	}

	if len(changes) > 0 {
		_, err = r.client.ChangeResourceRecordSetsWithContext(ctx, &route53.ChangeResourceRecordSetsInput{
			ChangeBatch: &route53.ChangeBatch{
				Changes: changes,
			},
			HostedZoneId: awssdk.String(hostedZoneId),
		})
		if err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

func getAWSSdkChangesFromDnsRecords(logger logr.Logger, dnsRecords []resolver.DNSRecord) []*route53.Change {
	var changes []*route53.Change
	for _, record := range dnsRecords {
		switch record.Kind {
		case resolver.DnsRecordTypeCname, resolver.DnsRecordTypeA:
			changes = append(changes, &route53.Change{
				Action: awssdk.String("UPSERT"),
				ResourceRecordSet: &route53.ResourceRecordSet{
					Name: awssdk.String(record.Name),
					Type: awssdk.String(string(record.Kind)),
					TTL:  awssdk.Int64(300),
					ResourceRecords: []*route53.ResourceRecord{
						{
							Value: awssdk.String(record.Values[0]),
						},
					},
				}})
		case resolver.DnsRecordTypeAlias:
			changes = append(changes, &route53.Change{
				Action: awssdk.String("UPSERT"),
				ResourceRecordSet: &route53.ResourceRecordSet{
					Name: awssdk.String(record.Name),
					Type: awssdk.String("A"),
					AliasTarget: &route53.AliasTarget{
						DNSName:              awssdk.String(record.Values[0]),
						EvaluateTargetHealth: awssdk.Bool(false),
						HostedZoneId:         awssdk.String(canonicalHostedZones[record.Region]),
					},
				},
			})
		default:
			logger.Info("dns record type not supported, skipping", "dnsRecord", record)
		}
	}

	return changes
}

func (r *Route53) DeleteDelegationFromParentZone(ctx context.Context, logger logr.Logger, parentZoneId string, resourceRecord *resolver.DNSRecord) error {
	var awsResourceRecords []*route53.ResourceRecord
	for _, value := range resourceRecord.Values {
		awsResourceRecords = append(awsResourceRecords, &route53.ResourceRecord{
			Value: awssdk.String(value),
		})
	}

	_, err := r.client.ChangeResourceRecordSetsWithContext(ctx, &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: awssdk.String(parentZoneId),
		ChangeBatch: &route53.ChangeBatch{
			Changes: []*route53.Change{
				{
					Action: awssdk.String("DELETE"),
					ResourceRecordSet: &route53.ResourceRecordSet{
						Name:            awssdk.String(resourceRecord.Name),
						Type:            awssdk.String("NS"),
						TTL:             awssdk.Int64(300),
						ResourceRecords: awsResourceRecords,
					},
				},
			},
		},
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case route53.ErrCodeInvalidChangeBatch:
				return nil
			default:
				return errors.WithStack(err)
			}
		}

		return errors.WithStack(err)
	}

	return nil
}
