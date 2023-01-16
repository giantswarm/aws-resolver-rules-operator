package aws

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
