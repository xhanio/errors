# Errors Package

A structured error handling package for Go applications that provides enhanced error management with categories, codes, stack traces, and error chaining.

## Installation

```sh
go get github.com/xhanio/errors
```

## Features

- **Error Categories**: Predefined HTTP-compatible error categories (BadRequest, NotFound, Internal, etc.)
- **Error Codes**: Custom error codes with key-value details for structured error identification
- **Stack Traces**: Automatic stack trace capture for debugging
- **Error Chaining**: Support for wrapping and chaining errors with context
- **Combined Errors**: One error for an operation that failed for several reasons, with queries over the reasons
- **Flexible Formatting**: Multiple output formats including structured details and stack traces
- **Error Traversal**: Methods to navigate error chains and find specific causes

## Integration with Standard Errors

Errors from this package implement `Unwrap() error`, so the standard library
traverses them like any other wrapped error. Chains may mix freely with errors
from other packages, in either direction:

```go
// Standard error
stdErr := fmt.Errorf("standard error")

// Wrap with enhanced error
enhanced := errors.Wrapf(stdErr, "enhanced context")

// Wrap returns nil if the parent error is nil
err := errors.Wrap(nil)                   // err is nil
err := errors.Wrapf(nil, "some message")  // err is still nil

// stdlib errors.Is / errors.As / errors.Unwrap all walk our chains,
// including through wrappers this package did not create
stderrors.Is(enhanced, stdErr)  // true
```

`Is` and `As` are re-exported so you need not import `errors` under an alias
alongside this package. One exception: a **category** is stored on an error
rather than wrapped by it, so the standard library cannot see one. Use this
package's `Is` or `CategoryOf` for categories.

What this package records survives wrappers from other packages, too.
`CategoryOf`, `CodeOf`, and `StackTrace` walk through them, so an error wrapped
by `fmt.Errorf("%w")` in some middleware keeps its category, code, and stack:

```go
err := fmt.Errorf("handler: %w", errors.NotFound.Newf("user %d not found", 1))

err.Error()                           // "handler: user 1 not found"
errors.CategoryOf(err).StatusCode()   // 404
errors.Is(err, errors.NotFound)       // true
```

## Quick Start

### Creating Simple Errors

```go
// Simple error with message
err := errors.Newf("user not found")

// Category as error directly
err := errors.NotFound

// Using category shorthand
err := errors.NotFound.Newf("user %d not found", userID)
```

### Wrapping Errors

```go
// Direct wrap: adds a stack trace when err does not carry one already
if err != nil {
    return errors.Wrap(err)
}

// Simple wrap with message
err := errors.Wrapf(originalErr, "processing failed for user %d", userID)

// Wrap with category
err := errors.Internal.Wrapf(dbErr, "database operation failed")
```

### Always Give an Error a Message

**Never write `errors.Newf("")` or `errors.Wrapf(err, "")`.** Both compile, both
look harmless, and both destroy information.

```go
err := errors.Newf("")
err != nil      // true  -- the caller takes its error branch
err.Error()     // ""    -- and whatever logs it records nothing
```

An error with no message reports that something failed while withholding what.
It also corrupts `Combine`, which joins messages with `"; "`:

```go
errors.Combine(errors.Newf(""), errors.Newf("real")).Error()  // "; real"
```

`Wrapf(err, "")` is a different trap. `Wrapf` always applies an option, so it
appends a layer with no message: the result prints the same as `err`, but its
chain gains a node no reader can see. When you have no context to add, call
`Wrap(err)`, which adds no message. When you have context, say it.

### Never Compare Errors with `==`

