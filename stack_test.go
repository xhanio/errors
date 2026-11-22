package errors

import (
	stderrors "errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// initSentinel is created while the test binary initialises its packages, so
// the stack it captures ends in runtime.doInit and says nothing useful.
var initSentinel = NotFound.Newf("sentinel")

// assertTopFrame fails unless st starts at line in the named test function.
func assertTopFrame(t *testing.T, st []runtime.Frame, fn string, line int) {
	t.Helper()
	if len(st) == 0 {
		t.Fatalf("empty stack trace, want one starting at %s line %d", fn, line)
	}
	want := "github.com/xhanio/errors." + fn
	if st[0].Function != want || st[0].Line != line {
		t.Errorf("stack starts at %s line %d, want %s line %d", st[0].Function, st[0].Line, want, line)
	}
}

// StackTrace finds where an error was created even when another package has
// wrapped it, and a wrapper does not replace that stack with its own.
func TestStackTraceThroughForeignWrappers(t *testing.T) {
	_, _, line, _ := runtime.Caller(0)
	inner := Newf("inner") // must stay on the line after runtime.Caller

	tests := []struct {
		name string
		err  error
	}{
		{"own error", inner},
		{"through fmt.Errorf", fmt.Errorf("handler: %w", inner)},
		{"own wrapper over fmt.Errorf", Wrapf(fmt.Errorf("handler: %w", inner), "svc")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertTopFrame(t, StackTrace(tt.err), "TestStackTraceThroughForeignWrappers", line+1)
			if b, ok := tt.err.(*base); ok {
				assertTopFrame(t, b.StackTrace(), "TestStackTraceThroughForeignWrappers", line+1)
			}
		})
	}

	if st := StackTrace(nil); st != nil {
		t.Errorf("StackTrace(nil) = %v, want nil", st)
	}
	if st := StackTrace(stderrors.New("std")); st != nil {
		t.Errorf("StackTrace(stdlib error) = %v, want nil", st)
	}
}

// Wrapping an error that carries no stack captures one at the wrap site.
func TestWrapCapturesStackWhenCauseHasNone(t *testing.T) {
	_, _, line, _ := runtime.Caller(0)
	err := Wrapf(stderrors.New("std"), "ctx") // must stay on the line after runtime.Caller
	assertTopFrame(t, StackTrace(err), "TestWrapCapturesStackWhenCauseHasNone", line+1)
}

// A stack captured during package initialisation does not count: wrapping a
// package-level sentinel captures a fresh stack at the wrap site.
func TestWrapSentinelRecapturesStack(t *testing.T) {
	if st := StackTrace(initSentinel); len(st) == 0 {
		t.Fatal("StackTrace(sentinel) is empty; the sentinel should keep the stack it was created with")
	}

	_, _, line, _ := runtime.Caller(0)
	withMessage := Wrapf(initSentinel, "lookup user 1") // line+1
	bare := Wrap(initSentinel)                          // line+2
	recategorised := Internal.Wrap(initSentinel)        // line+3

	for i, err := range []error{withMessage, bare, recategorised} {
		assertTopFrame(t, StackTrace(err), "TestWrapSentinelRecapturesStack", line+1+i)
		if !Is(err, initSentinel) {
			t.Errorf("Is(wrapped %d, sentinel) = false, want true", i)
		}
	}
	if got := CategoryOf(bare); got != NotFound {
		t.Errorf("CategoryOf(Wrap(sentinel)) = %v, want NotFound", got)
	}
}

// The stack printed by %+v keeps the layout pkg/errors used, and StackFrames
// exposes the same stack unresolved, for reporters such as Sentry.
func TestStackLayoutAndFrames(t *testing.T) {
	_, file, line, _ := runtime.Caller(0)
	err := Newf("boom") // must stay on the line after runtime.Caller
	b := err.(*base)

	want := "boom\ngithub.com/xhanio/errors.TestStackLayoutAndFrames\n\t" + file + ":" + strconv.Itoa(line+1) + "\n"
	if got := fmt.Sprintf("%+v", err); !strings.HasPrefix(got, want) {
		t.Errorf("%%+v = %q, want prefix %q", got, want)
	}

	pcs := b.StackFrames()
	frames := b.StackTrace()
	if len(pcs) == 0 || len(frames) == 0 {
		t.Fatalf("empty stack: StackFrames=%d StackTrace=%d", len(pcs), len(frames))
	}
	first, _ := runtime.CallersFrames(pcs).Next()
	if first.Function != frames[0].Function || first.File != frames[0].File || first.Line != frames[0].Line {
		t.Errorf("StackFrames resolves to %s %s:%d, StackTrace starts at %s %s:%d",
			first.Function, first.File, first.Line, frames[0].Function, frames[0].File, frames[0].Line)
	}

	pcs[0] = 0
	if b.StackFrames()[0] == 0 {
		t.Error("StackFrames returned the error's own slice rather than a copy")
	}

	if got := (&base{message: "no stack"}).StackFrames(); got != nil {
		t.Errorf("StackFrames() without a stack = %v, want nil", got)
	}
}

// A package-level sentinel is shared, so goroutines may wrap it at the same
// time. Run with -race: deciding whether its stack came from initialisation
// must not race.
func TestWrapSentinelConcurrently(t *testing.T) {
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if st := StackTrace(Wrap(initSentinel)); len(st) == 0 {
				t.Error("empty stack trace")
			}
		}()
	}
	wg.Wait()
}
