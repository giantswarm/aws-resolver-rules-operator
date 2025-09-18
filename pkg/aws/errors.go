package aws

import (
	"reflect"

	"github.com/aws/smithy-go"
	"github.com/pkg/errors"
)

const (
	ErrAssociationNotFound = "InvalidAssociationID.NotFound"
	ErrRouteTableNotFound  = "InvalidRouteTableID.NotFound"
	ErrPrefixListNotFound  = "InvalidPrefixListID.NotFound"
	ErrSubnetNotFound      = "InvalidSubnetID.NotFound"
	ErrVPCNotFound         = "InvalidVpcID.NotFound"
	ErrIncorrectState      = "IncorrectState"
)

func HasErrorCode(err error, code string) bool {
	var gae *smithy.GenericAPIError
	ok := errors.As(err, &gae)
	if !ok {
		return false
	}

	return gae.Code == code
}

type TransitGatewayNotReadyError struct{}

func (e *TransitGatewayNotReadyError) Error() string {
	return "transit gateway not ready"
}

func (e *TransitGatewayNotReadyError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}

type SecurityGroupNotFoundError struct{}

func (e *SecurityGroupNotFoundError) Error() string {
	return "security group was not found"
}

func (e *SecurityGroupNotFoundError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}

type ResolverEndpointNotFoundError struct{}

func (e *ResolverEndpointNotFoundError) Error() string {
	return "resolver endpoint was not found"
}

func (e *ResolverEndpointNotFoundError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}

type HostedZoneNotFoundError struct{}

func (e *HostedZoneNotFoundError) Error() string {
	return "hosted zone was not found"
}

func (e *HostedZoneNotFoundError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}

type DnsRecordNotSupportedError struct{}

func (e *DnsRecordNotSupportedError) Error() string {
	return "dns record type is not supported"
}

func (e *DnsRecordNotSupportedError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}

type RouteTableNotFoundError struct{}

func (e *RouteTableNotFoundError) Error() string {
	return "route table was not found"
}

func (e *RouteTableNotFoundError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}
