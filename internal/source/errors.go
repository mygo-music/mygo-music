package source

import (
	"errors"
	"fmt"
	"time"
)

// Code says what kind of failure an Error is; the codes are the SDK
// specification's (docs/03-abi.md).
type Code string

const (
	CodeUnauthenticated Code = "unauthenticated"
	CodeUnavailable     Code = "unavailable"
	CodeNotFound        Code = "not_found"
	CodeRateLimited     Code = "rate_limited"
	CodeNetwork         Code = "network"
	CodeInvalidArgument Code = "invalid_argument"
	CodeUnsupported     Code = "unsupported"
	CodeConflict        Code = "conflict"
	CodeInternal        Code = "internal"
)

// An Error is a failure a source reports. Compare with the Err values by
// errors.Is: they match any Error of the same Code.
type Error struct {
	Code    Code
	Message string
	// RetryAfter is when to ask again, for CodeRateLimited.
	RetryAfter time.Duration
	// Err is the failure underneath, if any.
	Err error
}

func (e *Error) Error() string {
	switch {
	case e.Message != "" && e.Err != nil:
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	case e.Message != "":
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	case e.Err != nil:
		return fmt.Sprintf("%s: %v", e.Code, e.Err)
	}
	return string(e.Code)
}

func (e *Error) Unwrap() error { return e.Err }

// Is matches an Error of e's Code.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && t.Code == e.Code
}

// What to match with errors.Is.
var (
	ErrUnauthenticated = &Error{Code: CodeUnauthenticated}
	ErrUnavailable     = &Error{Code: CodeUnavailable}
	ErrNotFound        = &Error{Code: CodeNotFound}
	ErrRateLimited     = &Error{Code: CodeRateLimited}
	ErrNetwork         = &Error{Code: CodeNetwork}
	ErrInvalidArgument = &Error{Code: CodeInvalidArgument}
	ErrUnsupported     = &Error{Code: CodeUnsupported}
	// ErrConflict is a change that is made already, as adding a track to a
	// playlist that has it.
	ErrConflict = &Error{Code: CodeConflict}
	ErrInternal = &Error{Code: CodeInternal}
)

// Wrap makes err an Error of code, unless it is one already.
func Wrap(code Code, err error) error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return err
	}
	return &Error{Code: code, Err: err}
}

// Fail makes an Error of code with a message for the listener.
func Fail(code Code, message string) error { return &Error{Code: code, Message: message} }
