package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// svarReflectedBoard builds the Incubation Druid fixture: Ayula's Influence
// ({G}{G}{G}) in seat 0's hand, two untapped Forests beside it, and -- unless
// druid is false, the control board -- the Druid untapped with n +1/+1
// counters. The Druid's "{T}: Add one mana of any type that a land you control
// could produce" carries its amount in an Amount$ SVar (IncubationAmount: 3
// with a counter on it, 1 without), so its printed body is the executor's
// blank one-colourless default and the production is only available to a
// reader that evaluates the SVar and the reflection.
func svarReflectedBoard(t *testing.T, druid bool, counters int) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	e := handEngine(t, corpusCardOrFatal(t, "Ayula's Influence"))
	down := e.G.Zone(state.ZHand, 0)[0]
	var druidID state.ObjID
	if druid {
		druidID = onBoardCard(t, e, 0, corpusCardOrFatal(t, "Incubation Druid"))
		e.G.Obj(druidID).SummonSick = false
		for i := 0; i < counters; i++ {
			e.G.Obj(druidID).Counters = append(e.G.Obj(druidID).Counters, state.Counter{Kind: "P1P1", N: 1})
		}
		o := e.G.Obj(druidID)
		if o == nil || o.Zone != state.ZBattlefield || o.Tapped {
			t.Fatalf("precondition: Druid = %+v, want untapped on the battlefield", o)
		}
		if len(o.Counters) != counters {
			t.Fatalf("precondition: Druid counters = %+v, want %d", o.Counters, counters)
		}
	}
	for _, n := range []string{"Forest", "Forest"} {
		id := onBoardCard(t, e, 0, corpusCardOrFatal(t, n))
		e.G.Obj(id).SummonSick = false
	}
	if e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("precondition: floating pool %v, want empty", e.G.Players[0].Pool)
	}
	return e, down, druidID
}

// druidAlts is the Druid's cast-window production alternatives.
func druidAlts(t *testing.T, e *Engine, down, druid state.ObjID) []state.Mana {
	t.Helper()
	pc := &pendingCast{player: 0, card: down, from: state.ZHand, ability: -1,
		cost: e.parseCost("G G G")}
	var got []state.Mana
	for _, u := range e.castWindowUnits(pc) {
		if u.ID != druid {
			continue
		}
		for _, a := range u.Alts {
			got = append(got, a.Mana())
		}
	}
	if len(got) == 0 {
		t.Fatal("the cast window prices no production for the Druid")
	}
	return got
}

// TestReflectedSVarWindowPricesItsOwnColours pins the cast window's probe
// pricing a dynamic-amount ManaReflected as its fold's colours at the
// evaluated amount: with a counter the Druid's unit offers three {G} (the
// Forests' type, the SVar's 3), mana that can pay a {G} pip. The printed
// body's blank default would have priced three {C}, which cannot pay {G}
// (CR 107.5b), and left Ayula's Influence {G}{G}{G} off the priority menu
// (lockstep tail-55 pair 3, seed 595286).
func TestReflectedSVarWindowPricesItsOwnColours(t *testing.T) {
	t.Parallel()
	e, down, druid := svarReflectedBoard(t, true, 3)
	want := []state.Mana{{4: 3}} // three {G}, no colourless
	if got := druidAlts(t, e, down, druid); len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("Druid alts = %v, want %v (three {G})", got, want)
	}
}

// TestReflectedSVarWindowAmountFollowsTheSVar pins the amount half: without a
// counter the same SVar resolves to 1, so the unit offers one {G} -- not the
// counter-boosted three, and never the blank default's {C}.
func TestReflectedSVarWindowAmountFollowsTheSVar(t *testing.T) {
	t.Parallel()
	e, down, druid := svarReflectedBoard(t, true, 0)
	want := []state.Mana{{4: 1}} // one {G}
	if got := druidAlts(t, e, down, druid); len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("Druid alts = %v, want %v (one {G})", got, want)
	}
}

// TestReflectedSVarBoundOffersTheCast pins the potential bound's half, the one
// the priority menu prices on: with the Druid (counter-boosted {G}) the priced
// offer gate admits the {G}{G}{G} cast an empty pool alone cannot pay, while
// the control board -- the same two Forests, no Druid -- refuses it, so the
// differential is the reflection's own colours. The pre-fix bound read the
// reflection as unbounded colourless, which cannot pay a {G} pip, and
// withheld the cast.
func TestReflectedSVarBoundOffersTheCast(t *testing.T) {
	t.Parallel()
	ctl, ctlDown, _ := svarReflectedBoard(t, false, 0)
	if got := ctl.offerCastablePotential(0, ctlDown, ctl.parseCost("G G G"), spellScope(""), false); got {
		t.Fatalf("precondition: the two Forests alone pay {G}{G}{G} (bound %v); the differential below would not test the reflection", ctl.potentialManaFor(0, nil))
	}
	e, down, _ := svarReflectedBoard(t, true, 3)
	if !e.offerCastablePotential(0, down, e.parseCost("G G G"), spellScope(""), false) {
		t.Fatal("the potential bound prices the Druid's reflection colourless; the {G}{G}{G} cast stays unaffordable")
	}
}
