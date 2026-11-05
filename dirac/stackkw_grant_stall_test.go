package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// A granted Convoke reaches spells kgen refuses for the printed keyword:
// one that also has Improvise, and one with an {X} cost. gorge offers a
// single mixed announcement (ConvokeAsk with Convoke and Improvise, or with
// HasX); the kernel has no such announcement, so the cast stalls unmodelled
// instead of posing one keyword's menu (kw_convoke.go convokeOptions).

// stackKwStallScen is Chief Engineer on the battlefield with spell in hand:
// Chief Engineer grants artifact spells Convoke, Kappa Cannoneer has printed
// Improvise, and Walking Ballista is an {X} artifact spell.
func stackKwStallScen(t *testing.T, spell string) (*scen, Obj) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Chief Engineer", Count: 1},
		{Name: spell, Count: 1},
		{Name: "Island", Count: 8},
	}
	set := cardsettest.Set(t, "stackgrantkw")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if _, ok := id[spell]; !ok {
		t.Fatalf("%s is not in the stackgrantkw pool", spell)
	}
	sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
	sc.lands(0, id["Island"], 8)
	sc.perm(id["Chief Engineer"], 0)
	return sc, sc.hand(id[spell], 0)
}

func stackKwStall(t *testing.T, spell string) {
	t.Helper()
	sc, m := stackKwStallScen(t, spell)
	sc.do(t, AkCast, uint8(m), 0xff)
	d := sc.s.Step()
	if !sc.s.Stalled() {
		t.Fatalf("%s: no stall; decision %v/%d options %v", spell, d.Kind, d.Sub, d.Options[:d.N])
	}
	if got := sc.s.StallReason(); got != StallStackKwGrant {
		t.Fatalf("%s: stall %q, want %q", spell, StallName(got), StallName(StallStackKwGrant))
	}
	if !StallIsUnmodelled(StallStackKwGrant) {
		t.Fatalf("%q is not classed unmodelled", StallName(StallStackKwGrant))
	}
	if d.Kind != DkNone {
		t.Fatalf("%s: posed decision %v/%d past the stall", spell, d.Kind, d.Sub)
	}
}

// TestStackKwGrantBothKeywordsStalls: Chief Engineer grants Convoke to
// Kappa Cannoneer, which has printed Improvise. gorge offers creatures and
// artifacts in one announcement; the kernel stalls.
func TestStackKwGrantBothKeywordsStalls(t *testing.T) {
	stackKwStall(t, "Kappa Cannoneer")
}

// TestStackKwGrantXCostStalls: Chief Engineer grants Convoke to Walking
// Ballista, whose cost has {X}. gorge announces before X and offers every
// creature a possible generic payment; the kernel stalls.
func TestStackKwGrantXCostStalls(t *testing.T) {
	stackKwStall(t, "Walking Ballista")
}

// TestStackKwGrantPlainCastDoesNotStall: the same grant on a spell without
// Improvise or {X} still announces and casts (the guard is narrow).
func TestStackKwGrantPlainCastDoesNotStall(t *testing.T) {
	sc, m := stackKwStallScen(t, "Pilgrim's Eye")
	sc.do(t, AkCast, uint8(m), 0xff)
	d := sc.s.Step()
	if sc.s.Stalled() {
		t.Fatalf("Pilgrim's Eye stalled: %q", StallName(sc.s.StallReason()))
	}
	if d.Kind != DkChoose || d.Sub != ChGenConvoke {
		t.Fatalf("Pilgrim's Eye: decision %v/%d, want the convoke ask", d.Kind, d.Sub)
	}
}

// TestStackKwGrantHarmonizeStalls: Chief Engineer grants Convoke to a spell
// cast with Harmonize. gorge's convokeAsk offers harmonize and convoke picks
// together; the kernel would pose harmonize only, so it stalls.
func TestStackKwGrantHarmonizeStalls(t *testing.T) {
	sc, m := stackKwStallScen(t, "Pilgrim's Eye")
	k := sc.k
	k.t.Cards[sc.s.objs[m].card].Harmonize = true
	sc.do(t, AkCast, uint8(m), 0xff)
	sc.s.bld().e.flags |= sfFlashback
	if !harmonizeEntry(&k.t.Cards[sc.s.bld().e.card], &sc.s.bld().e) {
		t.Fatal("entry is not a harmonize cast")
	}
	d := sc.s.Step()
	if !sc.s.Stalled() || sc.s.StallReason() != StallStackKwGrant {
		t.Fatalf("no StallStackKwGrant; stalled=%v decision %v/%d", sc.s.Stalled(), d.Kind, d.Sub)
	}
	if d.Kind != DkNone {
		t.Fatalf("posed decision %v/%d past the stall", d.Kind, d.Sub)
	}
}

// TestStackKwGrantAltRouteStalls: a granted Convoke on a card with an
// alternative-cost route (here Impending) stalls rather than pose a menu
// unverified against gorge.
func TestStackKwGrantAltRouteStalls(t *testing.T) {
	sc, m := stackKwStallScen(t, "Pilgrim's Eye")
	sc.k.t.Cards[sc.s.objs[m].card].Impending = 2
	sc.do(t, AkCast, uint8(m), 0xff)
	d := sc.s.Step()
	if !sc.s.Stalled() || sc.s.StallReason() != StallStackKwGrant || d.Kind != DkNone {
		t.Fatalf("no StallStackKwGrant; stalled=%v decision %v/%d", sc.s.Stalled(), d.Kind, d.Sub)
	}
}
