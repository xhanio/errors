package errors

import (
	"context"
	stderrors "errors"

	"github.com/xhanio/labels"
)

// CategoryOf returns the category err resolves to, or nil if err is nil. Every
// non-nil error resolves to exactly one category, whichever package created it.
//
// CategoryOf walks err's chain from the outside in, through wrappers from any
// package, and returns the first category it meets: one set on an error from
// this package, or a Category used as an error itself. The outermost category
// therefore wins, so a service boundary can reclassify what it wraps.
//
// An error that carries no category resolves to Internal, with two exceptions
// from the context package: an error that errors.Is reports as
// context.Canceled resolves to Canceled, and one it reports as
// context.DeadlineExceeded resolves to DeadlineExceeded. A caller that gave up,
// or a deadline that passed, is then not reported as a server fault.
//
// A tree of errors, such as one from Combine, errors.Join, or fmt.Errorf with
// several %w verbs, resolves to Combined unless an error wrapping it declares a
// category. CategoryOf does not look inside it: the members are the reasons an
// operation failed, not the failure itself. Use Any, All, or ErrorsOf to examine
// them.
//
// Prefer CategoryOf to asserting the Error interface, which fails as soon as
// another package wraps the error:
//
//	status := errors.CategoryOf(err).StatusCode()
func CategoryOf(err error) Category {
	if err == nil {
		return nil
	}
	if c := givenCategory(err); c != nil {
		return c
	}
	// The chain holds no category and no tree, so errors.Is walks only the
	// chain just examined.
	switch {
	case stderrors.Is(err, context.Canceled):
		return Canceled
	case stderrors.Is(err, context.DeadlineExceeded):
		return DeadlineExceeded
	}
	return Internal
}

// Categorized reports whether the category CategoryOf resolves err to was
// given to err rather than assumed: whether err's chain holds a category, set
// on an error from this package or used as an error itself, or a tree of
// errors, which resolves to Combined. Categorized reports false for a nil
// error, and for one whose category is only CategoryOf's default: Internal, or
// Canceled or DeadlineExceeded for a context error. A declared Internal counts
// as given.
//
// Use Categorized where an error type from another package carries a
// classification of its own that this package cannot read, such as the status
// code of a web framework's HTTP error. An adapter can honour that
// classification when Categorized reports false, without overriding a
// category that a wrapper declared:
//
//	status := errors.CategoryOf(err).StatusCode()
//	var he *echo.HTTPError
//	if !errors.Categorized(err) && errors.As(err, &he) {
//		status = he.Code
//	}
//
// Honour only classifications made for the request being served, such as by
// the framework serving it. A classification from another service is that
// service's verdict on its own request; absorb such an error with Handled,
// which neither Categorized nor errors.As looks behind.
//
// Categorized does not look at where a category sits relative to an error from
// another package. A category declared beneath a framework error still counts:
// when the framework error wraps NotFound.Newf(...), the adapter above answers
// 404 rather than the framework's status.
func Categorized(err error) bool {
	return givenCategory(err) != nil
}

// givenCategory returns the first category in err's chain, walking from the
// outside in through wrappers from any package: one set on an error from this
// package, or a Category used as an error itself. It returns Combined if it
// meets a tree of errors first, and nil if the chain holds neither.
func givenCategory(err error) Category {
	for err != nil {
		switch e := err.(type) {
		case *base:
			if e.category != nil {
				return e.category
			}
		case Category:
			return e
		}
		switch u := err.(type) {
		case interface{ Unwrap() []error }:
			return Combined
		case interface{ Unwrap() error }:
			err = u.Unwrap()
		default:
			return nil
		}
	}
	return nil
}

// CodeOf returns the outermost code in err's chain together with the details
// given alongside it, or an empty code and nil details if there is none. Like
// CategoryOf, it walks through wrappers from any package and stops at a tree of
// errors, which carries no code of its own. To read the code of each member of
// a tree, apply CodeOf to each error that ErrorsOf returns.
func CodeOf(err error) (string, labels.Set) {
	for err != nil {
		if b, ok := err.(*base); ok && b.code != "" {
			return b.code, b.details
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			break
		}
		err = u.Unwrap()
	}
	return "", nil
}

// MessageOf returns the outermost message set with this package in err's
// chain, as by Newf, Wrapf, or WithMessage, or the empty string if there is
// none. Like CodeOf, it walks through wrappers from any package and stops at a
// tree of errors, which carries no message of its own. Handled stops it too:
// an absorbed error's message is not yours.
//
// Unlike Error.Message, MessageOf never falls back to the text of an error
// from another package. Wrap(dbErr).Message() returns the database driver's
// message, which can name hosts, users, or queries; MessageOf returns the
// empty string.
//
// Use MessageOf when a response to a client is to carry a message, since it is
// text your code wrote rather than text it received. It does not make the
// message safe to show: what a message says is up to whoever wrote it.
func MessageOf(err error) string {
	for err != nil {
		if b, ok := err.(*base); ok && b.message != "" {
			return b.message
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			break
		}
		err = u.Unwrap()
	}
	return ""
}
