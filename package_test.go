package errors

import (
	"errors"
	"testing"
)

func TestCombine(t *testing.T) {
	tests := []struct {
		name string
		errs []error
		want string
	}{
		{
			name: "no errors",
			errs: []error{},
			want: "",
		},
		{
			name: "single error",
			errs: []error{Newf("error1")},
			want: "error1",
		},
		{
			name: "multiple errors",
			errs: []error{Newf("error1"), Newf("error2")},
			want: "error1; error2",
		},
		{
			name: "with nil errors",
			errs: []error{Newf("error1"), nil, Newf("error2")},
			want: "error1; error2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Combine(tt.errs...)
			if tt.want == "" {
				if got != nil {
					t.Errorf("Combine() = %v, want nil", got)
				}
			} else {
				if got == nil || got.Error() != tt.want {
					t.Errorf("Combine() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
		want string
	}{
		{
			name: "empty error",
			opts: []Option{},
			want: "",
		},
		{
			name: "error with message",
			opts: []Option{WithMessage("test error")},
			want: "test error",
		},
		{
			name: "error with code",
			opts: []Option{WithCode("TEST001", map[string]string{"type": "test"})},
			want: "",
		},
		{
			name: "error with message and code",
			opts: []Option{
				WithMessage("test error"),
				WithCode("TEST001", map[string]string{"type": "test"}),
			},
			want: "test error",
		},
		{
			name: "error with category",
			opts: []Option{WithCategory(BadRequest)},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New(tt.opts...)
			if err.Error() != tt.want {
				t.Errorf("New() = %v, want %v", err.Error(), tt.want)
			}

			// verify it has stack trace
			if eb, ok := err.(*base); ok {
				if eb.stack == nil {
					t.Errorf("New() error should have stack trace")
				}
			}
		})
	}
}

func TestNewf(t *testing.T) {
	tests := []struct {
		name   string
		format string
		args   []any
		want   string
	}{
		{
			name:   "simple message",
			format: "test error",
			args:   []any{},
			want:   "test error",
		},
		{
			name:   "formatted message",
			format: "error: %s %d",
			args:   []any{"test", 123},
			want:   "error: test 123",
		},
		{
			name:   "empty format",
			format: "",
			args:   []any{},
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Newf(tt.format, tt.args...)
			if err.Error() != tt.want {
				t.Errorf("Newf() = %v, want %v", err.Error(), tt.want)
			}

			// verify it has stack trace
			if eb, ok := err.(*base); ok {
				if eb.stack == nil {
					t.Errorf("Newf() error should have stack trace")
				}
			}
		})
	}
}

func TestWrap(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		opts    []Option
		want    string
		wantNil bool
	}{
		{
			name:    "wrap nil error",
			err:     nil,
			opts:    []Option{},
			wantNil: true,
		},
		{
			name: "wrap standard error",
			err:  errors.New("original error"),
			opts: []Option{WithMessage("wrapped")},
			want: "wrapped: original error",
		},
		{
			name: "wrap without options",
			err:  errors.New("original error"),
			opts: []Option{},
			want: "original error",
		},
		{
			name: "wrap base without options returns same error",
			err:  New(WithMessage("base error")),
			opts: []Option{},
			want: "base error",
		},
		{
			name: "wrap base with options",
			err:  New(WithMessage("base error")),
			opts: []Option{WithMessage("wrapped")},
			want: "wrapped: base error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Wrap(tt.err, tt.opts...)
			if tt.wantNil {
				if got != nil {
					t.Errorf("Wrap() = %v, want nil", got)
				}
				return
			}

			if got.Error() != tt.want {
				t.Errorf("Wrap() = %v, want %v", got.Error(), tt.want)
			}

			// verify cause is set correctly
			if eb, ok := got.(*base); ok {
				// when wrapping base without options, it returns the same error, so no cause
				if tt.name != "wrap base without options returns same error" && eb.cause != tt.err {
					t.Errorf("Wrap() cause = %v, want %v", eb.cause, tt.err)
				}

				// verify stack trace behavior
				_, isErrBase := tt.err.(*base)
				if !isErrBase && eb.stack == nil {
					t.Errorf("Wrap() should have stack trace for non-base errors")
				}
			}
		})
	}
}

