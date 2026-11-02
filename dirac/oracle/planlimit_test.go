package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
)

// A game with Leyline of the Guildpact in play, whose payments reach gorge's
// planner node limit: gorge pays with the best plan it had seen, and the
// kernel must stop where gorge stops.
func TestPlanLimitLockstep(t *testing.T) {
	c := &Corpus{Reg: synthReg(t), decks: map[string][]*cards.Card{}}
	set := cardsettest.Synth(t, "std-mkm")
	pool, files, err := SynthPool(c.Reg, set, 1, SynthDecks)
	if err != nil {
		t.Fatal(err)
	}
	c.AddDecks(files)
	cfg := Config{}
	p := PairingAt(pool, 314, 1)
	m, tbl, err := SetMatchup(set, c, p.A, p.B, 1)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(set, tbl, cfg)(m, p.Seed)
	if err != nil {
		t.Fatal(err)
	}
	g, err := NewGorgeSide(m, p.Seed, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	res := LockstepLed(m, p.Seed, g, k, RandomPolicy(p.Seed), RunConfig{Config: cfg, MaxDecisions: 20000, MaxTurns: 100})
	if res.Div != nil {
		t.Fatalf("divergence at decision %d: %s %s", res.Div.At, res.Div.What, res.Div.Detail)
	}
}
