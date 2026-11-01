package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Genesis Wave's picks enter in the order they were chosen, not library order:
// here Garruk's Uprising, then a Mountain that sat above it. Forge and gorge
// both move the chooser's list as answered.
func TestDigMovesInPickOrder(t *testing.T) {
	t.Parallel()
	c, pool := testPoolCorpus(t, "fdn")
	set, err := cardset.ByName("fdn")
	if err != nil {
		t.Fatal(err)
	}
	p := PairingAt(pool, 913, 1)
	m, tbl, err := SetMatchup(set, c, p.A, p.B, 0)
	if err != nil {
		t.Fatal(err)
	}
	var cfg Config
	k, err := SetSide(set, tbl, cfg)(m, p.Seed)
	if err != nil {
		t.Fatal(err)
	}
	res := Lockstep(m, p.Seed, k, RandomPolicy(p.Seed), RunConfig{Config: cfg, MaxDecisions: 20000, MaxTurns: 100})
	CloseSide(k)
	if res.Div != nil {
		t.Errorf("diverged at %d: %s %s %s", res.Div.At, res.Div.What, res.Div.Field, res.Div.Detail)
	}
}
