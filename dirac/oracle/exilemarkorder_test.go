package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Two Agate Assaults on one Tempest Angler: each is its own exile-instead
// replacement, so the death poses the CR 616.1 ask, naming each spell.
func TestExileMarksOrderedAtDeath(t *testing.T) {
	t.Parallel()
	c, pool := testPoolCorpus(t, "blb")
	set, err := cardset.ByName("blb")
	if err != nil {
		t.Fatal(err)
	}
	p := PairingAt(pool, 507, 1)
	m, tbl, err := SetMatchup(set, c, p.A, p.B, 1)
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
	if res.Stall != "" {
		t.Errorf("stalled: %s", res.Stall)
	}
}
