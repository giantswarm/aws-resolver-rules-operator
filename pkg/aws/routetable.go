package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
)

type RouteTable struct {
	RouteTableId string
	RouteRules   []resolver.RouteRule
}

type RouteTableClient struct {
	client *ec2.Client
}

func (r *RouteTableClient) createRoute(ctx context.Context, routeTableId, prefixListID, transitGatewayID string) error {
	if _, err := r.client.CreateRoute(ctx, &ec2.CreateRouteInput{
		RouteTableId:            aws.String(routeTableId),
		DestinationPrefixListId: aws.String(prefixListID),
		TransitGatewayId:        aws.String(transitGatewayID),
	}); err != nil {
		return err
	}
	return nil
}

func (r *RouteTableClient) deleteRoute(ctx context.Context, routeTableId, prefixListID string) error {
	if _, err := r.client.DeleteRoute(ctx, &ec2.DeleteRouteInput{
		RouteTableId:            aws.String(routeTableId),
		DestinationPrefixListId: aws.String(prefixListID),
	}); err != nil {
		var gae *smithy.GenericAPIError
		if errors.As(err, &gae) {
			if gae.Code == "InvalidRoute.NotFound" {
				return nil
			}
		}
		return errors.WithStack(err)
	}
	return nil
}

func (r *RouteTableClient) getRouteTables(ctx context.Context, filter resolver.Filter) ([]ec2types.RouteTable, error) {
	filterName := "association.subnet-id"
	output, err := r.client.DescribeRouteTables(ctx, &ec2.DescribeRouteTablesInput{
		Filters: []ec2types.Filter{
			{Name: &filterName, Values: filter},
		},
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if output != nil {
		return output.RouteTables, nil
	}
	return nil, &RouteTableNotFoundError{}
}

func (r *RouteTableClient) AddRoutes(ctx context.Context, route resolver.RouteRule, filter resolver.Filter) error {
	logger := log.FromContext(ctx)

	routeTables, err := r.getRouteTables(ctx, filter)
	if err != nil {
		return errors.WithStack(err)
	}

	for _, rt := range routeTables {
		if !routeExists(rt.Routes, route) {
			err := r.createRoute(ctx, *rt.RouteTableId, route.DestinationPrefixListId, route.TransitGatewayId)
			if err != nil {
				logValues := fmt.Sprintf("routeTableID=%s, prefixListID=%s, transitGatewayID=%s", *rt.RouteTableId, route.DestinationPrefixListId, route.TransitGatewayId)
				return errors.WithStack(errors.Wrap(err, logValues))
			}
			logger.Info("Added routes to route table", "routeTableID", rt.RouteTableId, "prefixListID", route.DestinationPrefixListId, "transitGatewayID", route.TransitGatewayId)
		}
	}

	return nil
}

func (r *RouteTableClient) RemoveRoutes(ctx context.Context, rule resolver.RouteRule, subnetFilter resolver.Filter) error {
	logger := log.FromContext(ctx)

	routeTables, err := r.getRouteTables(ctx, subnetFilter)
	if err != nil {
		return errors.WithStack(err)
	}

	for _, rt := range routeTables {
		err := r.deleteRoute(ctx, *rt.RouteTableId, rule.DestinationPrefixListId)
		if err != nil {
			logValues := fmt.Sprintf("routeTableID: %s, prefixListID: %s, transitGatewayID: %s", *rt.RouteTableId, rule.DestinationPrefixListId, rule.TransitGatewayId)
			return errors.WithStack(errors.Wrap(err, logValues))
		}
		logger.Info("Removed routes from route table", "routeTableID", rt.RouteTableId, "prefixListID", rule.DestinationPrefixListId)
	}

	return nil
}

func routeExists(routes []ec2types.Route, targetRoute resolver.RouteRule) bool {
	for _, route := range routes {
		if route.DestinationPrefixListId != nil && route.TransitGatewayId != nil && *route.DestinationPrefixListId == targetRoute.DestinationPrefixListId && *route.TransitGatewayId == targetRoute.TransitGatewayId {
			return true
		}
	}
	return false
}
