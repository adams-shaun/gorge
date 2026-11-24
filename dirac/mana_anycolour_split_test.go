package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAnyColourTapExtraSplitsOnThePool: Dawn's Reflection's two mana is
// answered one unit at a time against the pips the play still owes less the
// pool the tap's own mana reached (gorge's standalone colour ask, windowOwed;
// CR 106.12a, 605.4a). A {B}{B} cast taps a Swamp under the aura: the Swamp's
// own {B} covers one pip, so the extra is {B}{W} and the pool leaves {W}, not
// the {B}{B} that would leave {B} (pool dr-mbc-abbb pair 77, decision 287).
func TestAnyColourTapExtraSplitsOnThePool(t *testing.T) {
	set := cardsettest.Set(t, "mbc")
	deck := []cardset.Entry{
		{Name: "Swamp", Count: 1},
		{Name: "Dawn's Reflection", Count: 1},
		{Name: "Stitch Together", Count: 1},
	}
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not supported in the mbc table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Swamp"], Config{NoElide: true})
	swamp := sc.perm(id["Swamp"], 0)
	aura := sc.perm(id["Dawn's Reflection"], 0)
	sc.s.attachTo(sc.k, sc.slot(aura), sc.slot(swamp))
	stitch := sc.hand(id["Stitch Together"], 0)
	sl := sc.slot(swamp)
	if !sc.s.scriptTapsDefer(sc.k, 0).has(sl) {
		t.Fatal("precondition: the aura'd Swamp is not a Taps-deferred gap")
	}
	cast := MakeAction(AkCast, TCode(stitch), 0, 0)
	sc.s.tapDeferFloat(sc.k, 0, sl, 0, cast)
	want := [kop.NumManaKinds]uint8{kop.W: 1, kop.B: 2}
	if got := sc.s.pl[0].pool; got != want {
		t.Fatalf("pool %v after the tap, want %v: the extra must split {B}{W} beside the Swamp's {B}", got, want)
	}
}
