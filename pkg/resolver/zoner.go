package resolver

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

type Zoner struct {
	// awsClients is a factory to retrieve clients to talk to the AWS API using the right credentials.
	awsClients AWSClients
	// workloadClusterBaseDomain is the root hosted zone used to create the workload cluster hosted zone, i.e. gaws.gigantic.io
	workloadClusterBaseDomain string
}

func NewDnsZone(awsClients AWSClients, workloadClusterBaseDomain string) (Zoner, error) {
	return Zoner{
		awsClients:                awsClients,
		workloadClusterBaseDomain: workloadClusterBaseDomain,
	}, nil
}

func (d *Zoner) CreateHostedZone(ctx context.Context, logger logr.Logger, cluster Cluster) error {
	route53Client, err := d.awsClients.NewRoute53Client(cluster.Region, cluster.IAMRoleARN)
	if err != nil {
		return errors.WithStack(err)
	}

	hostedZoneName := d.getHostedZoneName(cluster)
	logger = logger.WithValues("hostedZoneName", hostedZoneName)

	dnsZoneToCreate := BuildPublicHostedZone(hostedZoneName, d.getTagsForHostedZone(cluster))
	if cluster.IsDnsModePrivate {
		dnsZoneToCreate = BuildPrivateHostedZone(hostedZoneName, d.getTagsForHostedZone(cluster), cluster.VPCId, cluster.Region, cluster.VPCsToAssociateToHostedZone)
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

	mcRoute53Client, err := d.awsClients.NewRoute53Client(cluster.Region, cluster.MCIAMRoleARN)
	if err != nil {
		return errors.WithStack(err)
	}

	if !cluster.IsDnsModePrivate {
		parentHostedZoneId, err := mcRoute53Client.GetHostedZoneIdByName(ctx, logger, d.getParentHostedZoneName())
		if err != nil {
			return errors.WithStack(err)
		}

		err = mcRoute53Client.AddDelegationToParentZone(ctx, logger, parentHostedZoneId, hostedZoneId)
		if err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

func (d *Zoner) DeleteHostedZone(ctx context.Context, logger logr.Logger, cluster Cluster) error {
	route53Client, err := d.awsClients.NewRoute53Client(cluster.Region, cluster.IAMRoleARN)
	if err != nil {
		return errors.WithStack(err)
	}
	hostedZoneName := d.getHostedZoneName(cluster)
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
		parentHostedZoneId, err := route53Client.GetHostedZoneIdByName(ctx, logger, d.getParentHostedZoneName())
		if err != nil {
			return errors.WithStack(err)
		}

		logger.Info("Deleting delegation from parent hosted zone", "parentHostedZoneId", parentHostedZoneId)
		err = route53Client.DeleteDelegationFromParentZone(ctx, logger, parentHostedZoneId, hostedZoneId)
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
	route53Client, err := d.awsClients.NewRoute53Client(cluster.Region, cluster.IAMRoleARN)
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
	return map[string]string{
		"Name": cluster.Name,
		fmt.Sprintf("sigs.k8s.io/cluster-api-provider-aws/cluster/%s", cluster.Name): "owned",
		"sigs.k8s.io/cluster-api-provider-aws/role":                                  "common",
	}
}

func (d *Zoner) getWorkloadClusterDnsRecords(workloadClusterHostedZoneName string, cluster Cluster) []DNSRecord {
	dnsRecords := []DNSRecord{
		{
			Kind:  DnsRecordTypeCname,
			Name:  fmt.Sprintf("*.%s", workloadClusterHostedZoneName),
			Value: fmt.Sprintf("ingress.%s", workloadClusterHostedZoneName),
		},
	}

	if cluster.ControlPlaneEndpoint != "" {
		if cluster.IsEKS {
			dnsRecords = append(dnsRecords, DNSRecord{
				Kind:   DnsRecordTypeCname,
				Name:   fmt.Sprintf("api.%s", workloadClusterHostedZoneName),
				Value:  strings.TrimPrefix(cluster.ControlPlaneEndpoint, "https://"),
				Region: cluster.Region,
			})
		} else {
			dnsRecords = append(dnsRecords, DNSRecord{
				Kind:   DnsRecordTypeAlias,
				Name:   fmt.Sprintf("api.%s", workloadClusterHostedZoneName),
				Value:  cluster.ControlPlaneEndpoint,
				Region: cluster.Region,
			})
		}
	}

	if cluster.BastionIp != "" {
		dnsRecords = append(dnsRecords, DNSRecord{
			Kind:  DnsRecordTypeA,
			Name:  fmt.Sprintf("bastion1.%s", workloadClusterHostedZoneName),
			Value: cluster.BastionIp,
		})
	}

	return dnsRecords
}
