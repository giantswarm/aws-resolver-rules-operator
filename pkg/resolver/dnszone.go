package resolver

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

type DnsZone struct {
	// awsClients is a factory to retrieve clients to talk to the AWS API using the right credentials.
	awsClients AWSClients
	// workloadClusterBaseDomain is the root hosted zone used to create the workload cluster hosted zone, i.e. gaws.gigantic.io
	workloadClusterBaseDomain string
}

func NewDnsZone(awsClients AWSClients, workloadClusterBaseDomain string) (DnsZone, error) {
	return DnsZone{
		awsClients:                awsClients,
		workloadClusterBaseDomain: workloadClusterBaseDomain,
	}, nil
}

func (d *DnsZone) CreatePublicHostedZone(ctx context.Context, logger logr.Logger, cluster Cluster) error {
	route53Client, err := d.awsClients.NewRoute53Client(cluster.Region, cluster.IAMRoleARN)
	if err != nil {
		return errors.WithStack(err)
	}

	logger.Info("Creating public hosted zone", "hostedZoneName", d.getHostedZoneName(cluster))
	hostedZoneId, err := route53Client.CreatePublicHostedZone(ctx, logger, d.getHostedZoneName(cluster), d.getTagsForHostedZone(cluster.Name))
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

	return nil
}

func (d *DnsZone) CreatePrivateHostedZone(ctx context.Context, logger logr.Logger, cluster Cluster, vpcsToAssociate []string) error {
	route53Client, err := d.awsClients.NewRoute53Client(cluster.Region, cluster.IAMRoleARN)
	if err != nil {
		return errors.WithStack(err)
	}

	logger.Info("Creating private hosted zone")
	hostedZoneId, err := route53Client.CreatePrivateHostedZone(ctx, logger, d.getHostedZoneName(cluster), cluster.VPCId, cluster.Region, d.getTagsForHostedZone(cluster.Name), vpcsToAssociate)
	if err != nil {
		return errors.WithStack(err)
	}
	logger.Info("Hosted zone created", "hostedZoneId", hostedZoneId, "hostedZoneName", d.getHostedZoneName(cluster))

	return nil
}

func (d *DnsZone) DeleteHostedZone(ctx context.Context, logger logr.Logger, cluster Cluster) error {
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

	logger.Info("Deleting hosted zone")
	err = route53Client.DeleteHostedZone(ctx, logger, hostedZoneId)
	if err != nil {
		return errors.WithStack(err)
	}
	logger.Info("Hosted zone deleted", "hostedZoneId", hostedZoneId)

	return nil
}

func (d *DnsZone) getHostedZoneName(cluster Cluster) string {
	return fmt.Sprintf("%s.%s", cluster.Name, d.workloadClusterBaseDomain)
}

func (d *DnsZone) getParentHostedZoneName() string {
	return d.workloadClusterBaseDomain
}

func (d *DnsZone) getTagsForHostedZone(clusterName string) map[string]string {
	return map[string]string{
		"Name": clusterName,
		fmt.Sprintf("sigs.k8s.io/cluster-api-provider-aws/cluster/%s", clusterName): "owned",
		"sigs.k8s.io/cluster-api-provider-aws/role":                                 "common",
	}
}
