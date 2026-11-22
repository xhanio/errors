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
