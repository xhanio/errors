package errors

import (
	stderrors "errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/xhanio/labels"
)

// Any and All unpack an archive and judge each reason by the rules for a
// single error. The rows follow the example table in docs/DESIGN.md 4.4.5.
func TestAnyAllCategory(t *testing.T) {
	a, b, n := BadRequest.Newf("a"), BadRequest.Newf("b"), NotFound.Newf("n")

	tests := []struct {
		name    string
		err     error
		wantAny bool
		wantAll bool
	}{
		{"single error", a, true, true},
		{"unanimous archive", Combine(a, b), true, true},
		{"mixed archive", Combine(a, n), true, false},
		{"reason without category counts as Internal", Combine(a, io.EOF), true, false},
		{"declared archive keeps its reasons", NotFound.Wrap(Combine(a, b)), true, true},
		{"archive under fmt.Errorf", fmt.Errorf("handler: %w", Combine(a, n)), true, false},
		{"errors.Join", stderrors.Join(a, b), true, true},
		{"no reason matches", Combine(n, NotFound.Newf("m")), false, false},
		{"a nested archive is one reason, judged as a whole", Combine(Wrapf(Combine(a, n), "inner"), b), true, false},
		{"nil", nil, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Any(tt.err, BadRequest); got != tt.wantAny {
				t.Errorf("Any(err, BadRequest) = %v, want %v", got, tt.wantAny)
			}
			if got := All(tt.err, BadRequest); got != tt.wantAll {
				t.Errorf("All(err, BadRequest) = %v, want %v", got, tt.wantAll)
			}
		})
	}
}

// A reason without a category resolves to Internal, so Any can find it.
func TestAnyFindsUncategorisedReasonAsInternal(t *testing.T) {
	if !Any(Combine(BadRequest.Newf("a"), io.EOF), Internal) {
		t.Error("Any(Combine(a, io.EOF), Internal) = false, want true")
	}
}

// With a sentinel target, each reason is matched with errors.Is.
func TestAnyAllSentinel(t *testing.T) {
	oneEOF := Combine(Wrapf(io.EOF, "read header"), Newf("bad checksum"))
	twoEOF := Combine(Wrapf(io.EOF, "read header"), fmt.Errorf("read body: %w", io.EOF))

	if !Any(oneEOF, io.EOF) || All(oneEOF, io.EOF) {
		t.Errorf("one EOF reason: Any = %v, All = %v; want true, false", Any(oneEOF, io.EOF), All(oneEOF, io.EOF))
	}
	if !All(twoEOF, io.EOF) {
		t.Error("All(both reasons EOF, io.EOF) = false, want true")
	}
}

func TestErrorsOf(t *testing.T) {
	a := BadRequest.New(WithCode("INVALID_EMAIL", labels.Set{"field": "email"}), WithMessage("bad email"))
	n := NotFound.Newf("no such user")
	archive := Combine(a, n)

	if got := ErrorsOf(nil); got != nil {
		t.Errorf("ErrorsOf(nil) = %v, want nil", got)
	}
	if got := ErrorsOf(a); len(got) != 1 || got[0].Error() != "bad email" {
		t.Errorf("ErrorsOf(single) = %v, want the error itself as the only reason", got)
	}

	for name, err := range map[string]error{
		"archive":          archive,
		"declared archive": BadRequest.Wrapf(archive, "invalid request"),
		"under fmt.Errorf": fmt.Errorf("handler: %w", archive),
		"errors.Join":      stderrors.Join(a, n),
	} {
		t.Run(name, func(t *testing.T) {
			got := ErrorsOf(err)
			var msgs []string
			for _, r := range got {
				msgs = append(msgs, r.Error())
			}
			if strings.Join(msgs, "|") != "bad email|no such user" {
				t.Fatalf("ErrorsOf() messages = %q, want [bad email, no such user]", msgs)
			}
			if code, details := CodeOf(got[0]); code != "INVALID_EMAIL" || details["field"] != "email" {
				t.Errorf("CodeOf(first reason) = (%q, %v), want (INVALID_EMAIL, field=email)", code, details)
			}
		})
	}

	got := ErrorsOf(archive)
	got[0] = nil
	if ErrorsOf(archive)[0] == nil {
		t.Error("ErrorsOf returned the archive's own slice; changing the result changed the archive")
	}
}
