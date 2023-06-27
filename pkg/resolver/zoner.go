package resolver

import (
	"context"
	"fmt"

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

func (d *Zoner) CreatePublicHostedZone(ctx context.Context, logger logr.Logger, cluster Cluster) error {
	route53Client, err := d.awsClients.NewRoute53Client(cluster.Region, cluster.IAMRoleARN)
	if err != nil {
		return errors.WithStack(err)
	}

	logger.Info("Creating public hosted zone", "hostedZoneName", d.getHostedZoneName(cluster))
	hostedZoneId, err := route53Client.CreateHostedZone(ctx, logger, BuildPublicHostedZone(d.getHostedZoneName(cluster), d.getTagsForHostedZone(cluster)))
	if err != nil {
		return errors.WithStack(err)
	}
	logger.Info("Hosted zone created", "hostedZoneId", hostedZoneId, "hostedZoneName", d.getHostedZoneName(cluster))

	parentHostedZoneId, err := route53Client.GetHostedZoneIdByName(ctx, logger, d.getParentHostedZoneName())
	if err != nil {
		return errors.WithStack(err)
	}

	logger.Info("Adding delegation to parent hosted zone", "parentHostedZoneId", parentHostedZoneId)
	err = route53Client.AddDelegationToParentZone(ctx, logger, parentHostedZoneId, hostedZoneId)
	if err != nil {
		return errors.WithStack(err)
	}
	logger.Info("Added delegation to parent hosted zone")

	err = d.createDnsRecords(ctx, logger, cluster, hostedZoneId)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (d *Zoner) CreatePrivateHostedZone(ctx context.Context, logger logr.Logger, cluster Cluster, vpcsToAssociate []string) error {
	route53Client, err := d.awsClients.NewRoute53Client(cluster.Region, cluster.IAMRoleARN)
	if err != nil {
		return errors.WithStack(err)
	}

	logger.Info("Creating private hosted zone")
	hostedZoneId, err := route53Client.CreateHostedZone(ctx, logger, BuildPrivateHostedZone(d.getHostedZoneName(cluster), cluster, d.getTagsForHostedZone(cluster), vpcsToAssociate))
	if err != nil {
		return errors.WithStack(err)
	}
	logger.Info("Hosted zone created", "hostedZoneId", hostedZoneId, "hostedZoneName", d.getHostedZoneName(cluster))

	return nil
}

func (d *Zoner) DeleteHostedZone(ctx context.Context, logger logr.Logger, cluster Cluster) error {
	route53Client, err := d.awsClients.NewRoute53Client(cluster.Region, cluster.IAMRoleARN)
	if err != nil {
		return errors.WithStack(err)
	}

	parentHostedZoneId, err := route53Client.GetHostedZoneIdByName(ctx, logger, d.getParentHostedZoneName())
	if err != nil {
		return errors.WithStack(err)
	}

	hostedZoneId, err := route53Client.GetHostedZoneIdByName(ctx, logger, d.getHostedZoneName(cluster))
	if err != nil {
		if errors.Is(err, &HostedZoneNotFoundError{}) {
			logger.Info("Hosted zone can't be found, skipping deletion", "parentHostedZoneId", parentHostedZoneId, "hostedZoneId", hostedZoneId, "hostedZoneName", d.getHostedZoneName(cluster))
			return nil
		}

		return errors.WithStack(err)
	}

	logger.Info("Deleting delegation from parent hosted zone", "parentHostedZoneId", parentHostedZoneId, "hostedZoneId", hostedZoneId)
	err = route53Client.DeleteDelegationFromParentZone(ctx, logger, parentHostedZoneId, hostedZoneId)
	if err != nil {
		return errors.WithStack(err)
	}

	logger.Info("Deleting dns records from hosted zone")
	err = route53Client.DeleteDnsRecordsFromHostedZone(ctx, logger, hostedZoneId, d.getWorkloadClusterDnsRecords(d.workloadClusterBaseDomain, cluster))
	if err != nil {
		return errors.WithStack(err)
	}

	logger.Info("Deleting hosted zone")
	err = route53Client.DeleteHostedZone(ctx, logger, hostedZoneId)
	if err != nil {
		return errors.WithStack(err)
	}
	logger.Info("Hosted zone deleted", "hostedZoneId", hostedZoneId)

	return nil
}

func (d *Zoner) createDnsRecords(ctx context.Context, logger logr.Logger, cluster Cluster, hostedZoneId string) error {
	route53Client, err := d.awsClients.NewRoute53Client(cluster.Region, cluster.IAMRoleARN)
	if err != nil {
		return errors.WithStack(err)
	}

	dnsRecordsToCreate := d.getWorkloadClusterDnsRecords(d.workloadClusterBaseDomain, cluster)
	logger.Info("Creating DNS records", "dnsRecords", dnsRecordsToCreate, "hostedZoneId", hostedZoneId)
	err = route53Client.AddDnsRecordsToHostedZone(ctx, logger, hostedZoneId, d.getWorkloadClusterDnsRecords(d.workloadClusterBaseDomain, cluster))
	if err != nil {
		return errors.WithStack(err)
	}
	logger.Info("DNS records created", "dnsRecords", dnsRecordsToCreate, "hostedZoneId", hostedZoneId)

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

func (d *Zoner) getWorkloadClusterDnsRecords(workloadClusterBaseDomain string, cluster Cluster) []DNSRecord {
	dnsRecords := []DNSRecord{
		{
			Kind:  "CNAME",
			Name:  fmt.Sprintf("*.%s", workloadClusterBaseDomain),
			Value: fmt.Sprintf("ingress.%s", workloadClusterBaseDomain),
		},
	}

	if cluster.ControlPlaneEndpoint != "" {
		dnsRecords = append(dnsRecords, DNSRecord{
			Kind:   "ALIAS",
			Name:   fmt.Sprintf("api.%s", workloadClusterBaseDomain),
			Value:  cluster.ControlPlaneEndpoint,
			Region: cluster.Region,
		})
	}

	if cluster.BastionIp != "" {
		dnsRecords = append(dnsRecords, DNSRecord{
			Kind:  "A",
			Name:  fmt.Sprintf("bastion1.%s", workloadClusterBaseDomain),
			Value: cluster.BastionIp,
		})
	}

	return dnsRecords
}
