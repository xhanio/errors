package errors

import (
	stderrors "errors"
	"fmt"
	"io"
	"runtime"
	"strings"

	"github.com/xhanio/labels"
)

var _ Error = (*base)(nil)

type base struct {
	stack *stack

	category Category
	code     string
	message  string
	details  labels.Set
	cause    error
}

// apply executes the given options on the error base and returns the modified error base
func (b *base) apply(opts ...Option) *base {
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// root traverses the error chain to find the root error base
func (b *base) rootBase() *base {
	curr := b
	for curr != nil && curr.cause != nil {
		cb, ok := curr.cause.(*base)
		if !ok {
			return curr
		}
		curr = cb
	}
	return curr
}

// Message returns the error message, traversing the cause chain if the current message is empty
func (b *base) Message() string {
	curr := b
	for curr != nil && curr.message == "" {
		if curr.cause == nil {
			return curr.message
		}
		cb, ok := curr.cause.(*base)
		if !ok {
			return curr.cause.Error()
		}
		curr = cb
	}
	return curr.message
}

// Error returns the formatted error string by concatenating all messages in the error chain
func (b *base) Error() string {
	var builder strings.Builder
	curr := b
	for curr != nil {
		if curr.message != "" {
			if builder.Len() > 0 {
				builder.WriteString(": ")
			}
			builder.WriteString(curr.message)
		}
		cb, ok := curr.cause.(*base)
		if !ok {
			if curr.cause != nil {
				if builder.Len() > 0 {
					builder.WriteString(": ")
				}
				builder.WriteString(curr.cause.Error())
			}
			break
		}
		curr = cb
	}
	return builder.String()
}

// Code returns the outermost code in the chain and its details, as CodeOf does.
func (b *base) Code() (string, labels.Set) {
	return CodeOf(b)
}

// Category returns the category the error resolves to, as CategoryOf does.
func (b *base) Category() Category {
	return CategoryOf(b)
}

// Cause returns the underlying cause of this error
func (b *base) Cause() error {
	return b.cause
}

// Unwrap returns the underlying cause of this error, allowing the standard
// library's errors.Is and errors.As to traverse the error chain
func (b *base) Unwrap() error {
	return b.cause
}

// StackTrace returns the stack trace the package-level StackTrace reports for
// this error, as resolved frames, or nil if none was captured.
func (b *base) StackTrace() []runtime.Frame {
	return StackTrace(b)
}

// StackFrames returns the program counters behind StackTrace, unresolved, or
// nil if none were captured. Error reporters such as Sentry look this method up
// by name and prefer it to StackTrace, and raw program counters let them expand
// inlined calls themselves.
func (b *base) StackFrames() []uintptr {
	if s := stackOf(b); s != nil {
		return append([]uintptr(nil), s.pcs...)
	}
	return nil
}

// Format implements fmt.Formatter. %s and %v print Error() and nothing else, so
// an error from this package reads like any other when printed or wrapped with
// fmt.Errorf("%w"), which renders its operand with %v. %+v adds the code and
// details before the message and the stack trace after it, followed by each
// reason in its own %+v form when the chain holds a tree of errors, such as one
// from Combine. %q prints Error() quoted, and %m prints only the outermost
// message.
func (b *base) Format(f fmt.State, c rune) {
	msg := b.Error()
	switch c {
	case 'm':
		io.WriteString(f, b.Message())
	case 's':
		io.WriteString(f, msg)
	case 'q':
		fmt.Fprintf(f, "%q", msg)
	case 'v':
		if !f.Flag('+') {
			io.WriteString(f, msg)
			return
		}
		if code, details := b.Code(); code != "" {
			if len(details) > 0 {
				code += ":" + details.String()
			}
			msg = "{" + code + "}" + msg
		}
		io.WriteString(f, msg)
		writeFrames(f, b.StackTrace())
		if reasons := reasonsOf(b); reasons != nil {
			writeReasons(f, reasons)
		}
	default:
		fmt.Fprintf(f, "!%%%c(%s)", c, msg)
	}
}

// Has reports whether cause exists anywhere in this error's tree.
//
// Deprecated: use the package-level Is, which performs the same search.
func (b *base) Has(cause error) bool {
	return stderrors.Is(b, cause)
}

// RootCause returns the root cause error at the end of the error chain
func (b *base) RootCause() error {
	r := b.rootBase()
	if r.cause != nil {
		return r.cause
	}
	return r
}

// Chain returns a slice of all errors in the error chain, starting from the current error
func (b *base) Chain() []error {
	var errs []error
	curr := b
	for curr != nil {
		errs = append(errs, curr)
		if curr.cause == nil {
			break
		}
		cb, ok := curr.cause.(*base)
		if !ok {
			break
		}
		curr = cb
	}
	return errs
}
