package resolver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	gocache "github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
)

type Zoner struct {
	// awsClients is a factory to retrieve clients to talk to the AWS API using the right credentials.
	awsClients AWSClients

	// Some client objects such as `Route53` contain a cache, so we should reuse the client object
	// (note that we have separate logic to reuse AWS SDK sessions, see `sessionCache`)
	awsClientCache *gocache.Cache

	// workloadClusterBaseDomain is the root hosted zone used to create the workload cluster hosted zone, i.e. gaws.gigantic.io
	workloadClusterBaseDomain string
}

func NewDnsZone(awsClients AWSClients, workloadClusterBaseDomain string) (Zoner, error) {
	return Zoner{
		awsClients:                awsClients,
		awsClientCache:            gocache.New(15*time.Minute, 60*time.Second),
		workloadClusterBaseDomain: workloadClusterBaseDomain,
	}, nil
}

func (d *Zoner) getCachedOrNewRoute53Client(region, arn string, logger logr.Logger) (Route53Client, error) {
	// Never cache for invalid values. Instead, we pass through to the real getter in order to return an error.
	if region == "" || arn == "" {
		return d.awsClients.NewRoute53Client(region, arn)
	}

	cacheKey := fmt.Sprintf("route53/%s/%s", region, arn)
	if cachedValue, ok := d.awsClientCache.Get(cacheKey); ok {
		logger.Info("Using Route53 client from cache", "region", region, "arn", arn)
		return cachedValue.(Route53Client), nil
	}

	logger.Info("Using new Route53 client", "region", region, "arn", arn)
	client, err := d.awsClients.NewRoute53Client(region, arn)
	if err != nil {
		return nil, err
	}
	d.awsClientCache.SetDefault(cacheKey, client)
	return client, err
}

