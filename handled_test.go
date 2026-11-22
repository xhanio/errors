package errors

import (
	"context"
	stderrors "errors"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/xhanio/labels"
)

func TestHandledNil(t *testing.T) {
	if got := Handled(nil); got != nil {
		t.Errorf("Handled(nil) = %v, want nil", got)
	}
}

// A handled error keeps the message but hides everything a caller could act
// on: its sentinels, its types, its category, and its code.
func TestHandledHidesItsCause(t *testing.T) {
	noRows := stderrors.New("no rows")
	downstream := NotFound.Wrap(noRows, WithCode("USER_MISSING", labels.Set{"id": "1"}), WithMessage("user 1"))
	h := Handled(downstream)

	if got, want := h.Error(), downstream.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	if Is(h, noRows) || stderrors.Is(h, noRows) {
		t.Error("Is(handled, inner sentinel) = true; the barrier must hide it")
	}
	var b *base
	if stderrors.As(h, &b) {
		t.Error("errors.As(handled, *base) = true; the barrier must hide the inner error")
	}
	if got := CategoryOf(h); got != Internal {
		t.Errorf("CategoryOf() = %v, want Internal: the downstream NotFound must not become ours", got)
	}
	if code, details := CodeOf(h); code != "" || details != nil {
		t.Errorf("CodeOf() = (%q, %v), want no code: the downstream code must not become ours", code, details)
	}
	if got := ErrorsOf(h); len(got) != 1 {
		t.Errorf("ErrorsOf() = %v, want the handled error as its only reason", got)
	}
}

// A context error behind the barrier is absorbed too, and so is an archive:
// neither resolves to its own category any more.
func TestHandledAbsorbsContextErrorsAndArchives(t *testing.T) {
	if got := CategoryOf(Handled(context.Canceled)); got != Internal {
		t.Errorf("CategoryOf(Handled(context.Canceled)) = %v, want Internal", got)
	}

	archive := Handled(Combine(BadRequest.Newf("a"), BadRequest.Newf("b")))
	if got := CategoryOf(archive); got != Internal {
		t.Errorf("CategoryOf(Handled(archive)) = %v, want Internal", got)
	}
	if Any(archive, BadRequest) {
		t.Error("Any(Handled(archive), BadRequest) = true; the barrier must hide the reasons")
	}
}

// A wrapper outside the barrier declares the category, as anywhere else.
func TestHandledDeclaredByWrapper(t *testing.T) {
	downstream := NotFound.Newf("user 1 not found")
	err := Unavailable.Wrapf(Handled(downstream), "call users service")

	if got := CategoryOf(err); got != Unavailable {
		t.Errorf("CategoryOf() = %v, want Unavailable", got)
	}
	if got := err.Error(); got != "call users service: user 1 not found" {
		t.Errorf("Error() = %q, want %q", got, "call users service: user 1 not found")
	}
}

// The stack is for diagnosis, not handling, so it stays reachable through the
// barrier, and wrapping a handled error does not replace it.
func TestHandledKeepsStack(t *testing.T) {
	_, _, line, _ := runtime.Caller(0)
	inner := Newf("inner") // must stay on the line after runtime.Caller

	const fn = "TestHandledKeepsStack"
	assertTopFrame(t, StackTrace(Handled(inner)), fn, line+1)
	assertTopFrame(t, StackTrace(Wrapf(Handled(inner), "outer")), fn, line+1)

	pcs := Handled(inner).(interface{ StackFrames() []uintptr }).StackFrames()
	if first, _ := runtime.CallersFrames(pcs).Next(); first.Line != line+1 {
		t.Errorf("StackFrames() starts at line %d, want %d", first.Line, line+1)
	}
}

// %v prints the message only; %+v shows the absorbed error in full, for
// diagnosis.
func TestHandledFormat(t *testing.T) {
	inner := NotFound.New(WithCode("USER_MISSING", labels.Set{"id": "1"}), WithMessage("user 1 not found"))
	h := Handled(inner)

	if got := fmt.Sprintf("%v", h); got != "user 1 not found" {
		t.Errorf("%%v = %q, want %q", got, "user 1 not found")
	}
	verbose := fmt.Sprintf("%+v", h)
	for _, want := range []string{"{USER_MISSING:id=1}user 1 not found\n", "TestHandledFormat"} {
		if !strings.Contains(verbose, want) {
			t.Errorf("%%+v = %q, want it to contain %q", verbose, want)
		}
	}
}

// Handling an error twice adds no second barrier, and the barrier's own
// methods report what the package-level functions do.
func TestHandledTwiceAndMethods(t *testing.T) {
	inner := Newf("inner")
	h := Handled(inner)

	again, ok := Handled(h).(*handled)
	if !ok {
		t.Fatalf("Handled(Handled(e)) is %T, want a barrier", Handled(h))
	}
	if _, nested := again.err.(*handled); nested {
		t.Error("Handled(Handled(e)) put a barrier inside a barrier, want a single one")
	}
	if again.err != inner {
		t.Errorf("Handled(Handled(e)) wraps %v, want e itself", again.err)
	}
	if got := h.(interface{ StackTrace() []runtime.Frame }).StackTrace(); len(got) == 0 || got[0] != StackTrace(inner)[0] {
		t.Error("StackTrace() method does not report the absorbed error's stack")
	}
	if got := Handled(stderrors.New("no stack")).(interface{ StackFrames() []uintptr }).StackFrames(); got != nil {
		t.Errorf("StackFrames() without a stack = %v, want nil", got)
	}
	if got := fmt.Sprintf("%q|%s|%x", h, h, h); got != `"inner"|inner|!%x(inner)` {
		t.Errorf("formatted = %q, want %q", got, `"inner"|inner|!%x(inner)`)
	}
}
