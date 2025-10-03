package errors

import (
	"fmt"

	"k8s.io/apimachinery/pkg/labels"
)

type Option func(*base)

func WithCode(code string, details labels.Set) Option {
	return func(b *base) {
		b.code = code
		b.details = details
	}
}

func WithMessage(format string, args ...any) Option {
	return func(b *base) {
		b.message = fmt.Sprintf(format, args...)
	}
}

func WithCategory(category Category) Option {
	return func(b *base) {
		b.category = category
	}
}
