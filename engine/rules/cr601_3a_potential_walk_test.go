package rules

// CR 601.3a's offer gate prices its candidate X announcements against the
// pool the walk prices affordability with: the real walk (legalActions)
// prices the seat's restriction-adjusted floating pool, and the potential
// walk (PotentialActions) prices the hypothetical bound the seat would hold
// after floating every untapped source. Before the pool was threaded into
// the gate the escape always read the floating pool, so with the mana still
// sitting in untapped lands -- the normal state -- the projection hid a
// cast whose X announcement would escape a mana-value prohibition once the
// mana was floated, and a client or auto-pass never learned the cast
// existed. Real corpus cards; the board is built eventlessly.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestCR601_3aPotentialWalkSeesPayableEscape: Mind Spring ({X}{U}{U}) under
// an opponent's Void Winnower, mana in UNTAPPED Islands and an empty pool.
// The potential walk's bound (4 Islands -> {U}{U}{U}{U}) pays X = 1, whose
// mana value 3 is odd, so the projection offers the plain cast; with only
// 2 Islands no X is payable and the cast stays hidden. A no-Winnower
// control pins the harness: without the prohibition the projection offers
// the cast from the same untapped mana.
func TestCR601_3aPotentialWalkSeesPayableEscape(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		islands  int
		winnower bool
		want     bool
		label    string
	}{
		{4, true, true, "four_islands_escape_projected"},
		{2, true, false, "two_islands_no_payable_escape"},
		{4, false, true, "no_winnower_control"},
	} {
		t.Run(tc.label, func(t *testing.T) {
			t.Parallel()
			e := handEngine(t, card(t, corpusCardText(t, "m/mind_spring.txt")))
			spring := e.G.Zone(state.ZHand, 0)[0]
			if tc.winnower {
				battlePerm(t, e, 1, corpusCardText(t, "v/void_winnower.txt"))
			}
			for i := 0; i < tc.islands; i++ {
				onBoardCard(t, e, 0, mustCorpusCardT(t, "Island"))
			}
			if n := e.G.Players[0].Pool.Total(); n != 0 {
				t.Fatalf("pool holds %d mana, want empty: the projection must see the escape through untapped mana", n)
			}
			offered := false
			for _, a := range e.PotentialActions(0) {
				if a.Kind == "cast" && a.Obj == spring && a.Mode == "" {
					offered = true
				}
			}
			if offered != tc.want {
				t.Fatalf("PotentialActions(0) offers the Mind Spring cast = %v, want %v (Islands: %d, Void Winnower: %t)",
					offered, tc.want, tc.islands, tc.winnower)
			}
		})
	}
}

// TestCR601_3aEscapeReservesEveryManaPart: the escape's payable-X pricing
// runs the offer's affordability helper, which settles EVERY mana part --
// a hybrid pip is paid by one of its faces, never by generic alone. A
// synthetic {X}{W/U}{W/U} spell (mana value 2, so Void Winnower's even-mana
// prohibition covers it at X = 0; X = 1 is mana value 3) whose base cast IS
// affordable: with {W}{U} floating the coloured+generic-only budget claimed
// the X = 1 escape, but X = 1 needs {1} more generic than the two pips'
// payment leaves, so no escaping X is payable and the cast must be withheld;
// with {W}{W}{U}{U} floating the X = 1 escape is payable and the cast is
// offered.
func TestCR601_3aEscapeReservesEveryManaPart(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		pool  map[string]int
		want  bool
		label string
	}{
		{map[string]int{"W": 1, "U": 1}, false, "pips_leave_no_generic"},
		{map[string]int{"W": 2, "U": 2}, true, "payable_escape"},
	} {
		t.Run(tc.label, func(t *testing.T) {
			t.Parallel()
			e := handEngine(t, card(t, "Name:Hybrid Spring\nManaCost:X W/U W/U\nTypes:Instant\nOracle:x\n"))
			spring := e.G.Zone(state.ZHand, 0)[0]
			battlePerm(t, e, 1, corpusCardText(t, "v/void_winnower.txt"))
			for c, n := range tc.pool {
				e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: c, Amount: int32(n)})
			}
			offered := false
			for _, a := range e.PotentialActions(0) {
				if a.Kind == "cast" && a.Obj == spring && a.Mode == "" {
					offered = true
				}
			}
			if offered != tc.want {
				t.Fatalf("PotentialActions(0) offers the {X}{{W/U}}{{W/U}} cast = %v, want %v (pool %+v)",
					offered, tc.want, tc.pool)
			}
		})
	}
}
