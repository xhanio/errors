package errors

import (
	"fmt"

	"github.com/xhanio/labels"
)

type Error interface {
	Message() string            // outermost message, else the text of the first cause that is not a layer of this package; see MessageOf
	Error() string              // all error messages concatenated
	Format(f fmt.State, c rune) // %s, %v, %q and %m print messages; %+v adds code, details and stack
	Code() (string, labels.Set) // outermost code and its details, as CodeOf
	Category() Category         // resolved category, as CategoryOf
	Has(cause error) bool
	Cause() error
	RootCause() error
	Chain() []error
}

// Errors is implemented by the result of Combine when it holds more
// than one reason, as Error is implemented by the result of New. A wrapper
// hides it, as it hides Error, so prefer the package-level ErrorsOf, Any, All,
// and CategoryOf, which see through wrappers from any package.
type Errors interface {
	Error() string              // the reasons' messages, joined by "; "
	Format(f fmt.State, c rune) // %s, %v, %q and %m print Error(); %+v adds the stack and each reason
	Category() Category         // always Combined: the archive is not judged until unpacked
	Errors() []error            // the reasons, as a copy
	Unwrap() []error            // the reasons, for errors.Is and errors.As
}

type Category interface {
	Error() string
	StatusCode() int
	New(opts ...Option) error
	Newf(format string, args ...any) error
	Wrap(err error, opts ...Option) error
	Wrapf(err error, format string, args ...any) error
}