func (d *Zoner) CreateHostedZone(ctx context.Context, logger logr.Logger, cluster Cluster) error {
	route53Client, err := d.getCachedOrNewRoute53Client(cluster.Region, cluster.IAMRoleARN, logger)
	if err != nil {
		return errors.WithStack(err)
	}

	hostedZoneName := d.getHostedZoneName(cluster)
	logger = logger.WithValues("hostedZoneName", hostedZoneName)

	var dnsZoneToCreate DnsZone
	if cluster.IsDnsModePrivate {
		dnsZoneToCreate = BuildPrivateHostedZone(hostedZoneName, d.getTagsForHostedZone(cluster), cluster.VPCId, cluster.Region, cluster.VPCsToAssociateToHostedZone)
	} else {
		dnsZoneToCreate = BuildPublicHostedZone(hostedZoneName, d.getTagsForHostedZone(cluster))
	}

	hostedZoneId, err := route53Client.CreateHostedZone(ctx, logger, dnsZoneToCreate)
	if err != nil {
		return errors.WithStack(err)
	}
	logger = logger.WithValues("hostedZoneId", hostedZoneId)

	err = d.createDnsRecords(ctx, logger, cluster, hostedZoneId)
	if err != nil {
		return errors.WithStack(err)
	}

	mcRoute53Client, err := d.getCachedOrNewRoute53Client(cluster.Region, cluster.MCIAMRoleARN, logger)
	if err != nil {
		return errors.WithStack(err)
	}

	if !cluster.IsDnsModePrivate {
		parentHostedZoneId, err := mcRoute53Client.GetHostedZoneIdByName(ctx, logger, d.getParentHostedZoneName())
		if err != nil {
			return errors.WithStack(err)
		}

		nsRecord, err := route53Client.GetHostedZoneNSRecords(ctx, hostedZoneId)
		if err != nil {
			return errors.WithStack(err)
		}

		err = mcRoute53Client.AddDelegationToParentZone(ctx, logger, parentHostedZoneId, nsRecord)
		if err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

func (d *Zoner) DeleteHostedZone(ctx context.Context, logger logr.Logger, cluster Cluster) error {
	route53Client, err := d.getCachedOrNewRoute53Client(cluster.Region, cluster.IAMRoleARN, logger)
	if err != nil {
		return errors.WithStack(err)
	}
	hostedZoneName := d.getHostedZoneName(cluster)
	logger = logger.WithValues("hostedZoneName", hostedZoneName)
	hostedZoneId, err := route53Client.GetHostedZoneIdByName(ctx, logger, hostedZoneName)
	if err != nil {
		if errors.Is(err, &HostedZoneNotFoundError{}) {
			logger.Info("Hosted zone can't be found, skipping deletion", "hostedZoneId", hostedZoneId, "hostedZoneName", hostedZoneName)
			return nil
		}

		return errors.WithStack(err)
	}
	logger = logger.WithValues("hostedZoneId", hostedZoneId)

	if !cluster.IsDnsModePrivate {
		mcRoute53Client, err := d.getCachedOrNewRoute53Client(cluster.Region, cluster.MCIAMRoleARN, logger)
		if err != nil {
			return errors.WithStack(err)
		}
		parentHostedZoneId, err := mcRoute53Client.GetHostedZoneIdByName(ctx, logger, d.getParentHostedZoneName())
		if err != nil {
			return errors.WithStack(err)
		}

		nsRecord, err := route53Client.GetHostedZoneNSRecords(ctx, hostedZoneId)
		if err != nil {
			return errors.WithStack(err)
		}

		logger.Info("Deleting delegation from parent hosted zone", "parentHostedZoneId", parentHostedZoneId)
		err = mcRoute53Client.DeleteDelegationFromParentZone(ctx, logger, parentHostedZoneId, nsRecord)
		if err != nil {
			return errors.WithStack(err)
		}
	}

	err = route53Client.DeleteDnsRecordsFromHostedZone(ctx, logger, hostedZoneId)
	if err != nil {
		return errors.WithStack(err)
	}

	err = route53Client.DeleteHostedZone(ctx, logger, hostedZoneId)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (d *Zoner) createDnsRecords(ctx context.Context, logger logr.Logger, cluster Cluster, hostedZoneId string) error {
	route53Client, err := d.getCachedOrNewRoute53Client(cluster.Region, cluster.IAMRoleARN, logger)
	if err != nil {
		return errors.WithStack(err)
	}

	dnsRecordsToCreate := d.getWorkloadClusterDnsRecords(d.getHostedZoneName(cluster), cluster)
	err = route53Client.AddDnsRecordsToHostedZone(ctx, logger, hostedZoneId, dnsRecordsToCreate)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (d *Zoner) getHostedZoneName(cluster Cluster) string {
	return fmt.Sprintf("%s.%s", cluster.Name, d.workloadClusterBaseDomain)
}

func (d *Zoner) getParentHostedZoneName() string {
	return d.workloadClusterBaseDomain
}

func (d *Zoner) getTagsForHostedZone(cluster Cluster) map[string]string {
	var tags = map[string]string{
		"Name": cluster.Name,
		fmt.Sprintf("sigs.k8s.io/cluster-api-provider-aws/cluster/%s", cluster.Name): "owned",
		"sigs.k8s.io/cluster-api-provider-aws/role":                                  "common",
	}

	for key, value := range cluster.AdditionalTags {
		tags[key] = value
	}

	return tags
}

func (d *Zoner) getWorkloadClusterDnsRecords(workloadClusterHostedZoneName string, cluster Cluster) []DNSRecord {
	dnsRecords := []DNSRecord{
		{
			Kind:   DnsRecordTypeCname,
			Name:   fmt.Sprintf("*.%s", workloadClusterHostedZoneName),
			Values: []string{fmt.Sprintf("ingress.%s", workloadClusterHostedZoneName)},
		},
	}

	if cluster.ControlPlaneEndpoint != "" {
		if cluster.IsEKS {
			dnsRecords = append(dnsRecords, DNSRecord{
				Kind:   DnsRecordTypeCname,
				Name:   fmt.Sprintf("api.%s", workloadClusterHostedZoneName),
				Values: []string{strings.TrimPrefix(cluster.ControlPlaneEndpoint, "https://")},
				Region: cluster.Region,
			})
		} else {
			dnsRecords = append(dnsRecords, DNSRecord{
				Kind:   DnsRecordTypeAlias,
				Name:   fmt.Sprintf("api.%s", workloadClusterHostedZoneName),
				Values: []string{cluster.ControlPlaneEndpoint},
				Region: cluster.Region,
			})
		}
	}

	return dnsRecords
}
