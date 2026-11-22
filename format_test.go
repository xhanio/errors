package errors

import (
	"fmt"
	"strings"
	"testing"
)

// Wrapping with fmt.Errorf("%w") renders the operand with %v, so %v must not
// carry the stack trace: it would end up inside every wrapped message.
func TestFormatPlainVerbsOmitStack(t *testing.T) {
	err := NotFound.New(WithCode("E1", map[string]string{"k": "v"}), WithMessage("user 1 not found"))

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"fmt.Errorf %w", fmt.Errorf("handler: %w", err).Error(), "handler: user 1 not found"},
		{"Sprint", fmt.Sprint(err), "user 1 not found"},
		{"%v", fmt.Sprintf("%v", err), "user 1 not found"},
		{"%s", fmt.Sprintf("%s", err), "user 1 not found"},
		{"%q", fmt.Sprintf("%q", err), `"user 1 not found"`},
		{"%m", fmt.Sprintf("%m", Wrapf(err, "outer")), "outer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}

// %+v is the verbose form: code and details first, then the message, then the
// stack trace.
func TestFormatPlusVIncludesCodeAndStack(t *testing.T) {
	err := NotFound.New(WithCode("E1", map[string]string{"k": "v"}), WithMessage("user 1 not found"))

	got := fmt.Sprintf("%+v", err)
	if !strings.HasPrefix(got, "{E1:k=v}user 1 not found\n") {
		t.Errorf("%%+v = %q, want prefix %q", got, "{E1:k=v}user 1 not found\n")
	}
	if !strings.Contains(got, "TestFormatPlusVIncludesCodeAndStack") {
		t.Errorf("%%+v = %q, want the stack trace to name this test", got)
	}
}

// A code given without details prints as {code}, with no empty details after it.
func TestFormatPlusVCodeWithoutDetails(t *testing.T) {
	got := fmt.Sprintf("%+v", New(WithCode("C1", nil), WithMessage("m")))
	if !strings.HasPrefix(got, "{C1}m\n") {
		t.Errorf("%%+v = %q, want prefix %q", got, "{C1}m\n")
	}
}
