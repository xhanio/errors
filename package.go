package errors

import (
	stderrors "errors"
	"net/http"

	"go.uber.org/multierr"
)

// error

// Combine returns an error holding every non-nil error passed to it, or nil if
// there are none. The result implements Unwrap() []error, so errors.Is and
// errors.As examine each combined error in turn. Note that a combined error is
// a tree rather than a chain, and so carries no single category.
func Combine(errors ...error) error {
	return multierr.Combine(errors...)
}

// New returns a new error configured by opts, capturing a stack trace at the
// call site. It never returns nil.
//
// Always give an error a message. With no options, or with WithMessage(""), the
// error carries only its stack trace: it is non-nil, so callers take their error
// branch, but its Error method returns the empty string, so whatever logs it
// records nothing. Such an error reports that something failed while withholding
// what.
func New(opts ...Option) error {
	return newError(opts...)
}

// Newf returns a new error with a formatted message, capturing a stack trace at
// the call site. It is New with a single WithMessage option.
//
// Never call Newf(""). The result is a non-nil error whose Error method returns
// the empty string: callers enter their error branch and log nothing. It also
// corrupts Combine, whose output for Combine(Newf(""), Newf("real")) is
// "; real". Give every error a message.
func Newf(format string, args ...any) error {
	return newError(WithMessage(format, args...))
}

// Wrap returns err wrapped with opts, or nil if err is nil, so it is safe to
// call on a value that may not be an error.
//
// Given no options and an err this package already produced, Wrap returns err
// unchanged. Otherwise it returns a new error whose cause is err, capturing a
// stack trace only when err did not already carry one.
func Wrap(err error, opts ...Option) error {
	return wrapError(err, opts...)
}

// Has reports whether cause exists anywhere in err's tree.
//
// Deprecated: use Is, which performs the same search. Has is retained for
// compatibility and calls the standard library's errors.Is directly. Unlike Is,
// it gives a Category target no special meaning, matching one only when the
// category value itself appears in the tree.
func Has(err error, cause error) bool {
	return stderrors.Is(err, cause)
}

// Wrapf returns err wrapped with a formatted message, or nil if err is nil.
//
// Never call Wrapf(err, ""). Unlike Wrap, Wrapf always applies an option, so an
// empty format does not yield err: it appends a message-less layer to the chain,
// which lengthens Chain and allocates while adding nothing a reader can see.
// When there is no context to add, call Wrap(err), which returns err untouched.
func Wrapf(err error, format string, args ...any) error {
	return wrapError(err, WithMessage(format, args...))
}

// Is reports whether err matches target. Which search it performs depends on
// the dynamic type of target, because the two are not variations of one idea.
//
// If target is a Category, Is reports whether err resolves to that category.
// An error carries exactly one category, decided by the outermost error in the
// chain that carries one; see Error.Category. Is therefore resolves err to a
// single category and compares the two by identity. It does not search err's
// tree, and it never consults the standard library: a category is stored on an
// error rather than wrapped by it, so errors.Is cannot find one. A category
// held by an inner error is shadowed by one held further out, and categories
// from separate NewCategory calls are never equal, even given the same name.
//
// Otherwise Is delegates to the standard library's errors.Is, which matches
// target against every error in err's tree. Use this form for sentinel errors.
func Is(err error, target error) bool {
	switch t := target.(type) {
	case Category:
		switch e := err.(type) {
		case Error:
			return e.Category() == t
		case Category:
			return e == t
		default:
			return false
		}
	case error:
		return stderrors.Is(err, t)
	}
	return false
}

// As finds the first error in err's tree whose concrete type is assignable to
// the value pointed to by target, sets target to that error, and reports whether
// one was found. Use it to recover a concrete error type and read its fields.
//
// As adds nothing to the standard library's errors.As and exists only so that
// callers of this package need not also import errors under an alias, this
// package having taken the name.
//
// As has nothing to do with categories. Unlike Is, it does not special-case a
// Category target, because a category is stored on an error rather than being an
// element of its tree. For a Category c, As(err, &c) reports only whether a
// Category value happens to sit in the chain: true for a bare NotFound.New(),
// false for NotFound.Newf("gone"), whose category is nonetheless NotFound. To
// test a category use Is; to read one use Error.Category.
func As(err error, target any) bool {
	return stderrors.As(err, target)
}

// Message returns err.Error(), or the empty string if err is nil. For errors
// this package produced, that is every message in the chain, joined by ": ".
// To read only the outermost message, use Error.Message.
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

// NewCategory returns a new Category with the given name and HTTP status code,
// and registers it under that name for LookupCategory.
//
// Categories compare by identity, so two calls with the same name and status
// code return categories that are not equal. Registering a name that is already
// taken replaces the earlier entry in the registry, but errors already carrying
// the earlier category keep it.
func NewCategory(category string, statusCode int) Category {
	c := newCategory(category, statusCode)
	categories.mu.Lock()
	defer categories.mu.Unlock()
	categories.data[c.Error()] = c
	return c
}

// LookupCategory returns the category registered under name, or nil if no
// category has been registered under it.
func LookupCategory(name string) Category {
	categories.mu.RLock()
	defer categories.mu.RUnlock()
	return categories.data[name]
}
