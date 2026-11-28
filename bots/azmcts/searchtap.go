package azmcts

// SetSearchTap installs fn, which receives every search DecideSearch runs:
// its root, the options exactly as run (the per-decision seed and budget
// among them) and the result, a refused redeal counted. A cross-engine mirror
// reads the search through it; fn must only read, so the seat's answers are
// unchanged. nil is off.
func (s *Seat) SetSearchTap(fn func(Root, Options, Result)) { s.tap = fn }

func (s *Seat) tapSearch(root Root, opts Options, res Result) {
	if s.tap != nil {
		s.tap(root, opts, res)
	}
}