func TestWrapf(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		format  string
		args    []any
		want    string
		wantNil bool
	}{
		{
			name:    "wrap nil error",
			err:     nil,
			format:  "wrapped",
			args:    []any{},
			wantNil: true,
		},
		{
			name:   "wrap standard error",
			err:    errors.New("original error"),
			format: "wrapped: %s",
			args:   []any{"test"},
			want:   "wrapped: test: original error",
		},
		{
			name:   "wrap with empty format",
			err:    errors.New("original error"),
			format: "",
			args:   []any{},
			want:   "original error",
		},
		{
			name:   "wrap base with empty format returns same error",
			err:    New(WithMessage("base error")),
			format: "",
			args:   []any{},
			want:   "base error",
		},
		{
			name:   "wrap base with format",
			err:    New(WithMessage("base error")),
			format: "wrapped: %d",
			args:   []any{123},
			want:   "wrapped: 123: base error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Wrapf(tt.err, tt.format, tt.args...)
			if tt.wantNil {
				if got != nil {
					t.Errorf("Wrapf() = %v, want nil", got)
				}
				return
			}

			if got.Error() != tt.want {
				t.Errorf("Wrapf() = %v, want %v", got.Error(), tt.want)
			}

			// verify cause is set correctly
			if eb, ok := got.(*base); ok {
				// when wrapping base with empty format, it returns the same error, so no cause
				if tt.name != "wrap base with empty format returns same error" && eb.cause != tt.err {
					t.Errorf("Wrapf() cause = %v, want %v", eb.cause, tt.err)
				}
			}
		})
	}
}

