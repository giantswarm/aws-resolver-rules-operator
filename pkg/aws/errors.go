package aws

import (
	"reflect"
)

type SecurityGroupNotFoundError struct {
}

func (e *SecurityGroupNotFoundError) Error() string {
	return "security group was not found"
}

func (e *SecurityGroupNotFoundError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}

type ResolverEndpointNotFoundError struct {
}

func (e *ResolverEndpointNotFoundError) Error() string {
	return "resolver endpoint was not found"
}

func (e *ResolverEndpointNotFoundError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}

type HostedZoneNotFoundError struct {
}

func (e *HostedZoneNotFoundError) Error() string {
	return "hosted zone was not found"
}

func (e *HostedZoneNotFoundError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}

type DnsRecordNotSupportedError struct {
}

func (e *DnsRecordNotSupportedError) Error() string {
	return "dns record type is not supported"
}

func (e *DnsRecordNotSupportedError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}

type RouteTableNotFoundError struct {
}

func (e *RouteTableNotFoundError) Error() string {
	return "route table was not found"
}

func (e *RouteTableNotFoundError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}
