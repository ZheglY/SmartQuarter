package domain

import "errors"

var (
	ErrInvalid         = errors.New("invalid argument")
	ErrUnauthenticated = errors.New("actor context required")
	ErrPermission      = errors.New("access denied")
	ErrNotFound        = errors.New("resource not found")
	ErrExists          = errors.New("already exists")
	ErrPrecondition    = errors.New("precondition failed")
	ErrLimit           = errors.New("limit exceeded")
	ErrUnavailable     = errors.New("dependency unavailable")
)

type Error struct {
	Kind    error
	Message string
}

func (e *Error) Error() string              { return e.Message }
func (e *Error) Unwrap() error              { return e.Kind }
func Fail(kind error, message string) error { return &Error{Kind: kind, Message: message} }
