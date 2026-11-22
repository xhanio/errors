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
	for curr := err; curr != nil; {
		switch e := curr.(type) {
		case *base:
			if e.category != nil {
				return e.category
			}
		case Category:
			return e
		}
		switch u := curr.(type) {
		case interface{ Unwrap() []error }:
			return Combined
		case interface{ Unwrap() error }:
			curr = u.Unwrap()
		default:
			curr = nil
		}
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
