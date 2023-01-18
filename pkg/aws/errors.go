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