// TestHas covers the deprecated Has. It must answer exactly as it did before
// deprecation, so it delegates to errors.Is rather than to this package's Is,
// whose Category branch would give a different answer.
func TestHas(t *testing.T) {
	cause1 := errors.New("cause1")
	cause2 := errors.New("cause2")
	baseErr := New(WithMessage("base error"))

	tests := []struct {
		name   string
		err    error
		target error
		want   bool
	}{
		{name: "non-Error err, unrelated target", err: errors.New("standard error"), target: cause1, want: false},
		{name: "matching cause", err: Wrap(cause1, WithMessage("wrapper")), target: cause1, want: true},
		{name: "non-matching cause", err: Wrap(cause1, WithMessage("wrapper")), target: cause2, want: false},
		{name: "checking self", err: baseErr, target: baseErr, want: true},
		{name: "nested cause", err: Wrapf(Wrap(cause1, WithMessage("mid")), "outer"), target: cause1, want: true},
		// Has must not resolve categories: only a category value present in the
		// tree matches. Is(NotFound.Newf("x"), NotFound) is true; Has is not.
		{name: "category is not resolved", err: NotFound.Newf("gone"), target: error(NotFound), want: false},
		{name: "category value in tree", err: Wrapf(NotFound.New(), "outer"), target: error(NotFound), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Has(tt.err, tt.target); got != tt.want {
				t.Errorf("Has() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIs(t *testing.T) {
	tests := []struct {
		name string
		err  error
		cat  Category
		want bool
	}{
		{
			name: "non-Error interface returns false",
			err:  errors.New("standard error"),
			cat:  BadRequest,
			want: false,
		},
		{
			name: "Error interface with matching category",
			err:  New(WithCategory(BadRequest)),
			cat:  BadRequest,
			want: true,
		},
		{
			name: "Error interface with different category",
			err:  New(WithCategory(BadRequest)),
			cat:  NotFound,
			want: false,
		},
		{
			name: "Error interface with default category",
			err:  New(WithMessage("no category")),
			cat:  Internal,
			want: true,
		},
		{
			// Category.New() with no options returns the category itself,
			// which is not an Error. Is must still match it.
			name: "bare category matches itself",
			err:  NotFound.New(),
			cat:  NotFound,
			want: true,
		},
		{
			// Guards the identity comparison: an implementation that asserts
			// the Category type without comparing the target passes every
			// other row in this table.
			name: "bare category against different category",
			err:  NotFound.New(),
			cat:  Internal,
			want: false,
		},
		{
			name: "category value used directly as an error",
			err:  error(NotFound),
			cat:  NotFound,
			want: true,
		},
		{
			name: "wrapped bare category resolves through the chain",
			err:  Wrapf(NotFound.New(), "outer"),
			cat:  NotFound,
			want: true,
		},
		{
			name: "nil error",
			err:  nil,
			cat:  NotFound,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Is(tt.err, tt.cat)
			if got != tt.want {
				t.Errorf("Is() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsSentinel covers Is's non-Category branch, which delegates to the
// standard library and is not exercised by TestIs.
func TestIsSentinel(t *testing.T) {
	root := errors.New("root")
	other := errors.New("other")

	tests := []struct {
		name   string
		err    error
		target error
		want   bool
	}{
		{name: "sentinel through chain", err: Wrapf(root, "outer"), target: root, want: true},
		{name: "sentinel not in chain", err: Wrapf(root, "outer"), target: other, want: false},
		{name: "sentinel is the error itself", err: root, target: root, want: true},
		{name: "nil target", err: Wrapf(root, "outer"), target: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Is(tt.err, tt.target); got != tt.want {
				t.Errorf("Is() = %v, want %v", got, tt.want)
			}
		})
	}
}

// customError is a target type for As() lookups.
type customError struct {
	id int
}

func (c *customError) Error() string { return "custom error" }

// otherError is never placed in a chain, so As() must never find it.
type otherError struct{}

func (o *otherError) Error() string { return "other error" }

func TestUnwrap(t *testing.T) {
	cause := errors.New("cause")

	tests := []struct {
		name string
		err  error
		want error
	}{
		{
			name: "no cause returns nil",
			err:  New(WithMessage("standalone")),
			want: nil,
		},
		{
			name: "wrapped standard error returns cause",
			err:  Wrap(cause, WithMessage("wrapper")),
			want: cause,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// exercise the method directly and through the stdlib
			if got := tt.err.(*base).Unwrap(); got != tt.want {
				t.Errorf("Unwrap() = %v, want %v", got, tt.want)
			}
			if got := errors.Unwrap(tt.err); got != tt.want {
				t.Errorf("errors.Unwrap() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestUnwrapWalksFullChain verifies repeated unwrapping reaches the root cause,
// which is what allows As() to inspect every error in the chain.
func TestUnwrapWalksFullChain(t *testing.T) {
	root := errors.New("root")
	err := Wrapf(Internal.Wrap(root, WithMessage("middle")), "outer")

	var depth int
	curr := err
	for curr != nil {
		if curr == root {
			return
		}
		curr = errors.Unwrap(curr)
		depth++
		if depth > 10 {
			t.Fatalf("Unwrap() did not terminate after %d hops", depth)
		}
	}
	t.Fatalf("Unwrap() never reached root cause after %d hops", depth)
}

func TestAs(t *testing.T) {
	target := &customError{id: 42}

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "target is the error itself",
			err:  target,
			want: true,
		},
		{
			name: "target wrapped once",
			err:  Wrap(target, WithMessage("wrapper")),
			want: true,
		},
		{
			name: "target wrapped through multiple layers",
			err:  Wrapf(Internal.Wrap(target, WithMessage("middle")), "outer"),
			want: true,
		},
		{
			name: "target behind a category wrap",
			err:  NotFound.Wrapf(target, "not found"),
			want: true,
		},
		{
			name: "target not in chain",
			err:  Wrapf(New(WithMessage("base")), "outer"),
			want: false,
		},
		{
			name: "standard error chain without target",
			err:  Wrap(errors.New("standard"), WithMessage("wrapper")),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *customError
			if ok := As(tt.err, &got); ok != tt.want {
				t.Fatalf("As() = %v, want %v", ok, tt.want)
			}
			if !tt.want {
				if got != nil {
					t.Errorf("As() set target to %v on failed match, want nil", got)
				}
				return
			}
			if got != target {
				t.Errorf("As() target = %v, want %v", got, target)
			}
			if got.id != 42 {
				t.Errorf("As() target.id = %d, want 42", got.id)
			}

			// As() must agree with the stdlib it delegates to
			var std *customError
			if ok := errors.As(tt.err, &std); ok != tt.want {
				t.Errorf("errors.As() = %v, want %v", ok, tt.want)
			}
		})
	}
}

// TestAsNeverMatchesAbsentType guards against As() reporting a false positive
// for a type that was never put in the chain.
func TestAsNeverMatchesAbsentType(t *testing.T) {
	err := Wrapf(Internal.Wrap(&customError{id: 1}, WithMessage("middle")), "outer")

	var absent *otherError
	if As(err, &absent) {
		t.Errorf("As() matched *otherError, which is not in the chain")
	}
}

// TestAsIntoErrorInterface verifies As() can extract this package's Error
// interface out of a chain that starts with a standard library error.
func TestAsIntoErrorInterface(t *testing.T) {
	err := Wrap(errors.New("root"), WithMessage("wrapper"))

	var target Error
	if !As(err, &target) {
		t.Fatal("As() did not find Error in chain")
	}
	if target.Message() != "wrapper" {
		t.Errorf("As() target.Message() = %q, want %q", target.Message(), "wrapper")
	}
	if errors.Unwrap(target) == nil {
		t.Error("errors.Unwrap(As target) = nil, want root cause")
	}
}

func TestMessage(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "nil error",
			err:  nil,
			want: "",
		},
		{
			name: "standard error",
			err:  errors.New("standard error"),
			want: "standard error",
		},
		{
			name: "custom error",
			err:  New(WithMessage("custom error")),
			want: "custom error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Message(tt.err)
			if got != tt.want {
				t.Errorf("Message() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCategoryMatch(t *testing.T) {
	match := NotFound.New() == NotFound
	if !match {
		t.Fatal(match)
	}
}
