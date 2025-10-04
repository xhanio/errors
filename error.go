package errors

func newError(opts ...Option) error {
	b := &base{
		stack: callers(),
	}
	return b.apply(opts...)
}

func wrapError(err error, opts ...Option) error {
	if err == nil {
		return nil
	}
	_, ok := err.(*base)
	if ok && len(opts) == 0 {
		return err
	}
	b := &base{
		cause: err,
	}
	if !ok {
		b.stack = callers()
	}
	return b.apply(opts...)
}
