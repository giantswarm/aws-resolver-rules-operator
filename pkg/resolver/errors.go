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

type HostedZoneNotFoundError struct {
	error
}

func (e *HostedZoneNotFoundError) Error() string {
	return "hosted zone was not found"
}

func (e *HostedZoneNotFoundError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}
