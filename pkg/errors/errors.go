package errors

import (
	"reflect"
	"time"
)

func NewRetryableError(message string, retryAfter time.Duration) *RetryableError {
	return &RetryableError{
		Message:    message,
		retryAfter: retryAfter,
	}
}

type RetryableError struct {
	Message    string
	retryAfter time.Duration
}

func (e *RetryableError) Error() string {
	return e.Message
}

func (e *RetryableError) RetryAfter() time.Duration {
	return e.retryAfter
}

func (e *RetryableError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}
