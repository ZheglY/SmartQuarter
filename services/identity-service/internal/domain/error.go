package domain

import "errors"

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrHouseNotFound      = errors.New("house not found")
	ErrMembershipNotFound = errors.New("membership not found")
	ErrMembershipInactive = errors.New("membership is inactive")
	ErrInvalidInput       = errors.New("invalid input data")
)
