package errors

import (
	"io"
	"runtime"
	"strconv"
	"sync/atomic"
)

// stack is a captured call stack.
type stack struct {
	pcs []uintptr
	// origin records whether the stack was captured while packages were being
	// initialised, as for a package-level sentinel error: such a stack ends in
	// runtime.doInit and says nothing about where a failure happened. Working
	// that out costs several symbol lookups, so it is done only when a stack is
	// wrapped or reported, and cached. Errors are shared between goroutines, so
	// the cache is atomic.
	origin atomic.Uint32
}

const (
	originUnknown uint32 = iota
	originInit
	originRun
)

// duringInit reports whether s was captured during package initialisation.
func (s *stack) duringInit() bool {
	switch s.origin.Load() {
	case originInit:
		return true
	case originRun:
		return false
	}
	init := duringInit(s.pcs)
	if init {
		s.origin.Store(originInit)
	} else {
		s.origin.Store(originRun)
	}
	return init
}

const skip = 4 // skip runtime.Callers, callers(), and the error creation function (New/Wrap/etc)

// callers captures the current stack trace and returns it as a stack pointer
func callers() *stack {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(skip, pcs[:])
	return &stack{pcs: pcs[:n]}
}

// duringInit reports whether pcs were captured during package initialisation,
// which runs under runtime.doInit (runtime.doInit1 in recent releases). Those
// frames sit at the bottom of the stack, just above runtime.main, so only the
// last few are examined. A stack deeper than callers records is cut off before
// them and is not recognised; such a stack is still reported, only not
// replaced when wrapped.
func duringInit(pcs []uintptr) bool {
	for i := len(pcs) - 1; i >= 0 && i >= len(pcs)-4; i-- {
		fn := runtime.FuncForPC(pcs[i] - 1)
		if fn == nil {
			continue
		}
		switch fn.Name() {
		case "runtime.doInit", "runtime.doInit1":
			return true
		}
	}
	return false
}

// StackTrace returns the stack trace of the deepest error in err's chain that
// carries one: where the failure began, not where it was last wrapped. It walks
// through wrappers from any package, so a stack captured by this package stays
// reachable after fmt.Errorf("%w") or similar has wrapped the error. It stops at
// a tree of errors. A stack captured during package initialisation, as for a
// package-level sentinel, is reported only when the chain holds no other.
// StackTrace returns nil if err is nil or carries no stack.
func StackTrace(err error) []runtime.Frame {
	if s := stackOf(err); s != nil {
		return s.frames()
	}
	return nil
}

// stackOf returns the stack StackTrace reports for err, or nil if there is none.
func stackOf(err error) *stack {
	usable, deepest := stacksOf(err)
	if usable != nil {
		return usable
	}
	return deepest
}

// stacksOf returns the deepest stack in err's chain that was not captured
// during package initialisation, and the deepest stack of any kind. A result of
// Combine contributes its own stack, and the walk ends there: its reasons are
// peers, none of them the place where the failure as a whole began. The walk
// passes through a barrier from Handled, which hides its error from handling
// but not from diagnosis.
func stacksOf(err error) (usable, deepest *stack) {
	for err != nil {
		var s *stack
		switch e := err.(type) {
		case *base:
			s = e.stack
		case *combined:
			s = e.stack
		case *handled:
			err = e.err
			continue
		}
		if s != nil {
			deepest = s
			if !s.duringInit() {
				usable = s
			}
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			break
		}
		err = u.Unwrap()
	}
	return usable, deepest
}

// frames resolves the stack's program counters into frames, expanding inlined
// calls into a frame each.
func (s *stack) frames() []runtime.Frame {
	if s == nil || len(s.pcs) == 0 {
		return nil
	}
	iter := runtime.CallersFrames(s.pcs)
	frames := make([]runtime.Frame, 0, len(s.pcs))
	for {
		frame, more := iter.Next()
		frames = append(frames, frame)
		if !more {
			return frames
		}
	}
}

// writeFrames writes frames in the layout pkg/errors uses for %+v, one
// "\n<function>\n\t<file>:<line>" per frame, so that output is unchanged for
// anyone parsing it.
func writeFrames(w io.Writer, frames []runtime.Frame) {
	for _, f := range frames {
		fn := f.Function
		if fn == "" {
			fn = "unknown"
		}
		io.WriteString(w, "\n"+fn+"\n\t"+f.File+":"+strconv.Itoa(f.Line))
	}
}
