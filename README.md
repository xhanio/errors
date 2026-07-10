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
package's `Is` for category checks.

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
// Direct wrap (preserves existing error, adds stack trace if not already an Error)
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

`Wrapf(err, "")` is a different trap. `Wrap(err)` is a no-op on an error this
package produced, returning it unchanged. `Wrapf(err, "")` always applies an
option, so it appends a message-less layer instead:

```go
inner := errors.Newf("inner")

errors.Wrap(inner) == inner        // true  -- returns the same error
errors.Wrapf(inner, "") == inner   // false -- a new, invisible chain node
```

Both print `"inner"`, but the second lengthens the chain with a node no reader
can see. When you have no context to add, call `Wrap(err)`. When you have
context, say it.

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

`As` recovers a concrete error type so you can read its fields. It has nothing
to do with categories.

```go
var pe *net.ParseError
if errors.As(err, &pe) {
    fmt.Println(pe.Type)
}

// To read a category, resolve it rather than searching for it:
if e, ok := err.(errors.Error); ok {
    status := e.Category().StatusCode()
}
```

## Error Categories

The package provides predefined categories that map to HTTP status codes:

| Category | HTTP Status | Description |
|----------|-------------|-------------|
| `BadRequest` | 400 | Invalid request parameters |
| `Unauthorized` | 401 | Authentication required |
| `Forbidden` | 403 | Permission denied |
| `NotFound` | 404 | Resource not found |
| `Conflict` | 409 | Resource conflict |
| `TooManyRequests` | 429 | Rate limit exceeded |
| `Internal` | 500 | Internal server error |
| `NotImplemented` | 501 | Feature not implemented |
| `Unavailable` | 503 | Service unavailable |

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
code, details := err.(errors.Error).Code()
category := err.(errors.Error).Category()
message := err.(errors.Error).Message()

fmt.Printf("Code: %s, Status: %d\n", code, category.StatusCode())
```

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

### Combining Multiple Errors

```go
var errs []error
errs = append(errs, errors.Newf("error 1"))
errs = append(errs, errors.Newf("error 2"))

combined := errors.Combine(errs...)
```

## Formatting and Output

### Stack Traces

Stack traces are captured automatically when creating a new error, and when
wrapping an error that does not already carry one. The `%v` verb includes the
stack trace; `%s` and `%m` do not.

```go
err := errors.Newf("something went wrong")
fmt.Printf("%v", err)  // message, then the full stack trace
fmt.Printf("%s", err)  // message only
```

Note that `fmt.Errorf` renders a `%w` operand with `%v`, so wrapping one of
these errors with `fmt.Errorf` embeds the stack trace in the resulting message.
Prefer `errors.Wrapf`, which keeps the stack where it belongs.

### Format Options

```go
err := errors.Internal.New(
    errors.WithCode("DB_ERROR", map[string]string{"table": "users"}),
    errors.WithMessage("database operation failed"),
)

// Different format options
fmt.Printf("%s", err)   // "database operation failed" (whole chain, joined by ": ")
fmt.Printf("%m", err)   // "database operation failed" (outermost message only)
fmt.Printf("%v", err)   // "{DB_ERROR:table=users}database operation failed\n[stack trace]"
```

## Reference

### Error Interface

Errors implement the `Error` interface providing these methods:

```go
type Error interface {
    Message() string             // Outermost error message, without causes
    Error() string               // All messages in the chain, joined by ": "
    Format(f fmt.State, c rune)  // Supports the %s, %m and %v verbs
    Code() (string, labels.Set)  // Error code and details
    Category() Category          // Resolved category; Internal if none was set
    Has(cause error) bool        // Deprecated: use Is
    Cause() error                // Direct cause
    RootCause() error            // Root cause in chain
    Chain() []error              // All errors in chain
}
```

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

## What's New in v1.1.0

This release is backwards compatible. Existing code keeps compiling and
behaving as it did.

**`errors.Is` and `errors.As` from the standard library now traverse these
errors.** Errors from this package implement `Unwrap() error`, so a chain mixing
these errors with any others can be walked from either side.

**`As` is re-exported**, so you need not import the standard `errors` package
under an alias alongside this one.

**`Has` is deprecated.** Use `Is`, which performs the same search and
additionally sees through `Combine`. `Has` still works and answers exactly as
before.

```go
- if errors.Has(err, originalErr) {
+ if errors.Is(err, originalErr) {
```

**A bare category now matches itself.** `Category.New()` with no options returns
the category itself, which previously failed to match:

```go
errors.Is(errors.NotFound.New(), errors.NotFound)  // was false, now true
```
