package errors

import (
	"net/http"

	"go.uber.org/multierr"
)

// error

func Combine(errors ...error) error {
	return multierr.Combine(errors...)
}

func New(opts ...Option) error {
	return newError(opts...)
}

func Newf(format string, args ...any) error {
	return newError(WithMessage(format, args...))
}

func Wrap(err error, opts ...Option) error {
	return wrapError(err, opts...)
}

func Wrapf(err error, format string, args ...any) error {
	return wrapError(err, WithMessage(format, args...))
}

func Has(err error, cause error) bool {
	be, ok := err.(Error)
	if !ok {
		return be == cause
	}
	return be.Has(cause)
}

func Is(err error, target error) bool {
	switch t := target.(type) {
	case Category:
		be, ok := err.(Error)
		if !ok {
			return false
		}
		return be.Category() == t
	case error:
		return err == t
	}
	return false
}

func Message(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

// error category

var (
	Cancaled = NewCategory("Cancelled", 499) // non-standard status code for cancellation

	BadRequest       = NewCategory("BadRequest", http.StatusBadRequest)
	InvalidArgument  = NewCategory("InvalidArgument", http.StatusBadRequest)
	Unauthorized     = NewCategory("Unauthorized", http.StatusUnauthorized)
	Forbidden        = NewCategory("Forbidden", http.StatusForbidden)
	PermissionDenied = NewCategory("PermissionDenied", http.StatusForbidden)
	NotFound         = NewCategory("NotFound", http.StatusNotFound)
	DeadlineExceeded = NewCategory("DeadlineExceeded", http.StatusRequestTimeout)
	Conflict         = NewCategory("Conflict", http.StatusConflict)
	AlreadyExist     = NewCategory("AlreadyExist", http.StatusConflict)
	TooManyRequests  = NewCategory("TooManyRequests", http.StatusTooManyRequests)

	Internal          = NewCategory("Internal", http.StatusInternalServerError)
	NotImplemented    = NewCategory("NotImplemented", http.StatusNotImplemented)
	Unavailable       = NewCategory("Unavailable", http.StatusServiceUnavailable)
	ResourceExhausted = NewCategory("ResourceExhausted", http.StatusServiceUnavailable)

	DBFailed = NewCategory("DBFailed", http.StatusInternalServerError)
)

func NewCategory(category string, statusCode int) Category {
	c := newCategory(category, statusCode)
	categories.mu.Lock()
	defer categories.mu.Unlock()
	categories.data[c.Error()] = c
	return c
}

func LookupCategory(name string) Category {
	categories.mu.RLock()
	defer categories.mu.RUnlock()
	return categories.data[name]
}
