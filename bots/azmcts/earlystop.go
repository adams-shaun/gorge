package azmcts

// GapStop is the exact early stop (Options.StopEarly): it ends a search once
// the most-visited root child can no longer be overtaken. Every attempted
// simulation adds at most one visit to one root child, so when the leader's
// visits exceed the runner-up's by more than the Sims-attempted simulations
// left, no run of them can make the runner-up reach, or tie, the leader: the
// argmax of the full search is already decided, and stopping changes only the
// cost and the recorded visits. It is exact only when the move is that
// argmax; the seat drops it when the move is sampled or the root is noised
// (exactStop).
type GapStop struct {
	// Sims is the search's simulation budget, Options.Sims. The seat rebinds
	// it to each decision's own budget (exactStop).
	Sims int
}

// Stop reports whether the leader's visits minus the runner-up's exceed the
// simulations left.
func (g *GapStop) Stop(visits []int, _, attempted int) bool {
	first, second := 0, 0
	for _, n := range visits {
		if n > first {
			first, second = n, first
		} else if n > second {
			second = n
		}
	}
	return first-second > g.Sims-attempted
}

// exactStop keeps opts.StopEarly only where a GapStop is exact for this
// decision: it is cleared when the move is sampled (Sample, Temp, Second) or
// the root prior is noised, and a GapStop is rebound to opts.Sims, the
// decision's own budget (SimsByKind, a cheap or decided search, an overlaid
// -az-sims), since a smaller Sims than the search runs would stop a search
// whose leader can still be overtaken.
func exactStop(opts *Options) {
	if opts.StopEarly == nil {
		return
	}
	if opts.Sample || opts.Temp > 0 || opts.Second > 0 || opts.Noise {
		opts.StopEarly = nil
		return
	}
	if g, ok := opts.StopEarly.(*GapStop); ok && g.Sims != opts.Sims {
		opts.StopEarly = &GapStop{Sims: opts.Sims}
	}
}
