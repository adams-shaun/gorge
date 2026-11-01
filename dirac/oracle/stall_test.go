package oracle

import "testing"

// stallAfter is a kernel side that stops at its capacity after n decisions
// (State.StallReason's board stall), as the FDN kernel does when its object
// table fills.
type stallAfter struct {
	Side
	n int
}

func (s *stallAfter) Step() CDecision {
	if s.n <= 0 {
		return CDecision{Kind: DNone}
	}
	return s.Side.Step()
}

func (s *stallAfter) Apply(c Canon) { s.n--; s.Side.Apply(c) }

func (s *stallAfter) CapacityStall() string {
	if s.n <= 0 {
		return "board"
	}
	return ""
}

// TestCapacityStallIsNoDivergence: a kernel game stopped at the
// kernel's fixed capacity ends the lockstep as a "kernel board" stall, not
// as a projection or terminal divergence (the 10k slice's 36 NInst and 2
// Over first divergences were such stalls).
func TestCapacityStallIsNoDivergence(t *testing.T) {
	t.Parallel()

	c, pool := testCorpus(t)
	p := Pairings(pool, 1, 20261017)[0]
	m, err := c.Matchup(p.A, p.B, 0)
	if err != nil {
		t.Fatal(err)
	}
	k, err := GorgeKernel(Config{})(m, p.Seed)
	if err != nil {
		t.Fatal(err)
	}
	res := Lockstep(m, p.Seed, &stallAfter{Side: k, n: 20}, RandomPolicy(p.Seed), RunConfig{})
	if res.Div != nil {
		t.Fatalf("a capacity stall diverged: %v %v", res.Div, res.Div.Diff)
	}
	if res.Stall != KernelStallPrefix+"board" || res.Decisions != 20 {
		t.Fatalf("stall %q after %d decisions, want %q after 20", res.Stall, res.Decisions, KernelStallPrefix+"board")
	}
}
