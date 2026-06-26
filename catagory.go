package errors

import "sync"

var _ Category = (*errCategory)(nil)

var categories = &errCategories{
	data: make(map[string]Category),
}

type errCategories struct {
	mu   sync.RWMutex
	data map[string]Category
}

type errCategory struct {
	category   string
	statusCode int
}

// newCategory creates a new errCategory instance with the given category and status code
func newCategory(category string, statusCode int) *errCategory {
	return &errCategory{category: category, statusCode: statusCode}
}

// Error returns the category of the error category, implementing the error interface
func (c *errCategory) Error() string {
	return c.category
}

// StatusCode returns the HTTP status code associated with this error category
func (c *errCategory) StatusCode() int {
	return c.statusCode
}

// New creates a new error with this category and the given options
func (c *errCategory) New(opts ...Option) error {
	if len(opts) == 0 {
		// use category as standard error
		return c
	}
	opts = append(opts, WithCategory(c))
	return newError(opts...)
}

// Newf creates a new error with this category and a formatted message
func (c *errCategory) Newf(format string, args ...any) error {
	opts := []Option{
		WithCategory(c),
		WithMessage(format, args...),
	}
	return newError(opts...)
}

// Wrap wraps an existing error with this category and the given options
func (c *errCategory) Wrap(err error, opts ...Option) error {
	opts = append(opts, WithCategory(c))
	return wrapError(err, opts...)
}

// Wrapf wraps an existing error with this category and a formatted message
func (c *errCategory) Wrapf(err error, format string, args ...any) error {
	opts := []Option{
		WithCategory(c),
		WithMessage(format, args...),
	}
	return wrapError(err, opts...)
}
