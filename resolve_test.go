package errors

import (
	"context"
	stderrors "errors"
	"fmt"
	"testing"

	"github.com/xhanio/labels"
)

// A category must survive wrappers from other packages, and every non-nil
// error must resolve to one, whoever created it.
func TestCategoryOf(t *testing.T) {
	nf := NotFound.Newf("user 1 not found")

	tests := []struct {
		name string
		err  error
		want Category
	}{
		{"nil", nil, nil},
		{"own error", nf, NotFound},
		{"through fmt.Errorf", fmt.Errorf("handler: %w", nf), NotFound},
		{"own wrapper over fmt.Errorf", Wrapf(fmt.Errorf("handler: %w", nf), "svc"), NotFound},
		{"bare category under fmt.Errorf", fmt.Errorf("handler: %w", NotFound), NotFound},
		{"outermost category wins", BadRequest.Wrap(fmt.Errorf("handler: %w", nf)), BadRequest},
		{"own error without category", Newf("x"), Internal},
		{"stdlib error", stderrors.New("x"), Internal},
		{"fmt.Errorf without category", fmt.Errorf("x: %w", stderrors.New("y")), Internal},
		{"errors.Join is an archive", stderrors.Join(nf, NotFound.Newf("b")), Combined},
		{"multiple %w is an archive", fmt.Errorf("%w: %w", NotFound, stderrors.New("db")), Combined},
		{"wrapped archive", fmt.Errorf("handler: %w", stderrors.Join(nf, Newf("b"))), Combined},
		{"declared archive", BadRequest.Wrap(stderrors.Join(nf, Newf("b"))), BadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CategoryOf(tt.err); got != tt.want {
				t.Fatalf("CategoryOf() = %v, want %v", got, tt.want)
			}
			if tt.want != nil && !Is(tt.err, tt.want) {
				t.Errorf("Is(err, %v) = false, want true", tt.want)
			}
			if e, ok := tt.err.(Error); ok && e.Category() != tt.want {
				t.Errorf("Error.Category() = %v, want %v", e.Category(), tt.want)
			}
		})
	}
}

// Is with a category target answers for the error as a whole: an archive is
// Combined and nothing else until it is unpacked.
func TestIsCategoryOnArchive(t *testing.T) {
	nf := NotFound.Newf("a")
	archive := stderrors.Join(nf, NotFound.Newf("b"))

	if Is(archive, NotFound) {
		t.Error("Is(archive, NotFound) = true; an archive must be unpacked before judging its members")
	}
	if Is(archive, Internal) {
		t.Error("Is(archive, Internal) = true; Combined is a category of its own")
	}
	if !Is(archive, nf) {
		t.Error("Is(archive, member) = false; sentinel matching still sees members, as errors.Is does")
	}
	if Is(nil, Internal) {
		t.Error("Is(nil, Internal) = true, want false")
	}
}

// CodeOf mirrors CategoryOf: the outermost code wins, wrappers from other
// packages do not hide it, and an archive carries none of its own.
func TestCodeOf(t *testing.T) {
	inner := BadRequest.New(WithCode("INVALID_EMAIL", labels.Set{"field": "email"}), WithMessage("bad email"))

	tests := []struct {
		name        string
		err         error
		wantCode    string
		wantDetails labels.Set
	}{
		{"nil", nil, "", nil},
		{"own error", inner, "INVALID_EMAIL", labels.Set{"field": "email"}},
		{"through fmt.Errorf", fmt.Errorf("handler: %w", inner), "INVALID_EMAIL", labels.Set{"field": "email"}},
		{"outermost code wins", Wrap(fmt.Errorf("handler: %w", inner), WithCode("OUTER", labels.Set{"layer": "outer"})), "OUTER", labels.Set{"layer": "outer"}},
		{"no code", Wrapf(fmt.Errorf("x: %w", stderrors.New("y")), "z"), "", nil},
		{"archive carries no code", Wrapf(stderrors.Join(inner, Newf("b")), "validate"), "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, details := CodeOf(tt.err)
			if code != tt.wantCode || details.String() != tt.wantDetails.String() {
				t.Fatalf("CodeOf() = (%q, %v), want (%q, %v)", code, details, tt.wantCode, tt.wantDetails)
			}
			if e, ok := tt.err.(Error); ok {
				if code, details := e.Code(); code != tt.wantCode || details.String() != tt.wantDetails.String() {
					t.Errorf("Error.Code() = (%q, %v), want (%q, %v)", code, details, tt.wantCode, tt.wantDetails)
				}
			}
		})
	}
}

// MessageOf mirrors CodeOf: the outermost message set with this package wins,
// wrappers from other packages do not hide it, and neither an archive nor a
// Handled barrier carries one of its own. Unlike Error.Message, it never falls
// back to text this package did not write.
func TestMessageOf(t *testing.T) {
	foreign := stderrors.New(`pq: password authentication failed for user "admin"`)

	tests := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ""},
		{"own error", Newf("load user"), "load user"},
		{"own error without message", New(), ""},
		{"foreign error", foreign, ""},
		{"message-less wrap of a foreign error", Wrap(foreign), ""},
		{"category wrap of a foreign error", Internal.Wrap(foreign), ""},
		{"wrap of a bare category", Wrap(NotFound), ""},
		{"own message over a foreign error", Wrapf(foreign, "load user"), "load user"},
		{"message-less layer over a written one", Wrap(Wrapf(foreign, "load user")), "load user"},
		{"outermost message wins", Wrapf(Wrapf(foreign, "query users"), "load user"), "load user"},
		{"through fmt.Errorf", fmt.Errorf("handler: %w", Wrapf(foreign, "load user")), "load user"},
		{"through a barrier", Unavailable.Wrap(Handled(Wrapf(foreign, "users service"))), ""},
		{"declared barrier", Unavailable.Wrapf(Handled(foreign), "call users service"), "call users service"},
		{"archive", Combine(Newf("email is required"), Newf("password is too short")), ""},
		{"message-less wrap of an archive", Wrap(Combine(Newf("email is required"), Newf("password is too short"))), ""},
		{"declared archive", BadRequest.Wrapf(Combine(Newf("email is required"), Newf("password is too short")), "invalid request"), "invalid request"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MessageOf(tt.err); got != tt.want {
				t.Errorf("MessageOf() = %q, want %q", got, tt.want)
			}
		})
	}
}

