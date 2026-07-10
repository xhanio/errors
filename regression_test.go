package errors

import (
	"fmt"
	"testing"
)

// #2: Message() must not panic when there is no message and no cause.
func TestMessageNoMessageNoCause(t *testing.T) {
	err := New()
	got := err.(*base).Message()
	if got != "" {
		t.Errorf("Message() = %q, want empty string", got)
	}
}

// #3: Confirm whether the captured stack's top frame is the same for a
// directly-created error and a category-created error. If skip is off for
// one path, the top frame function names will differ.
func TestStackSkipConsistency(t *testing.T) {
	direct := New().(*base).StackTrace()
	category := Internal.New(WithMessage("boom")).(*base).StackTrace()

	if len(direct) == 0 || len(category) == 0 {
		t.Fatalf("empty stack trace: direct=%d category=%d", len(direct), len(category))
	}

	directTop := fmt.Sprintf("%n", direct[0])
	categoryTop := fmt.Sprintf("%n", category[0])
	t.Logf("direct top frame:   %s", directTop)
	t.Logf("category top frame: %s", categoryTop)

	if directTop != categoryTop {
		t.Errorf("stack top frame differs: direct=%q category=%q (skip is off for one path)", directTop, categoryTop)
	}
}
