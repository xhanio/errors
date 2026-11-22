package errors

import (
	"fmt"
	"io"
	"runtime"
)

// Handled returns an error with err's message that hides err itself. It returns
// nil if err is nil.
//
// Use Handled to absorb an error you have dealt with, typically one from
// another service, so that nothing about it leaks into your own handling. The
// result implements no Unwrap, so errors.Is and errors.As do not see err or
// anything it wraps, and CategoryOf, CodeOf, Any, All, and ErrorsOf treat the
// result as an error of its own: it resolves to Internal and carries no code. A
// downstream NotFound, for example, would otherwise become your own 404. Wrap
// the result with a category to say what the failure means to you:
//
//	return errors.Unavailable.Wrapf(errors.Handled(err), "call users service")
//
// The stack trace is kept, because it is for diagnosis rather than handling:
// StackTrace sees through the barrier, and %+v prints err in its verbose form.
func Handled(err error) error {
	switch err.(type) {
	case nil:
		return nil
	case *handled:
		return err
	}
	return &handled{err: err}
}

// handled is the barrier Handled returns.
type handled struct {
	err error
}

// Error returns the absorbed error's message.
func (h *handled) Error() string {
	return h.err.Error()
}

// StackTrace returns the stack of the absorbed error, as StackTrace reports it.
func (h *handled) StackTrace() []runtime.Frame {
	return StackTrace(h)
}

// StackFrames returns the program counters behind StackTrace, unresolved, for
// error reporters such as Sentry.
func (h *handled) StackFrames() []uintptr {
	if s := stackOf(h); s != nil {
		return append([]uintptr(nil), s.pcs...)
	}
	return nil
}

// Format implements fmt.Formatter. %s, %v, and %m print Error(), and %q prints
// it quoted. %+v prints the absorbed error in its own verbose form.
func (h *handled) Format(f fmt.State, verb rune) {
	msg := h.Error()
	switch verb {
	case 's', 'm':
		io.WriteString(f, msg)
	case 'q':
		fmt.Fprintf(f, "%q", msg)
	case 'v':
		if f.Flag('+') {
			io.WriteString(f, verbose(h.err))
			return
		}
		io.WriteString(f, msg)
	default:
		fmt.Fprintf(f, "!%%%c(%s)", verb, msg)
	}
}