// httpError stands for a web framework's error type that carries a
// classification of its own, as echo's *HTTPError carries a status code, and
// unwraps to the error behind it.
type httpError struct {
	code int
	err  error
}

func (e *httpError) Error() string { return "http error" }
func (e *httpError) Unwrap() error { return e.err }

// Categorized tells a category the chain declares, even Internal, from the one
// CategoryOf assumes for an error that carries none.
func TestCategorized(t *testing.T) {
	foreign := stderrors.New("x")
	nf := NotFound.Newf("user 1 not found")

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"own error with category", nf, true},
		{"bare category", NotFound, true},
		{"own error without category", Newf("x"), false},
		{"foreign error", foreign, false},
		{"message-less wrap of a foreign error", Wrap(foreign), false},
		{"declared Internal", Internal.Wrap(foreign), true},
		{"category through fmt.Errorf", fmt.Errorf("handler: %w", nf), true},
		{"category under a framework error", Wrap(&httpError{code: 401, err: nf}), true},
		{"framework error without category", Wrap(&httpError{code: 400, err: foreign}), false},
		{"context error", Wrap(context.Canceled), false},
		{"declared over a context error", Unavailable.Wrap(context.DeadlineExceeded), true},
		{"archive", Combine(Newf("a"), Newf("b")), true},
		{"foreign tree", stderrors.Join(foreign, Newf("b")), true},
		{"barrier", Handled(nf), false},
		{"declared barrier", Unavailable.Wrap(Handled(nf)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Categorized(tt.err); got != tt.want {
				t.Errorf("Categorized() = %v, want %v", got, tt.want)
			}
		})
	}
}

// An adapter honours a framework's own classification only where nothing
// declared a category, so a declared Internal is never overridden.
func TestCategorizedAdapter(t *testing.T) {
	status := func(err error) int {
		var he *httpError
		if !Categorized(err) && As(err, &he) {
			return he.code
		}
		return CategoryOf(err).StatusCode()
	}
	bind := func() error { return &httpError{code: 400, err: stderrors.New("syntax error")} }

	tests := []struct {
		name string
		err  error
		want int
	}{
		{"wrapped without category", Wrap(bind()), 400},
		{"declared Internal", Internal.Wrap(bind()), 500},
		{"declared BadRequest", BadRequest.Wrapf(bind(), "bind user"), 400},
		{"absorbed", Wrap(Handled(bind())), 500},
		{"in an archive", Wrap(Combine(bind(), Newf("b"))), 500},
		{"no framework error", Wrap(stderrors.New("x")), 500},
		// Documented limitation: a category beneath the framework error counts.
		{"category beneath the framework error", Wrap(&httpError{code: 401, err: NotFound.Newf("x")}), 404},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := status(tt.err); got != tt.want {
				t.Errorf("status = %d, want %d", got, tt.want)
			}
		})
	}
}

// deadlineErr reports itself as context.DeadlineExceeded through an Is method,
// as some libraries' timeout errors do.
type deadlineErr struct{}

func (deadlineErr) Error() string        { return "upstream timed out" }
func (deadlineErr) Is(target error) bool { return target == context.DeadlineExceeded }

// An error that carries no category but is a context cancellation or deadline
// resolves to Canceled or DeadlineExceeded rather than Internal, so a client
// that gave up is not reported as a server fault.
func TestCategoryOfContextErrors(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, cancelExpired := context.WithTimeout(context.Background(), -1)
	defer cancelExpired()

	tests := []struct {
		name string
		err  error
		want Category
	}{
		{"context.Canceled", context.Canceled, Canceled},
		{"ctx.Err() after cancel", canceled.Err(), Canceled},
		{"context.DeadlineExceeded", context.DeadlineExceeded, DeadlineExceeded},
		{"ctx.Err() after the deadline", expired.Err(), DeadlineExceeded},
		{"wrapped by fmt.Errorf", fmt.Errorf("query: %w", context.Canceled), Canceled},
		{"wrapped by this package", Wrapf(context.DeadlineExceeded, "call upstream"), DeadlineExceeded},
		{"matched through an Is method", deadlineErr{}, DeadlineExceeded},
		{"an outer category wins", Unavailable.Wrap(context.DeadlineExceeded), Unavailable},
		{"inside an archive", Combine(context.Canceled, Newf("x")), Combined},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CategoryOf(tt.err); got != tt.want {
				t.Errorf("CategoryOf() = %v, want %v", got, tt.want)
			}
		})
	}

	if Is(context.Canceled, Internal) {
		t.Error("Is(context.Canceled, Internal) = true, want false")
	}
	if !Any(Combine(context.Canceled, Newf("x")), Canceled) {
		t.Error("Any(archive with a cancellation, Canceled) = false, want true")
	}
}
