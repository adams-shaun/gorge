package rules

// mayplay_indomitable_test.go pins The Indomitable's graveyard cast: its
// printed MayPlay$ True static is gated by IsPresent$ Pirate...,Vehicle... with
// PresentCompare$ GE3, which the may-play fallback used to refuse outright.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

func tappedOnBoard(t *testing.T, e *Engine, script string) {
	t.Helper()
	id := onBoard(t, e, 0, script)
	e.G.Obj(id).Tapped = true
	e.staticEpoch = -1
	e.activeEpoch = -1
}

const (
	probePirate  = "Name:Probe Pirate\nTypes:Creature Pirate\nPT:1/1\nOracle:\n"
	probeVehicle = "Name:Probe Vehicle\nTypes:Artifact Vehicle\nPT:1/1\nOracle:\n"
)

func TestIndomitableGraveyardCastNeedsThreeTapped(t *testing.T) {
	for _, third := range []string{probePirate, probeVehicle} {
		e := mayPlayBase(t)
		o := e.G.AddObject(corpusAlternativeCard(t, "The Indomitable"), 0)
		o.Zone = state.ZGraveyard
		e.G.SetZone(state.ZGraveyard, 0, []state.ObjID{o.ID})
		tappedOnBoard(t, e, probePirate)
		tappedOnBoard(t, e, probePirate)
		if e.G.Obj(o.ID).Zone != state.ZGraveyard || len(e.G.Zone(state.ZBattlefield, 0)) != 2 {
			t.Fatal("precondition: The Indomitable in the graveyard with two tapped Pirates")
		}
		if _, ok := e.mayPlayGrant(0, o.ID); ok {
			t.Fatal("granted with only two tapped Pirates")
		}
		if hasSpellOffer(e, o.ID) {
			t.Fatal("cast offered with only two tapped Pirates")
		}
		tappedOnBoard(t, e, third)
		if _, ok := e.mayPlayGrant(0, o.ID); !ok {
			t.Fatal("not granted with three tapped Pirates/Vehicles")
		}
		if !hasSpellOffer(e, o.ID) {
			t.Fatal("cast not offered with three tapped Pirates/Vehicles")
		}
		if third != probePirate {
			continue
		}
		addMana(t, e, 0, "UUCC")
		var opt *decision.Option
		for i := range e.Pending().Options {
			x := &e.Pending().Options[i]
			if x.Kind == "cast" && x.Obj == o.ID && x.Mode == "mayplay" {
				opt = x
			}
		}
		if opt == nil {
			t.Fatalf("no mayplay cast option: %+v", e.Pending().Options)
		}
		submitChoices(t, e, opt.Index)
		resolveStack(t, e)
		if got := e.G.Obj(o.ID).Zone; got != state.ZBattlefield {
			t.Fatalf("The Indomitable zone = %v, want battlefield", got)
		}
	}
}