Test errors with `Is` (or the standard library's `errors.Is`), never with `==`.
An error stops being `==` to itself the moment anything wraps it, and this
package makes no promise that a function returns the very error it was given:
`Wrap(err)`, for example, may return a new layer.

```go
if err == ErrNotFound {             // wrong: fails once err is wrapped
if errors.Is(err, ErrNotFound) {    // right
```

The [errorlint](https://github.com/polyfloyd/go-errorlint) linter, available in
golangci-lint, reports comparisons with `==`. The rule applies to errors, not to
category values, which are constants: comparing a category with `==` or in a
`switch` is fine.

```go
switch errors.CategoryOf(err) {
case errors.NotFound:
    // ...
}
```

### Checking Errors

`Is` performs one of two searches, chosen by the type of the target.

```go
// Target is a Category: resolves the error's single category.
// The outermost category in the chain wins.
if errors.Is(err, errors.NotFound) {
    // Handle not found error
}

// Target is any other error: searches the whole chain, via errors.Is.
if errors.Is(err, originalErr) {
    // Handle case where originalErr is in the chain
}
```

To read a category or a code rather than test for one, resolve it. These work on
any error, whichever package created or wrapped it:

```go
status := errors.CategoryOf(err).StatusCode()  // Internal (500) if err carries no category
code, details := errors.CodeOf(err)            // "" and nil if err carries no code
```

Context errors are the exception to the `Internal` default, so that a caller
who gave up is not reported as a server fault: an error that `errors.Is`
reports as `context.Canceled` resolves to `Canceled` (499), and one reported as
`context.DeadlineExceeded` to `DeadlineExceeded` (504). A category declared by
a wrapper still takes precedence.

`As` recovers a concrete error type so you can read its fields. It has nothing
to do with categories.

```go
var pe *net.ParseError
if errors.As(err, &pe) {
    fmt.Println(pe.Type)
}
```

## Error Categories

The package provides predefined categories that map to HTTP status codes:

| Category | HTTP Status | Description |
|----------|-------------|-------------|
| `BadRequest` | 400 | Invalid request parameters |
| `InvalidArgument` | 400 | An argument is invalid |
| `Unauthorized` | 401 | Authentication required |
| `Forbidden` | 403 | Permission denied |
| `PermissionDenied` | 403 | The caller lacks permission for the operation |
| `NotFound` | 404 | Resource not found |
| `Conflict` | 409 | Resource conflict |
| `AlreadyExist` | 409 | The resource already exists |
| `TooManyRequests` | 429 | Rate limit exceeded |
| `ResourceExhausted` | 429 | A quota or resource limit was exhausted |
| `Canceled` | 499 | The operation was canceled, typically by the caller; also the category of `context.Canceled` |
| `Internal` | 500 | Internal server error; also the category of an error that carries none, other than the context errors |
| `DBFailed` | 500 | A database operation failed |
| `Combined` | 500 | An undeclared result of `Combine`; see [Combining Errors](#combining-errors) |
| `NotImplemented` | 501 | Feature not implemented |
| `Unavailable` | 503 | Service unavailable |
| `DeadlineExceeded` | 504 | The operation timed out; also the category of `context.DeadlineExceeded` |

## Advanced Usage

### Creating Errors with Codes

```go
// Error with category and code
err := errors.BadRequest.New(
    errors.WithCode("INVALID_EMAIL", map[string]string{"field": "email"}),
    errors.WithMessage("invalid email: %s", email),
)

// Wrap with additional options
err := errors.Wrap(originalErr, errors.WithMessage("failed to process user"))
```

### Error Inspection

```go
// Create structured error
err := errors.BadRequest.New(
    errors.WithCode("VALIDATION_FAILED", map[string]string{
        "field": "email",
        "type": "format",
    }),
    errors.WithMessage("email validation failed: %s", email),
)

// Check error properties
code, details := errors.CodeOf(err)
category := errors.CategoryOf(err)

fmt.Printf("Code: %s, Status: %d\n", code, category.StatusCode())
```

### Responding to Clients

Build a response from the category, code, and details, which are meant to be
read by programs. Do not send `Error()`: it carries every message in the chain,
including raw messages from inner errors such as a database driver's.

```go
status := errors.CategoryOf(err).StatusCode()
code, details := errors.CodeOf(err)
```

### Absorbing Errors from Other Services

When you have dealt with an error, typically one from another service, absorb
it with `Handled`, so that nothing about it leaks into your own handling. The
result keeps the message, but `errors.Is` and `errors.As` no longer see the
original error, and it resolves to `Internal` with no code. Without it, a
`NotFound` from the users service would become your own 404. Wrap the result
with a category to say what the failure means to you:

```go
user, err := usersClient.Get(ctx, id)
if err != nil {
    return errors.Unavailable.Wrapf(errors.Handled(err), "call users service")
}
```

The stack trace is kept for diagnosis: `StackTrace` sees through the barrier,
and `%+v` prints the absorbed error in full.

### Error Chaining and Traversal

```go
// Create error chain
dbErr := errors.Internal.Newf("connection failed")
serviceErr := errors.Wrapf(dbErr, "user service error")
apiErr := errors.BadRequest.Wrapf(serviceErr, "API request failed")

// Navigate the chain
rootCause := apiErr.(errors.Error).RootCause()  // Gets dbErr
chain := apiErr.(errors.Error).Chain()          // Gets [apiErr, serviceErr, dbErr]

// Check for specific errors
if errors.Is(apiErr, dbErr) {
    // Handle database-related error
}
```

`RootCause` and `Chain` follow this package's layers only, and stop at a wrapper
from another package. To see through such wrappers, use the package-level
functions or the standard library's `errors.Unwrap`.

### Combining Errors

`Combine` reports that an operation failed **as a whole** for several reasons,
such as a request with more than one invalid field. Declare what the failure is
by wrapping the result with a category; the outermost category wins:

```go
var fieldErrs []error
// ... validate each field, appending a BadRequest error for each problem

if err := errors.Combine(fieldErrs...); err != nil {
    return errors.BadRequest.Wrapf(err, "invalid request")
}
```

`Combine` returns nil when every argument is nil, and the error itself when only
one is not.

**An undeclared result is like an unlabelled archive.** Its category is
`Combined` (500), even when every reason has the same category, because nothing
about the operation can be judged until the archive is unpacked. `Combined` is
distinct from `Internal`, so a handler can recognise it. Unpack it with `Any`,
`All`, or `ErrorsOf`:

```go
a := errors.BadRequest.Newf("email is required")
b := errors.BadRequest.Newf("password is too short")
err := errors.Combine(a, b)

errors.Is(err, errors.Combined)     // true
errors.Is(err, errors.BadRequest)   // false: the archive as a whole is not judged
errors.All(err, errors.BadRequest)  // true: every reason is a BadRequest
errors.Any(err, errors.NotFound)    // false: no reason is a NotFound

for _, reason := range errors.ErrorsOf(err) {
    code, details := errors.CodeOf(reason)
    // ... report each reason, for example as a field violation
}
```

`Any` and `All` judge each reason as `Is` judges a single error: against a
category by the category `CategoryOf` resolves it to, so a reason without one
usually counts as `Internal`, and against any other error with `errors.Is`. Both return false for
a nil error. Their judgement is about the reasons, so wrapping the result with a
category does not change it.

Only categories are sealed inside the archive. The result implements
`Unwrap() []error`, so `errors.Is`, `errors.As`, and `Is` with a sentinel error
see every reason, as they do for `errors.Join`.

Its `Error()` joins the reasons' messages with `"; "` for reading in logs; do not
parse it, use `ErrorsOf`. Its stack trace points at the `Combine` call, and `%+v`
prints each reason after it.

Trees of errors from other packages, such as `errors.Join` or `fmt.Errorf` with
several `%w` verbs, are treated the same way. To give an error a category, wrap
it with the category, as in `errors.NotFound.Wrap(err)`, rather than adding the
category as an extra `%w` operand.

**Do not combine partial failures.** When some items of a batch succeed and
others fail, handle each item where it happens, or return a result per item. A
combined error keeps no record of which item each reason belongs to.

## Formatting and Output

### Stack Traces

Stack traces are captured automatically when creating a new error, and when
wrapping an error that does not already carry one. Only the `%+v` verb prints
the stack trace; `%v`, `%s`, `%q`, and `%m` print messages only.

```go
err := errors.Newf("something went wrong")
fmt.Printf("%+v", err)  // message, then the full stack trace
fmt.Printf("%v", err)   // message only
```

Because `%v` prints the message only, wrapping one of these errors with
`fmt.Errorf("%w")` produces a clean message. To recover the stack trace after
such a wrapper, use `StackTrace`, which returns the stack of the deepest error
that carries one, where the failure began:

```go
frames := errors.StackTrace(fmt.Errorf("handler: %w", err))
```

A package-level sentinel such as `var ErrNotFound = errors.NotFound.Newf("not found")`
is created while the program initialises, so its own stack trace is of no use.
Wrapping it captures a fresh stack trace at the wrap site instead, and
`errors.Is(errors.Wrap(ErrNotFound), ErrNotFound)` remains true.

Errors also implement `StackFrames() []uintptr`, which error reporters such as
Sentry look for, and `StackTrace() []runtime.Frame`.

### Format Options

```go
err := errors.Internal.New(
    errors.WithCode("DB_ERROR", map[string]string{"table": "users"}),
    errors.WithMessage("database operation failed"),
)

// Different format options
fmt.Printf("%s", err)   // "database operation failed" (whole chain, joined by ": ")
fmt.Printf("%v", err)   // "database operation failed" (same as %s)
fmt.Printf("%q", err)   // "\"database operation failed\""
fmt.Printf("%m", err)   // "database operation failed" (outermost message only)
fmt.Printf("%+v", err)  // "{DB_ERROR:table=users}database operation failed\n[stack trace]"
```

A code given without details prints as `{DB_ERROR}`.

## Reference

### Error Interface

Errors implement the `Error` interface providing these methods:

```go
type Error interface {
    Message() string             // Outermost error message, without causes
    Error() string               // All messages in the chain, joined by ": "
    Format(f fmt.State, c rune)  // Supports the %s, %v, %+v, %q and %m verbs
    Code() (string, labels.Set)  // Outermost code and its details; see CodeOf
    Category() Category          // Resolved category; see CategoryOf
    Has(cause error) bool        // Deprecated: use Is
    Cause() error                // Direct cause
    RootCause() error            // Root cause among this package's layers
    Chain() []error              // This package's layers, outermost first
}
```

`Wrap` of a non-nil error always returns an `Error`.

### Errors Interface

A result of `Combine` holding more than one reason implements `Errors`
rather than `Error`, because a tree of errors has no single chain:

```go
type Errors interface {
    Error() string               // The reasons' messages, joined by "; "
    Format(f fmt.State, c rune)  // Supports the %s, %v, %+v, %q and %m verbs
    Category() Category          // Always Combined
    Errors() []error             // The reasons, as a copy
    Unwrap() []error             // The reasons, for errors.Is and errors.As
}
```

Like `Error`, it is lost as soon as another package wraps the error, so prefer
the package-level functions, which accept any error and see through wrappers:
`CategoryOf`, `CodeOf`, `StackTrace`, `Is`, `Any`, `All`, and `ErrorsOf`.

### Category Interface

A category classifies an error and carries an HTTP status code. Categories
compare by identity, so two created with the same name are never equal.

```go
type Category interface {
    Error() string   // The category name; a Category is itself an error
    StatusCode() int // Associated HTTP status code

    New(opts ...Option) error
    Newf(format string, args ...any) error
    Wrap(err error, opts ...Option) error
    Wrapf(err error, format string, args ...any) error
}
```

Register and look up custom categories with `NewCategory(name, statusCode)` and
`LookupCategory(name)`, which returns nil when the name is unknown.

### Options

Customize errors using these options:

- `WithCode(code, details)`: Add error code and key-value details
- `WithMessage(format, args...)`: Set formatted error message
- `WithCategory(category)`: Set error category

## What's New in v1.2.0

### Behaviour changes

Review these before upgrading:

1. **`%v` no longer prints the stack trace.** Use `%+v` to see it. As a result,
   `fmt.Errorf("%w")` no longer copies the stack trace into the message, and
   neither do `fmt.Println(err)` and `log.Printf("%v", err)`.
2. **Two status codes follow common practice.** `DeadlineExceeded` maps to 504
   instead of 408, and `ResourceExhausted` to 429 instead of 503.
3. **`Canceled` is spelled as in the standard library.** Its name is now
   `"Canceled"` instead of `"Cancelled"`, which changes what logs and JSON show;
   `LookupCategory("Cancelled")` still finds it. The misspelled variable
   `Cancaled` remains as a deprecated alias of the new `Canceled`.
4. **`StackTrace()` returns `[]runtime.Frame`**, and this package no longer
   depends on `github.com/pkg/errors`. Code that obtains stacks by asserting
   pkg/errors' `interface{ StackTrace() errors.StackTrace }` still compiles, but
   the assertion now fails and the stack is silently lost. zerolog's
   `pkgerrors.MarshalStack` is one such consumer. Sentry finds the new
   `StackFrames` method.
5. **Categories survive wrappers from other packages.** An error wrapped by
   `fmt.Errorf("%w")` resolves to its inner category instead of `Internal`.
6. **Every non-nil error has a category.** `Is(fmt.Errorf("x"), Internal)` is now
   true, as `Is(errors.Newf("x"), Internal)` already was.
7. **`Combine` returns this package's own type**, and this package no longer
   depends on `go.uber.org/multierr`. Its category is the new `Combined` rather
   than `Internal`; both map to 500. `multierr.Errors` can still split it.
8. **Wrapping a package-level sentinel captures a fresh stack trace**, so
   `Wrap(ErrSentinel)` returns a new layer. Compare errors with `Is`, never with
   `==`.
9. **Details are `github.com/xhanio/labels.Set`**, and this package no longer
   depends on `k8s.io/apimachinery`. Both types are `map[string]string`
   underneath, so map literals keep working, but a value of the Kubernetes type
   no longer compiles where details are expected, as in
   `errors.WithCode("C", k8slabels.Set{...})`. Pass a map literal, or convert
   with `labels.Set(m)`.
10. **`Is(nil, nil)` reports true**, as `errors.Is(nil, nil)` does.
11. **Context errors resolve to their own categories.** An error that
    `errors.Is` reports as `context.Canceled` resolves to `Canceled` (499), and
    one reported as `context.DeadlineExceeded` to `DeadlineExceeded` (504),
    unless a wrapper declares another category. Both used to resolve to
    `Internal` (500) through this package's wrappers, and to no category at all
    otherwise.

### Additions

- `CategoryOf`, `CodeOf`, and `StackTrace` resolve a category, a code, and a
  stack trace through wrappers from any package.
- `Any`, `All`, and `ErrorsOf` examine the reasons inside a combined error.
- `Handled`, which absorbs an error so that its sentinels, category, and code
  no longer affect your handling.
- `Combined`, the category of an undeclared combined error, and the
  `Errors` interface, which a combined error implements.
- `Canceled`, the correctly spelled category for canceled operations.
- `StackFrames`, the unresolved stack for error reporters such as Sentry.
- The `%q` verb.
