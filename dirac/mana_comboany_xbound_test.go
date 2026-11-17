package dirac

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// comboAnyXProbe is a board holding Sage of the Maze ("{T}: Add two mana in
// any combination of colors") and nothing else: its Combo Any production is a
// census gap (gfGapOther), so the planner prices no unit for it while gorge's
// potential walk prices it unbounded in every colour (producedOpenSet).
type comboAnyXProbe struct {
	sc   *scen
	tbl  *kop.Table
	lib  CardID
	sage CardID
	sl   Slot
}

func comboAnyXProbeBoard(t *testing.T) comboAnyXProbe {
	t.Helper()
	const sage = "Sage of the Maze"
	set := cardsettest.Synth(t, fmt.Sprintf("tail-%02d", cardset.TailBucketOf(sage)))
	deck := []cardset.Entry{{Name: sage, Count: 1}, {Name: "Plains", Count: 39}}
	tbl, ids, _, err := set.Matchup(deck, []cardset.Entry{{Name: "Plains", Count: 40}})
	if err != nil {
		t.Fatal(err)
	}
	var p comboAnyXProbe
	p.tbl = tbl
	for _, c := range ids {
		switch tbl.Cards[c].Name {
		case sage:
			p.sage = c
		case "Plains":
			p.lib = c
		}
	}
	if p.sage == NoCard || p.lib == NoCard {
		t.Fatalf("the probe cards are missing from the table")
	}
	p.sc = newScenTableLib(t, tbl, p.lib, Config{NoElide: true})
	p.sl = p.sc.slot(p.sc.perm(p.sage, 0))
	p.sc.s.perms[p.sl].flags &^= pfTapped
	return p
}

// TestComboAnyTriggerXBound: a trigger's own {X} announcement (PayManaX,
// trig_maypayx.go) bounds X by the payer's potential mana. A live Combo Any
// source is a census gap the planner prices no unit for, but the potential
// walk's further-ability fixpoint admits it as an open production
// (producedOpenSet), so an empty pool still announces X = trigXCap. A tapped
// source, and no source, leave the bound at 0.
func TestComboAnyTriggerXBound(t *testing.T) {
	p := comboAnyXProbeBoard(t)
	s, k := p.sc.s, p.sc.k
	if !k.isComboAny(p.sage) {
		t.Fatalf("the probe card is not the Combo Any census gap")
	}
	if !s.gapMask(k, 0, gfGapOther).has(p.sl) {
		t.Fatalf("the Combo Any source is not an activatable gap")
	}
	for c := 0; c < kop.NumManaKinds; c++ {
		if s.pl[0].pool[c] != 0 {
			t.Fatalf("the probe pool is not empty: colour %d holds %d", c, s.pl[0].pool[c])
		}
	}
	if s.execTrigX(k, 0, 0, trigXCap) {
		t.Fatalf("the X announcement suspended")
	}
	if got := s.g.pickLeft; got != trigXCap {
		t.Fatalf("X bound %d, want %d (a live Combo Any source is unbounded)", got, trigXCap)
	}

	// A tapped source is no source: nothing can be produced, so only X = 0.
	s.perms[p.sl].flags |= pfTapped
	s.execTrigX(k, 0, 0, trigXCap)
	if got := s.g.pickLeft; got != 0 {
		t.Fatalf("tapped source: X bound %d, want 0", got)
	}

	// The same table with no source at all: also only X = 0.
	empty := newScenTableLib(t, p.tbl, p.lib, Config{NoElide: true})
	empty.s.execTrigX(empty.k, 0, 0, trigXCap)
	if got := empty.s.g.pickLeft; got != 0 {
		t.Fatalf("no source: X bound %d, want 0", got)
	}
}
