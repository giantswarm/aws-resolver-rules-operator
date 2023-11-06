package aws

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/pkg/errors"
)

type RouteTables struct {
	ec2 *ec2.EC2
}

func (r *RouteTables) CreateRoute(ctx context.Context, routeTableId, prefixListID, transitGatewayID *string) error {
	if _, err := r.ec2.CreateRouteWithContext(ctx, &ec2.CreateRouteInput{
		RouteTableId:            routeTableId,
		DestinationPrefixListId: prefixListID,
		TransitGatewayId:        transitGatewayID,
	}); err != nil {
		return err
	}
	return nil
}

func (r *RouteTables) DeleteRoute(ctx context.Context, routeTableId, prefixListID *string) error {
	if _, err := r.ec2.DeleteRouteWithContext(ctx, &ec2.DeleteRouteInput{
		RouteTableId:            routeTableId,
		DestinationPrefixListId: prefixListID,
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

func (r *RouteTables) GetRouteTables(ctx context.Context, subnets []string) ([]*ec2.RouteTable, error) {
	filterName := "association.subnet-id"
	output, err := r.ec2.DescribeRouteTablesWithContext(ctx, &ec2.DescribeRouteTablesInput{
		Filters: []*ec2.Filter{
			{Name: &filterName, Values: aws.StringSlice(subnets)},
		},
	})
	if err != nil {
		return nil, err
	}

	if output != nil && len(output.RouteTables) > 0 {
		return output.RouteTables, nil
	}

	return nil, nil
}
