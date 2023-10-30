package route

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
)

type Route struct {
	awsClients resolver.AWSClients
}

func NewRoute(awsClients resolver.AWSClients) (Route, error) {
	return Route{
		awsClients: awsClients,
	}, nil
}

func (r *Route) AddRoutes(ctx context.Context, transitGatewayID, prefixListID *string, awsCluster *capa.AWSCluster, roleArn string, logger logr.Logger) error {
	subnets := []*string{}
	for _, s := range awsCluster.Spec.NetworkSpec.Subnets {
		temp := s.ID
		subnets = append(subnets, &temp)
	}

	routeTablesClient, err := r.awsClients.NewRouteTablesClient(awsCluster.Spec.Region, roleArn)
	if err != nil {
		return errors.WithStack(err)
	}

	routeTables, err := routeTablesClient.GetRouteTables(ctx, subnets)
	if err != nil {
		return errors.WithStack(err)
	}

	logger.Info("Adding routes to route tables", "routeTables", routeTables)
	for _, rt := range routeTables {
		if !routeExists(rt.Routes, prefixListID, transitGatewayID) {
			err := routeTablesClient.CreateRoute(ctx, rt.RouteTableId, prefixListID, transitGatewayID)
			if err != nil {
				logValues := fmt.Sprintf("routeTableID=%s, prefixListID=%s, transitGatewayID=%s", *rt.RouteTableId, *prefixListID, *transitGatewayID)
				return errors.WithStack(errors.Wrap(err, logValues))
			}
			logger.Info("Added routes to route table", "routeTableID", *rt.RouteTableId, "prefixListID", *prefixListID, "transitGatewayID", *transitGatewayID)
		}
	}

	return nil
}

func (r *Route) RemoveRoutes(ctx context.Context, transitGatewayID, prefixListID *string, awsCluster *capa.AWSCluster, roleArn string, logger logr.Logger) error {
	subnets := []*string{}
	for _, s := range awsCluster.Spec.NetworkSpec.Subnets {
		temp := s.ID
		subnets = append(subnets, &temp)
	}

	routeTablesClient, err := r.awsClients.NewRouteTablesClient(awsCluster.Spec.Region, roleArn)
	if err != nil {
		return errors.WithStack(err)
	}

	routeTables, err := routeTablesClient.GetRouteTables(ctx, subnets)
	if err != nil {
		return errors.WithStack(err)
	}

	for _, rt := range routeTables {
		err := routeTablesClient.DeleteRoute(ctx, rt.RouteTableId, prefixListID)
		if err != nil {
			logValues := fmt.Sprintf("routeTableID: %s, prefixListID: %s, transitGatewayID: %s", *rt.RouteTableId, *prefixListID, *transitGatewayID)
			return errors.WithStack(errors.Wrap(err, logValues))
		}
		logger.Info("Removed routes from route table", "routeTableID", rt.RouteTableId, "prefixListID", prefixListID)
	}

	return nil
}

func routeExists(routes []*ec2.Route, prefixListID, transitGatewayID *string) bool {
	for _, route := range routes {
		if route.DestinationPrefixListId != nil && route.TransitGatewayId != nil && *route.DestinationPrefixListId == *prefixListID && *route.TransitGatewayId == *transitGatewayID {
			return true
		}
	}
	return false
}
