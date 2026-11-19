package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestMyriadLandscapeActivatePaidByPathOfAncestry pins the farm fshadow
// answer-missing divergence (action:activate, Myriad Landscape, 4-seat
// Commander pod 13102605400): with a B/R commander, an untapped Path of
// Ancestry ("Produced$ Combo ColorIdentity | TriggersWhenSpent$ TrigScry") is
// a real mana source for the payment planner, so Myriad Landscape's
// {2},{T},Sacrifice ability (paid by the Path and a Swamp) is offered at
// priority and Commander's Sphere ({3}) has a payment plan tapping all three
// lands. The scry rider made the planner defer the Path
// ("source:special_production"), which left only the Swamp for the {2}.
func TestMyriadLandscapeActivatePaidByPathOfAncestry(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	strefan := corpusCommander(t, reg, "Strefan, Maurer Progenitor")
	e, cfg := colourIdentityGame(t, 4242, FormatCommander, strefan, nil,
		corpusCommander(t, reg, "Path of Ancestry"),
		corpusCommander(t, reg, "Myriad Landscape"),
		corpusCommander(t, reg, "Swamp"),
		corpusCommander(t, reg, "Commander's Sphere"))
	path := moveToBattlefieldByName(t, e, 0, "Path of Ancestry")
	ml := moveToBattlefieldByName(t, e, 0, "Myriad Landscape")
	swamp := moveToBattlefieldByName(t, e, 0, "Swamp")
	for _, id := range []state.ObjID{path, ml, swamp} {
		if e.G.Obj(id).Tapped {
			e.emit(events.Event{Kind: events.Untap, Obj: id})
		}
		if e.G.Obj(id).Tapped {
			t.Fatalf("precondition: object %d is still tapped", id)
		}
	}
	sphere := moveSeeded(t, e, 0, "Name:Commander's Sphere\nManaCost:3\nTypes:Artifact\n", state.ZHand)
	if o := e.G.Obj(sphere); o.Zone != state.ZHand {
		t.Fatalf("precondition: Commander's Sphere is in %s, want hand", o.Zone)
	}
	e.pending = nil
	e.priorityRound()
	d := e.Pending()
	if d == nil {
		t.Fatal("no pending decision")
	}

	// Myriad Landscape's {2},{T},Sac ability is an "ability" option (its {T}
	// mana ability is the separate "activate" one).
	abilities := 0
	for _, o := range d.Options {
		if o.Obj == ml && o.Kind == "ability" {
			abilities++
		}
	}
	if abilities != 1 {
		t.Errorf("Myriad Landscape's ability options = %d, want 1 (Path of Ancestry and the Swamp pay the {2}, the Landscape taps itself)", abilities)
	}

	// Commander's Sphere is cast through a payment plan whose activations
	// tap exactly the three lands.
	want := []state.ObjID{path, ml, swamp}
	slices.Sort(want)
	found := false
	for _, act := range e.EnsurePaymentPlanAlternatives() {
		if act.Cast.Object != sphere {
			continue
		}
		for _, p := range act.Plans {
			got := make([]state.ObjID, 0, len(p.Activations))
			for _, a := range p.Activations {
				got = append(got, a.Source)
			}
			slices.Sort(got)
			if slices.Equal(got, want) {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("no payment plan for Commander's Sphere taps the Path of Ancestry, Myriad Landscape and Swamp")
	}
	replayCheck(t, e, cfg)
}
