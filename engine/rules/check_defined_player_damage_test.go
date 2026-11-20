package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestCheckDefinedPlayerDragonCultistDamageDoneSingleSource: "a source you
// controlled dealt 5 or more damage this turn" reads the per-source damage
// ledger, summed per source, attributed by the controller at the hit.
func TestCheckDefinedPlayerDragonCultistDamageDoneSingleSource(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _, ids := dsBoard(t, reg, "Dragon Cultist")
	src := ids["Dragon Cultist"]
	const spec = "You.damageDoneSingleSource GE5"
	lib0 := e.G.Zone(state.ZLibrary, 0)
	lib1 := e.G.Zone(state.ZLibrary, 1)
	a, b, opp := lib0[0], lib0[1], lib1[0]
	if a == b || e.G.Obj(a).Controller != 0 || e.G.Obj(opp).Controller != 1 {
		t.Fatal("precondition: two distinct seat-0 sources and one seat-1 source")
	}
	hit := func(s state.ObjID, n int32) {
		t.Helper()
		before := len(e.G.Obj(s).DamageDealtThisTurn)
		e.damaging = s
		e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: n})
		if len(e.G.Obj(s).DamageDealtThisTurn) != before+1 {
			t.Fatalf("precondition: hit of %d by %d was not ledgered", n, s)
		}
	}
	want := func(you state.PlayerID, exp bool, why string) {
		t.Helper()
		if got := e.checkDefinedPlayerHolds(spec, src, you, nil); got != exp {
			t.Fatalf("%s: you=%d got %v want %v", why, you, got, exp)
		}
	}
	want(0, false, "no damage")
	hit(a, 4)
	want(0, false, "4 from one source")
	hit(b, 3)
	want(0, false, "4 and 3 from different sources")
	hit(a, 1)
	want(0, true, "5 from one source (4+1 over two hits)")
	want(1, false, "seat 1 controlled none of it")
	hit(opp, 5)
	want(1, true, "5 from a source seat 1 controlled")

	for _, bad := range []string{"You.damageDoneSingleSource", "You.damageDoneSingleSource XX5", "You.damageDoneSingleSource  "} {
		if e.checkDefinedPlayerHolds(bad, src, 0, nil) {
			t.Errorf("%q must fail closed", bad)
		}
	}
	e.emit(events.Event{Kind: events.TurnChange, Player: 1, Amount: 3})
	want(0, false, "ledger cleared at turn change")
	want(1, false, "ledger cleared at turn change")
}
