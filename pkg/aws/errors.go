package aws

import (
	"reflect"
)

type SecurityGroupNotFoundError struct {
	error
}

func (e *SecurityGroupNotFoundError) Error() string {
	return "security group was not found"
}

func (e *SecurityGroupNotFoundError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}

type ResolverEndpointNotFoundError struct {
	error
}

func (e *ResolverEndpointNotFoundError) Error() string {
	return "resolver endpoint was not found"
}

func (e *ResolverEndpointNotFoundError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}

type HostedZoneNotFoundError struct {
	error
}

func (e *HostedZoneNotFoundError) Error() string {
	return "hosted zone was not found"
}

func (e *HostedZoneNotFoundError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}
