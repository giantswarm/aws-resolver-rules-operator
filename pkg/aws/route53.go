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
	"github.com/pkg/errors"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
)

type Route53 struct {
	client *route53.Route53
}

func (r *Route53) CreatePublicHostedZone(ctx context.Context, logger logr.Logger, zoneName string, tags map[string]string) (string, error) {
	hostedZoneId, err := r.GetHostedZoneIdByName(ctx, logger, zoneName)
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
			Name: awssdk.String(zoneName),
		}

		createdHostedZone, err := r.client.CreateHostedZoneWithContext(ctx, createHostedZoneInput)
		if err != nil {
			return "", errors.WithStack(err)
		}
		hostedZoneId = *createdHostedZone.HostedZone.Id
	}

	err = r.tagHostedZone(ctx, hostedZoneId, tags)
	if err != nil {
		return "", errors.WithStack(err)
	}

	return hostedZoneId, nil
}

func (r *Route53) CreatePrivateHostedZone(ctx context.Context, logger logr.Logger, zoneName, vpcId, region string, tags map[string]string, vpcsToAssociate []string) (string, error) {
	if vpcId == "" {
		return "", errors.New("vpcId can't be empty when creating private hosted zones")
	}
	if region == "" {
		return "", errors.New("region can't be empty when creating private hosted zones")
	}

	hostedZoneId, err := r.GetHostedZoneIdByName(ctx, logger, zoneName)
	if err != nil && !errors.Is(err, &resolver.HostedZoneNotFoundError{}) {
		return "", errors.WithStack(err)
	}

	// We didn't find the hosted zone, so we need to create it.
	if err != nil {
		now := time.Now()
		createHostedZoneInput := &route53.CreateHostedZoneInput{
			CallerReference: awssdk.String(fmt.Sprintf("%d", now.UnixNano())),
			HostedZoneConfig: &route53.HostedZoneConfig{
				Comment:     awssdk.String("Zone for CAPI cluster"),
				PrivateZone: awssdk.Bool(true),
			},
			Name: awssdk.String(zoneName),
			VPC: &route53.VPC{
				VPCId:     awssdk.String(vpcId),
				VPCRegion: awssdk.String(region),
			},
		}

		createdHostedZone, err := r.client.CreateHostedZoneWithContext(ctx, createHostedZoneInput)
		if err != nil {
			return "", errors.WithStack(err)
		}
		hostedZoneId = *createdHostedZone.HostedZone.Id
	}

	logger.Info("Associating hosted zone with VPCs", "hostedZoneId", hostedZoneId, "vpcsToAssociate", vpcsToAssociate)

	err = r.associateHostedZoneWithAdditionalVPCs(ctx, hostedZoneId, region, vpcsToAssociate)
	if err != nil {
		return "", errors.WithStack(err)
	}

	err = r.tagHostedZone(ctx, hostedZoneId, tags)
	if err != nil {
		return "", errors.WithStack(err)
	}

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

func (r *Route53) associateHostedZoneWithAdditionalVPCs(ctx context.Context, hostedZoneId, region string, vpcsToAssociate []string) error {
	var err error
	for _, vpcIdToAssociate := range vpcsToAssociate {
		_, err = r.client.AssociateVPCWithHostedZoneWithContext(ctx, &route53.AssociateVPCWithHostedZoneInput{
			HostedZoneId: awssdk.String(hostedZoneId),
			VPC: &route53.VPC{
				VPCId:     awssdk.String(vpcIdToAssociate),
				VPCRegion: awssdk.String(region),
			},
		})
	}
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (r *Route53) DeleteHostedZone(ctx context.Context, logger logr.Logger, zoneId string) error {
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

	return nil
}

func (r *Route53) GetHostedZoneIdByName(ctx context.Context, logger logr.Logger, zoneName string) (string, error) {
	listResponse, err := r.client.ListHostedZonesByNameWithContext(ctx, &route53.ListHostedZonesByNameInput{
		DNSName:  awssdk.String(zoneName),
		MaxItems: awssdk.String("1"),
	})
	if err != nil {
		return "", errors.WithStack(err)
	}

	if len(listResponse.HostedZones) < 1 {
		return "", &resolver.HostedZoneNotFoundError{}
	}

	if *listResponse.HostedZones[0].Name != fmt.Sprintf("%s.", strings.TrimSuffix(zoneName, ".")) {
		return "", &resolver.HostedZoneNotFoundError{}
	}

	return *listResponse.HostedZones[0].Id, nil
}

// AddDelegationToParentZone adds a NS record (or updates if it already exists) to the parent hosted zone with the NS records of the subdomain.
func (r *Route53) AddDelegationToParentZone(ctx context.Context, logger logr.Logger, parentZoneId, zoneId string) error {
	listResourceRecordSetsOutput, err := r.client.ListResourceRecordSetsWithContext(ctx, &route53.ListResourceRecordSetsInput{
		HostedZoneId: awssdk.String(zoneId),
		MaxItems:     awssdk.String("1"), // First entry is always NS record
	})
	if err != nil {
		return err
	}

	_, err = r.client.ChangeResourceRecordSetsWithContext(ctx, &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: awssdk.String(parentZoneId),
		ChangeBatch: &route53.ChangeBatch{
			Changes: []*route53.Change{
				{
					Action: awssdk.String("UPSERT"),
					ResourceRecordSet: &route53.ResourceRecordSet{
						Name:            listResourceRecordSetsOutput.ResourceRecordSets[0].Name,
						Type:            awssdk.String("NS"),
						TTL:             awssdk.Int64(300),
						ResourceRecords: listResourceRecordSetsOutput.ResourceRecordSets[0].ResourceRecords,
					},
				},
			},
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func (r *Route53) DeleteDelegationFromParentZone(ctx context.Context, logger logr.Logger, parentZoneId, zoneId string) error {
	listResourceRecordSetsOutput, err := r.client.ListResourceRecordSetsWithContext(ctx, &route53.ListResourceRecordSetsInput{
		HostedZoneId: awssdk.String(zoneId),
		MaxItems:     awssdk.String("1"), // First entry is always NS record
	})
	if err != nil {
		return err
	}

	_, err = r.client.ChangeResourceRecordSetsWithContext(ctx, &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: awssdk.String(parentZoneId),
		ChangeBatch: &route53.ChangeBatch{
			Changes: []*route53.Change{
				{
					Action: awssdk.String("DELETE"),
					ResourceRecordSet: &route53.ResourceRecordSet{
						Name:            listResourceRecordSetsOutput.ResourceRecordSets[0].Name,
						Type:            awssdk.String("NS"),
						TTL:             awssdk.Int64(300),
						ResourceRecords: listResourceRecordSetsOutput.ResourceRecordSets[0].ResourceRecords,
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
