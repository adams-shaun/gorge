//go:build dirac_mp

package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestFourSeatBodyguardChoiceLockstep plays four-seat MBC pods of Dauntless
// Bodyguard (dr-mbc-bb): the as-enters choice of another creature you
// control, asked once the Bodyguard is on the battlefield, and the
// sacrifice-cost Pump over the chosen creature, read from the sacrificed
// source's last known choice. No divergence and no kernel stop, and the run
// reaches the ask, the sacrifice and a protected creature.
func TestFourSeatBodyguardChoiceLockstep(t *testing.T) {
	deck := deckOf(16, bodyguard, append(deckOf(8, "Soul Warden"), deckOf(16, "Plains")...)...)
	reg := synthReg(t)
	set := cardsettest.Synth(t, "dr-mbc-bb")
	var lists [][]cardset.Entry
	for range 4 {
		lists = append(lists, []cardset.Entry{{Name: bodyguard, Count: 16}, {Name: "Soul Warden", Count: 8}, {Name: "Plains", Count: 16}})
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	if !bodyguardLowered(tbl) {
		t.Fatalf("dr-mbc-bb's %s is not lowered", bodyguard)
	}
	tl := &bodyguardTally{seen: map[[2]int]bool{}}
	const seeds = 12
	for seed := uint64(1); seed <= seeds; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{deck, deck},
			MoreDecks: []string{"x2", "x3"}, MoreCards: [][]string{deck, deck}, Seed: seed, PassElision: true, Surface: "full"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 3000, 30
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		g, err := NewGorgeSide(m, seed, rc.Config, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, bodyguardPolicy(seed, tl), rc)
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v\nref %q\nker %q\n%s last %s", seed, d.At, d.What, d.Field, d.Diff,
				d.RefOptions, d.KerOptions, d.Detail, d.LastSourced)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
	}
	t.Logf("%d pods: %d Bodyguard choices asked, %d sacrifices, %d creatures protected", seeds, tl.asks, tl.activated, tl.protected)
	if tl.asks < 10 || tl.activated < 5 || tl.protected < 3 {
		t.Fatalf("too little evidence: %d asks, %d sacrifices, %d protected (want 10, 5, 3)", tl.asks, tl.activated, tl.protected)
	}
}
