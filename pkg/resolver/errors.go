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
