package errors

import (
	stderrors "errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// members returns the members of a tree of errors through the standard
// interface, so these tests do not depend on ErrorsOf.
func members(t *testing.T, err error) []error {
	t.Helper()
	tree, ok := err.(interface{ Unwrap() []error })
	if !ok {
		t.Fatalf("%T does not implement Unwrap() []error", err)
	}
	return tree.Unwrap()
}

func TestCombineNothingIsNil(t *testing.T) {
	if got := Combine(); got != nil {
		t.Errorf("Combine() = %v, want nil", got)
	}
	if got := Combine(nil, nil); got != nil {
		t.Errorf("Combine(nil, nil) = %v, want nil", got)
	}
}

// With only one reason, there is nothing to combine: the reason is the error.
func TestCombineSingleReasonIsTheError(t *testing.T) {
	got := Combine(nil, BadRequest.Newf("a"), nil)
	if got.Error() != "a" || CategoryOf(got) != BadRequest {
		t.Errorf("Combine(nil, a, nil) = %q in %v, want %q in BadRequest", got, CategoryOf(got), "a")
	}
	if _, ok := got.(interface{ Unwrap() []error }); ok {
		t.Error("Combine of a single reason returned a tree")
	}
}

// An undeclared archive is Combined even when every member agrees, and it does
// not pretend to be a single error of this package.
func TestCombineBuildsArchive(t *testing.T) {
	a, b := BadRequest.Newf("a"), BadRequest.Newf("b")
	archive := Combine(a, b)

	if got := archive.Error(); got != "a; b" {
		t.Errorf("Error() = %q, want %q", got, "a; b")
	}
	if got := CategoryOf(archive); got != Combined {
		t.Errorf("CategoryOf() = %v, want Combined", got)
	}
	if _, ok := archive.(Error); ok {
		t.Error("the archive implements Error; it must not, so that Chain and RootCause need not be defined for trees")
	}
	for _, verb := range []string{"%v", "%s"} {
		if got := fmt.Sprintf(verb, archive); got != "a; b" {
			t.Errorf("%s = %q, want %q", verb, got, "a; b")
		}
	}
	if got := fmt.Sprintf("%q", archive); got != `"a; b"` {
		t.Errorf("%%q = %q, want %q", got, `"a; b"`)
	}
	if got := fmt.Sprintf("%x", archive); got != "!%x(a; b)" {
		t.Errorf("%%x = %q, want %q", got, "!%x(a; b)")
	}
}

// Only categories are sealed inside the archive: the standard library still
// sees every member.
func TestCombineMembersVisibleToStdlib(t *testing.T) {
	sentinel := stderrors.New("sentinel")
	archive := Combine(Newf("a"), Wrapf(sentinel, "b"))

	if !stderrors.Is(archive, sentinel) || !Is(archive, sentinel) {
		t.Error("Is(archive, member sentinel) = false, want true")
	}
	var target *base
	if !stderrors.As(archive, &target) {
		t.Error("errors.As(archive, *base) = false, want true")
	}
}

func TestCombineFlattensNestedArchives(t *testing.T) {
	got := members(t, Combine(Combine(Newf("a"), Newf("b")), Newf("c")))
	var msgs []string
	for _, m := range got {
		msgs = append(msgs, m.Error())
	}
	if strings.Join(msgs, ",") != "a,b,c" {
		t.Errorf("members = %v, want [a b c]", msgs)
	}
}

// Wrapping with a category declares what the failure as a whole is; wrapping
// with only a message leaves the archive undeclared.
func TestCombineDeclaredByWrapper(t *testing.T) {
	a, n := BadRequest.Newf("a"), NotFound.Newf("n")

	declared := BadRequest.Wrapf(Combine(a, n), "invalid request")
	if got := CategoryOf(declared); got != BadRequest {
		t.Errorf("CategoryOf(declared) = %v, want BadRequest", got)
	}
	if got := declared.Error(); got != "invalid request: a; n" {
		t.Errorf("declared.Error() = %q, want %q", got, "invalid request: a; n")
	}

	annotated := Wrapf(Combine(a, n), "validate")
	if got := CategoryOf(annotated); got != Combined {
		t.Errorf("CategoryOf(annotated) = %v, want Combined", got)
	}
	if got := annotated.Error(); got != "validate: a; n" {
		t.Errorf("annotated.Error() = %q, want %q", got, "validate: a; n")
	}
}

// The archive records where the failure as a whole was established, and
// wrappers keep reporting that place rather than their own.
func TestCombineCapturesStackAtCallSite(t *testing.T) {
	a, b := Newf("a"), Newf("b")
	_, _, line, _ := runtime.Caller(0)
	archive := Combine(a, b) // must stay on the line after runtime.Caller

	const fn = "TestCombineCapturesStackAtCallSite"
	assertTopFrame(t, StackTrace(archive), fn, line+1)
	assertTopFrame(t, StackTrace(Wrapf(archive, "validate")), fn, line+1)
	assertTopFrame(t, StackTrace(fmt.Errorf("handler: %w", archive)), fn, line+1)

	// Error reporters such as Sentry read the unresolved form.
	pcs := archive.(interface{ StackFrames() []uintptr }).StackFrames()
	if len(pcs) == 0 {
		t.Fatal("StackFrames() is empty")
	}
	if first, _ := runtime.CallersFrames(pcs).Next(); first.Line != line+1 {
		t.Errorf("StackFrames() starts at line %d, want %d", first.Line, line+1)
	}
}

// %+v shows the archive's own stack and then each member in its verbose form,
// also when the archive is wrapped.
func TestCombineVerboseFormat(t *testing.T) {
	a := BadRequest.New(WithCode("E_A", nil), WithMessage("a"))
	archive := Combine(a, Newf("b"))

	for name, err := range map[string]error{
		"archive":  archive,
		"declared": BadRequest.Wrapf(archive, "invalid request"),
	} {
		t.Run(name, func(t *testing.T) {
			got := fmt.Sprintf("%+v", err)
			for _, want := range []string{"a; b\n", "TestCombineVerboseFormat", "[1/2] {E_A}a\n", "[2/2] b\n"} {
				if !strings.Contains(got, want) {
					t.Errorf("%%+v = %q, want it to contain %q", got, want)
				}
			}
		})
	}
}

// A reason wrapped by another package has no %+v of its own, so the archive
// prints that reason's stack itself rather than lose it.
func TestCombineVerboseFormatReachesStackThroughForeignWrapper(t *testing.T) {
	_, file, line, _ := runtime.Caller(0)
	inner := Newf("bad") // must stay on the line after runtime.Caller

	got := fmt.Sprintf("%+v", Combine(fmt.Errorf("field a: %w", inner), Newf("b")))
	want := "[1/2] field a: bad\n    github.com/xhanio/errors.TestCombineVerboseFormatReachesStackThroughForeignWrapper\n    \t" +
		file + ":" + strconv.Itoa(line+1) + "\n"
	if !strings.Contains(got, want) {
		t.Errorf("%%+v = %q, want it to contain %q", got, want)
	}
}

// Wrap of any non-nil error returns an Error, an archive included, and adds
// nothing a caller can observe when given no options.
func TestWrapAlwaysReturnsError(t *testing.T) {
	for name, err := range map[string]error{
		"archive":                  Combine(Newf("a"), Newf("b")),
		"stdlib error":             stderrors.New("x"),
		"own error":                Newf("x"),
		"foreign wrapper over own": fmt.Errorf("handler: %w", Newf("x")),
	} {
		t.Run(name, func(t *testing.T) {
			wrapped := Wrap(err)
			if _, ok := wrapped.(Error); !ok {
				t.Fatalf("Wrap(%T) returned %T, which does not implement Error", err, wrapped)
			}
			if wrapped.Error() != err.Error() || CategoryOf(wrapped) != CategoryOf(err) {
				t.Errorf("Wrap changed the error: %q in %v, want %q in %v",
					wrapped, CategoryOf(wrapped), err, CategoryOf(err))
			}
		})
	}
}

// The archive implements Errors, as an error from New implements Error.
func TestCombineImplementsErrors(t *testing.T) {
	a, n := BadRequest.Newf("a"), NotFound.Newf("n")

	c, ok := Combine(a, n).(Errors)
	if !ok {
		t.Fatalf("Combine(a, n) is %T, which does not implement Errors", Combine(a, n))
	}
	if got := c.Error(); got != "a; n" {
		t.Errorf("Error() = %q, want %q", got, "a; n")
	}
	if got := c.Category(); got != Combined {
		t.Errorf("Category() = %v, want Combined", got)
	}
	if got := len(c.Unwrap()); got != 2 {
		t.Errorf("len(Unwrap()) = %d, want 2", got)
	}

	reasons := c.Errors()
	if len(reasons) != 2 || reasons[0].Error() != "a" || reasons[1].Error() != "n" {
		t.Fatalf("Errors() = %v, want [a n]", reasons)
	}
	reasons[0] = nil
	if c.Errors()[0] == nil {
		t.Error("Errors() returned the archive's own slice; changing the result changed the archive")
	}

	if _, ok := Combine(a).(Errors); ok {
		t.Error("Combine of a single reason implements Errors; it should be the reason itself")
	}
}
