package errors

var _ Category = (*errCategory)(nil)

type errCategory struct {
	description string
	statusCode  int
}

// newCategory creates a new errCategory instance with the given description and status code
func newCategory(description string, statusCode int) *errCategory {
	return &errCategory{description: description, statusCode: statusCode}
}

// Error returns the description of the error category, implementing the error interface
func (c *errCategory) Error() string {
	return c.description
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
