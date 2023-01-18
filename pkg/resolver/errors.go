package resolver

import (
	"reflect"
)

type ResolverRuleNotFoundError struct {
	error
}

func (e *ResolverRuleNotFoundError) Error() string {
	return "resolver rule was not found"
}

func (e *ResolverRuleNotFoundError) Is(target error) bool {
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

type ResolverEndpointIpNotFoundError struct {
	error
}

func (e *ResolverEndpointIpNotFoundError) Error() string {
	return "resolver endpoint ip not found"
}

func (e *ResolverEndpointIpNotFoundError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}
