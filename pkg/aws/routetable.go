package aws

import (
	"context"
	"fmt"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type RouteTable struct {
	RouteTableId string
	RouteRules   []resolver.RouteRule
}

type RouteTableClient struct {
	client *ec2.EC2
}

func (r *RouteTableClient) createRoute(ctx context.Context, routeTableId, prefixListID, transitGatewayID string) error {
	if _, err := r.client.CreateRouteWithContext(ctx, &ec2.CreateRouteInput{
		RouteTableId:            aws.String(routeTableId),
		DestinationPrefixListId: aws.String(prefixListID),
		TransitGatewayId:        aws.String(transitGatewayID),
	}); err != nil {
		return err
	}
	return nil
}

func (r *RouteTableClient) deleteRoute(ctx context.Context, routeTableId, prefixListID string) error {
	if _, err := r.client.DeleteRouteWithContext(ctx, &ec2.DeleteRouteInput{
		RouteTableId:            aws.String(routeTableId),
		DestinationPrefixListId: aws.String(prefixListID),
	}); err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "InvalidRoute.NotFound":
				return nil
			default:
				return errors.WithStack(err)
			}
		}
		return err
	}
	return nil
}

func (r *RouteTableClient) GetRouteTables(ctx context.Context, filter resolver.Filter) ([]RouteTable, error) {
	filterName := "association.subnet-id"
	output, err := r.client.DescribeRouteTablesWithContext(ctx, &ec2.DescribeRouteTablesInput{
		Filters: []*ec2.Filter{
			{Name: &filterName, Values: aws.StringSlice(filter)},
		},
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	routeTables := make([]RouteTable, 0)
	if output != nil && len(output.RouteTables) > 0 {
		for _, rt := range output.RouteTables {
			routes := make([]resolver.RouteRule, 0)
			for _, route := range rt.Routes {
				rule := resolver.RouteRule{}
				if route.DestinationPrefixListId == nil {
					rule.DestinationPrefixListId = ""
				} else {
					rule.DestinationPrefixListId = *route.DestinationPrefixListId
				}

				if route.TransitGatewayId == nil {
					rule.TransitGatewayId = ""
				} else {
					rule.TransitGatewayId = *route.TransitGatewayId
				}

				routes = append(routes, rule)
			}
			routeTables = append(routeTables, RouteTable{
				RouteTableId: *rt.RouteTableId,
				RouteRules:   routes,
			})
		}
		return routeTables, nil
	}
	return nil, &RouteTableNotFoundError{}
}

func (r *RouteTableClient) AddRoutes(ctx context.Context, route resolver.RouteRule, filter resolver.Filter) error {
	logger := log.FromContext(ctx)

	routeTables, err := r.GetRouteTables(ctx, filter)
	if err != nil {
		return errors.WithStack(err)
	}

	for _, rt := range routeTables {
		if !routeExists(rt.RouteRules, route) {
			err := r.createRoute(ctx, rt.RouteTableId, route.DestinationPrefixListId, route.TransitGatewayId)
			if err != nil {
				logValues := fmt.Sprintf("routeTableID=%s, prefixListID=%s, transitGatewayID=%s", rt.RouteTableId, route.DestinationPrefixListId, route.TransitGatewayId)
				return errors.WithStack(errors.Wrap(err, logValues))
			}
			logger.Info("Added routes to route table", "routeTableID", rt.RouteTableId, "prefixListID", route.DestinationPrefixListId, "transitGatewayID", route.TransitGatewayId)
		}
	}

	return nil
}

func (r *RouteTableClient) RemoveRoutes(ctx context.Context, rule resolver.RouteRule, subnetFilter resolver.Filter) error {
	logger := log.FromContext(ctx)

	routeTables, err := r.GetRouteTables(ctx, subnetFilter)
	if err != nil {
		return errors.WithStack(err)
	}

	for _, rt := range routeTables {
		err := r.deleteRoute(ctx, rt.RouteTableId, rule.DestinationPrefixListId)
		if err != nil {
			logValues := fmt.Sprintf("routeTableID: %s, prefixListID: %s, transitGatewayID: %s", rt.RouteTableId, rule.DestinationPrefixListId, rule.TransitGatewayId)
			return errors.WithStack(errors.Wrap(err, logValues))
		}
		logger.Info("Removed routes from route table", "routeTableID", rt.RouteTableId, "prefixListID", rule.DestinationPrefixListId)
	}

	return nil
}

func routeExists(routes []resolver.RouteRule, targetRoute resolver.RouteRule) bool {
	for _, route := range routes {
		if route.DestinationPrefixListId != "" && route.TransitGatewayId != "" && route.DestinationPrefixListId == targetRoute.DestinationPrefixListId && route.TransitGatewayId == targetRoute.TransitGatewayId {
			return true
		}
	}
	return false
}
