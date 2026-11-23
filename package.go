package errors

import (
	stderrors "errors"
	"net/http"
)

// error

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
// The result has err as its cause and captures a stack trace only when err
// carries none worth reporting, so that StackTrace points at where the failure
// began. A stack captured during package initialisation, as by a package-level
// sentinel, is not worth reporting, so wrapping such an error captures a fresh
// one. Given no options, Wrap adds nothing else, and it may return err itself.
// Do not rely on either outcome: compare errors with Is, never with ==. The
// result of wrapping a non-nil err always implements Error.
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
// When there is no context to add, call Wrap(err), which adds no message.
func Wrapf(err error, format string, args ...any) error {
	return wrapError(err, WithMessage(format, args...))
}

// Is reports whether err matches target. Which search it performs depends on
// the dynamic type of target, because the two are not variations of one idea.
//
// If target is a Category, Is reports whether CategoryOf(err) is that category.
// An error resolves to exactly one category, decided by the outermost error in
// the chain that carries one, through wrappers from any package; see CategoryOf.
// A category held by an inner error is shadowed by one held further out, and
// categories from separate NewCategory calls are never equal, even given the
// same name. A tree of errors, such as one from Combine, resolves to Combined
// unless a wrapper declares a category, so Is does not answer for its members;
// use Any or All for that.
//
// Otherwise Is delegates to the standard library's errors.Is, which matches
// target against every error in err's tree, members of a Combine included. Use
// this form for sentinel errors.
func Is(err error, target error) bool {
	if t, ok := target.(Category); ok {
		return err != nil && CategoryOf(err) == t
	}
	return stderrors.Is(err, target)
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
// test a category use Is; to read one use CategoryOf.
func As(err error, target any) bool {
	return stderrors.As(err, target)
}

// Message returns err.Error(), or the empty string if err is nil. For errors
// this package produced, that is every message in the chain, joined by ": ",
// including the text of any error from another package that the chain wraps.
// To read only the outermost message this package set, use MessageOf.
func Message(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

// error category

var (
	Canceled = NewCategory("Canceled", 499) // non-standard status code for cancellation

	// Cancaled is a misspelling of Canceled, kept so existing code compiles.
	// It is the same category, so errors created with either name match both.
	//
	// Deprecated: use Canceled.
	Cancaled = Canceled

	BadRequest        = NewCategory("BadRequest", http.StatusBadRequest)
	InvalidArgument   = NewCategory("InvalidArgument", http.StatusBadRequest)
	Unauthorized      = NewCategory("Unauthorized", http.StatusUnauthorized)
	Forbidden         = NewCategory("Forbidden", http.StatusForbidden)
	PermissionDenied  = NewCategory("PermissionDenied", http.StatusForbidden)
	NotFound          = NewCategory("NotFound", http.StatusNotFound)
	Conflict          = NewCategory("Conflict", http.StatusConflict)
	AlreadyExist      = NewCategory("AlreadyExist", http.StatusConflict)
	TooManyRequests   = NewCategory("TooManyRequests", http.StatusTooManyRequests)
	ResourceExhausted = NewCategory("ResourceExhausted", http.StatusTooManyRequests) // gRPC RESOURCE_EXHAUSTED maps to 429

	Internal         = NewCategory("Internal", http.StatusInternalServerError)
	NotImplemented   = NewCategory("NotImplemented", http.StatusNotImplemented)
	Unavailable      = NewCategory("Unavailable", http.StatusServiceUnavailable)
	DeadlineExceeded = NewCategory("DeadlineExceeded", http.StatusGatewayTimeout) // gRPC DEADLINE_EXCEEDED maps to 504

	DBFailed = NewCategory("DBFailed", http.StatusInternalServerError)

	// Combined is the category of a tree of errors, such as one from Combine or
	// errors.Join, that no wrapper has given a category. Such an error is like
	// an unlabelled archive: it reports that an operation failed for several
	// reasons, and nothing about the operation can be judged until it is
	// unpacked with Any, All, or ErrorsOf. Declare the failure as a whole instead,
	// for example BadRequest.Wrapf(Combine(errs...), "invalid request").
	//
	// Combined maps to 500: an undeclared tree reaching an HTTP handler means
	// whoever combined the errors left out what they meant. It is distinct from
	// Internal, so a handler can recognise a tree and unpack it.
	Combined = NewCategory("Combined", http.StatusInternalServerError)
)

func init() {
	// Canceled was named "Cancelled" before v1.2. Keep the old name resolvable,
	// so a name recorded or sent by an older version still looks up.
	categories.mu.Lock()
	defer categories.mu.Unlock()
	categories.data["Cancelled"] = Canceled
}

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
