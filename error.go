package errors

func newError(opts ...Option) error {
	b := &base{
		stack: callers(),
	}
	return b.apply(opts...)
}

// wrapError wraps err in a new layer configured by opts. The layer captures a
// stack only when err carries none worth reporting, so the stack points at where
// the failure began. A stack captured during package initialisation is not worth
// reporting: wrapping a package-level sentinel captures a fresh one here.
//
// The result always implements Error. Given no options, a *base that already
// carries a stack worth reporting is therefore returned as it is; anything else,
// a result of Combine included, gets a layer of its own.
func wrapError(err error, opts ...Option) error {
	if err == nil {
		return nil
	}
	usable, _ := stacksOf(err)
	hasStack := usable != nil
	if _, ok := err.(*base); ok && hasStack && len(opts) == 0 {
		return err
	}
	b := &base{
		cause: err,
	}
	if !hasStack {
		b.stack = callers()
	}
	return b.apply(opts...)
}
