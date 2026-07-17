package testrun

import "errors"

var (
	ErrNotFound     = errors.New("test run not found")
	ErrMissingField = errors.New("missing field")
	ErrInvalidField = errors.New("invalid field")
)
