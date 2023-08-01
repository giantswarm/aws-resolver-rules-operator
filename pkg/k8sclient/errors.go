package k8sclient

import (
	"reflect"
)

type BastionNotFoundError struct {
}

func (e *BastionNotFoundError) Error() string {
	return "hosted zone was not found"
}

func (e *BastionNotFoundError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}
