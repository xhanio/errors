package errors

import (
	"fmt"
	"io"
	"runtime"
	"strconv"
	"strings"
)

// Combine returns an error reporting that an operation failed as a whole for
// every non-nil reason passed to it. It returns nil if there are none, and the
// reason itself if there is only one.
//
// Combine is for failures that reject the whole operation, such as a request
// with several invalid fields. It is not for partial failures, such as a batch
// in which some items failed: handle those item by item where they happen, or
// return a result per item, since a combined error keeps no record of which
// item each reason belongs to.
//
// Declare what the failure as a whole is by wrapping the result with a
// category, which takes precedence over anything inside it:
//
//	return errors.BadRequest.Wrapf(errors.Combine(fieldErrs...), "invalid request")
//
// An undeclared result is like an unlabelled archive. It resolves to the
// Combined category, even when every reason has the same category, because
// nothing about the operation can be judged until the archive is unpacked:
// examine its reasons with Any, All, or ErrorsOf, and read the code of each with
// CodeOf. Only categories are sealed this way. The result implements
// Unwrap() []error, so errors.Is, errors.As, and Is with a sentinel error still
// see every reason.
//
// The result's Error method joins the reasons' messages with "; ", for reading
// in logs; do not parse it, use ErrorsOf instead. Its stack trace is captured
// where Combine is called. Reasons that are themselves results of Combine are
// flattened into the new one. A result holding more than one reason implements
// Errors.
func Combine(errs ...error) error {
	return combine(errs)
}

// combine builds the result of Combine. It is a separate function so that
// callers skips the same number of frames as it does for New and Wrap.
func combine(errs []error) error {
	var reasons []error
	for _, err := range errs {
		switch e := err.(type) {
		case nil:
		case *combined:
			reasons = append(reasons, e.errs...)
		default:
			reasons = append(reasons, err)
		}
	}
	switch len(reasons) {
	case 0:
		return nil
	case 1:
		return reasons[0]
	}
	return &combined{errs: reasons, stack: callers()}
}

var _ Errors = (*combined)(nil)

// combined is the archive Combine returns: the reasons an operation failed,
// kept as errors so that each keeps its category, code, details, and stack. It
// has no message of its own. It implements Errors but not Error, since
// the chain methods there are not defined for trees.
type combined struct {
	errs  []error
	stack *stack
}

// Category returns Combined. The archive as a whole is not judged until it is
// unpacked; a wrapper can declare another category, which then takes
// precedence for CategoryOf.
func (c *combined) Category() Category {
	return Combined
}

// Errors returns the reasons, as a copy.
func (c *combined) Errors() []error {
	return append([]error(nil), c.errs...)
}

// Error joins the reasons' messages with "; ".
func (c *combined) Error() string {
	var b strings.Builder
	for i, err := range c.errs {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(err.Error())
	}
	return b.String()
}

// Unwrap returns the reasons, so the standard library can examine each.
func (c *combined) Unwrap() []error {
	return c.errs
}

// StackTrace returns the stack captured where Combine was called.
func (c *combined) StackTrace() []runtime.Frame {
	return StackTrace(c)
}

// StackFrames returns the program counters behind StackTrace, unresolved, for
// error reporters such as Sentry.
func (c *combined) StackFrames() []uintptr {
	if s := stackOf(c); s != nil {
		return append([]uintptr(nil), s.pcs...)
	}
	return nil
}

// Format implements fmt.Formatter. %s, %v, and %m print Error(), and %q prints
// it quoted. %+v adds the stack captured by Combine and then each reason in its
// own %+v form, numbered and indented.
func (c *combined) Format(f fmt.State, verb rune) {
	msg := c.Error()
	switch verb {
	case 's', 'm':
		io.WriteString(f, msg)
	case 'q':
		fmt.Fprintf(f, "%q", msg)
	case 'v':
		io.WriteString(f, msg)
		if f.Flag('+') {
			writeFrames(f, c.StackTrace())
			writeReasons(f, c.errs)
		}
	default:
		fmt.Fprintf(f, "!%%%c(%s)", verb, msg)
	}
}

// ErrorsOf returns the reasons err holds: the members of the first tree of errors
// in its chain, such as a result of Combine or errors.Join, found through
// wrappers from any package. An error that holds no tree is its own only
// reason. ErrorsOf returns nil if err is nil.
//
// Use ErrorsOf to list why an operation failed as a whole, for example to report
// each invalid field to the client, reading each reason's code with CodeOf, or
// to make a judgement that Any and All cannot express. The result is a copy;
// changing it does not change err.
func ErrorsOf(err error) []error {
	if err == nil {
		return nil
	}
	if reasons := reasonsOf(err); reasons != nil {
		return append([]error(nil), reasons...)
	}
	return []error{err}
}

// Any reports whether at least one reason that ErrorsOf returns for err matches
// target. Each reason is judged as a single error is by Is: against a Category
// by the category CategoryOf resolves it to, so that a reason without one
// usually counts as Internal, and against any other error by errors.Is. Any
// returns false if err is nil.
//
// Is answers for an error as a whole, and a tree of errors as a whole is
// Combined. Any and All answer for its reasons.
func Any(err, target error) bool {
	for _, reason := range ErrorsOf(err) {
		if Is(reason, target) {
			return true
		}
	}
	return false
}

// All reports whether every reason that ErrorsOf returns for err matches target,
// judging each as Any does. All returns false if err is nil, rather than
// reporting that every one of no reasons matches.
func All(err, target error) bool {
	reasons := ErrorsOf(err)
	if len(reasons) == 0 {
		return false
	}
	for _, reason := range reasons {
		if !Is(reason, target) {
			return false
		}
	}
	return true
}

// reasonsOf returns the members of the first tree of errors in err's chain,
// walking through wrappers from any package, or nil if the chain holds none.
func reasonsOf(err error) []error {
	for err != nil {
		switch u := err.(type) {
		case interface{ Unwrap() []error }:
			return u.Unwrap()
		case interface{ Unwrap() error }:
			err = u.Unwrap()
		default:
			return nil
		}
	}
	return nil
}

// writeReasons writes each reason in its verbose form, on lines of its own
// headed "[i/n]", with any further lines indented beneath it.
func writeReasons(w io.Writer, errs []error) {
	n := strconv.Itoa(len(errs))
	for i, err := range errs {
		v := strings.ReplaceAll(verbose(err), "\n", "\n    ")
		io.WriteString(w, "\n["+strconv.Itoa(i+1)+"/"+n+"] "+v)
	}
}

// verbose returns err in its %+v form. A reason wrapped by another package,
// such as by fmt.Errorf("%w"), usually implements no fmt.Formatter, so %+v
// would print only its message; for those, verbose appends the stack that
// StackTrace finds through the wrapper.
func verbose(err error) string {
	if _, ok := err.(fmt.Formatter); ok {
		return fmt.Sprintf("%+v", err)
	}
	var b strings.Builder
	b.WriteString(err.Error())
	writeFrames(&b, StackTrace(err))
	return b.String()
}
