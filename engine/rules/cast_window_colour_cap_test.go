package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// cwColourBoard is the board a Crimson Hellkite activation ({X}, spend only
// red mana on X) priced its X against when the tail-61 lockstep game hung:
// five Forests, two Mountains, a red source, two colourless sources, two
// fee-paid any-colour rocks ({1} and {2}, each with a free {C} too) and two
// free any-colour tokens. Seven red at most, fourteen mana in all.
func cwColourBoard() []windowManaUnit {
	one := func(sym byte) windowManaAlt {
		var c [6]int32
		c[state.ManaIndex(sym)] = 1
		return windowManaAlt{Counts: c, Amt: 1}
	}
	anyColour := func(fee int32) []windowManaAlt {
		var alts []windowManaAlt
		for _, sym := range []byte("WUBRG") {
			a := one(sym)
			a.CostGeneric = fee
			alts = append(alts, a)
		}
		return alts
	}
	var units []windowManaUnit
	add := func(alts ...windowManaAlt) {
		units = append(units, windowManaUnit{ID: state.ObjID(1000 + len(units)), Alts: alts})
	}
	for range 5 {
		add(one('G'))
	}
	for range 3 {
		add(one('R'))
	}
	add(one('C'))
	add(one('C'))
	add(append([]windowManaAlt{one('C')}, anyColour(1)...)...)
	add(append([]windowManaAlt{one('C')}, anyColour(2)...)...)
	add(anyColour(0)...)
	add(anyColour(0)...)
	return units
}

// TestCastWindowColourCapRefusesUnreachablePips: an {X} that must be paid in
// red, priced one past the red the sources can make, is refused without a
// walk. The walk's total-mana cap let it through (fourteen mana cover eight
// pips), so every X past seven walked to the node cap -- about a second each,
// asked once per X and again at every priority offer, which ran gorge past
// kfuzz's 30-second game watchdog (tail-61 pair 24, a hang). The reachable X
// is still found, and a rider or conversion that lets other colours pay the
// pips lifts the cap (CR 601.2g-h, CR 107.3).
func TestCastWindowColourCapRefusesUnreachablePips(t *testing.T) {
	t.Parallel()
	b := equipWindowGame(t, 722, cwFreeColorlessSource)
	e := b.engine
	units := cwColourBoard()
	red := func(n int32) Cost {
		var c Cost
		c.Colored[state.ManaIndex('R')] = n
		return c
	}
	var total, reds int32
	for _, u := range units {
		var best, bestRed int32
		for _, a := range u.Alts {
			best = max(best, a.Mana().Total())
			bestRed = max(bestRed, a.Mana()[state.ManaIndex('R')])
		}
		total += best
		reds += bestRed
	}
	if reds != 7 || total < 8 {
		t.Fatalf("precondition: the board must make 7 red and at least 8 mana, got %d red of %d", reds, total)
	}
	walk := func(c Cost, rider pipRider, conv *manaConv) (bool, int) {
		return e.castWindowReachableAfter(cwDeadAfter, state.Mana{}, 0, c, state.Mana{}, state.Mana{}, [8]state.Mana{}, 20, conv, rider, units)
	}
	if ok, nodes := walk(red(7), pipRider{}, nil); !ok {
		t.Fatalf("seven red pips are payable from this board (fees from the Forests), walk refused after %d nodes", nodes)
	}
	ok, nodes := walk(red(8), pipRider{}, nil)
	if ok {
		t.Fatal("eight red pips are not payable from seven red sources")
	}
	if nodes > 0 {
		t.Fatalf("eight red pips from seven red sources: the walk visited %d nodes, want the colour cap to refuse before the first", nodes)
	}
	if ok, nodes := walk(red(8), pipRider{AnyColor: true}, nil); !ok {
		t.Fatalf("mana of any colour pays the red pips: the colour cap must not apply (refused after %d nodes)", nodes)
	}
	conv := &manaConv{}
	for i := range conv.Wild {
		conv.Wild[i] = true
	}
	if ok, nodes := walk(red(8), pipRider{}, conv); !ok {
		t.Fatalf("a spend-as-any-colour conversion pays the red pips: the colour cap must not apply (refused after %d nodes)", nodes)
	}
}
