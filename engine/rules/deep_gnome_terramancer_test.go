package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// deepGnomeSrc is a Deep Gnome Terramancer-shaped fixture (parameters reordered,
// not the corpus line): a land entering under an opponent's control other than
// by being played (CR 305.1, 305.2: playing a land is a special action)
// triggers it.
const deepGnomeSrc = "Name:Deep Gnome Terramancer\nManaCost:1 W\nTypes:Creature Gnome Wizard\nPT:2/2\n" +
	"T:Mode$ ChangesZoneAll | Destination$ Battlefield | InvertValidCause$ True | ValidCause$ LandAbility | " +
	"ValidCards$ Land.OppCtrl | Execute$ TrigSearch | TriggerZones$ Battlefield | OptionalDecider$ You | ResolvedLimit$ 1 | TriggerDescription$ t\n" +
	"SVar:TrigSearch:DB$ ChangeZone | ChangeNum$ 1 | ChangeType$ Card.Plains | Origin$ Library | Destination$ Battlefield | Tapped$ True | ShuffleNonMandatory$ True\n" +
	"Oracle:x\n"

func TestDeepGnomeTerramancerIgnoresPlayedLands(t *testing.T) {
	t.Parallel()
	e := mayPlayBase(t)
	terra := onBoardCard(t, e, 1, card(t, deepGnomeSrc))
	if o := e.G.Obj(terra); o == nil || o.Zone != state.ZBattlefield || o.Controller != 1 {
		t.Fatalf("precondition: Terramancer %d is not on seat 1's battlefield", terra)
	}
	played := handLand(t, e, 0)
	put := handLand(t, e, 0)

	// A played land (the special action) is excluded.
	start := len(e.L.Events)
	e.priorityRound()
	d := e.Pending()
	if d == nil {
		t.Fatal("no decision pending")
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "play_land" && o.Obj == played {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("precondition: no play_land option for %d: %+v", played, d.Options)
	}
	submitChoices(t, e, idx)
	if e.G.Obj(played).Zone != state.ZBattlefield {
		t.Fatalf("precondition: played land is in %v, want the battlefield", e.G.Obj(played).Zone)
	}
	if n := countPushes(e, terra, start); n != 0 {
		t.Fatalf("played land triggered Mold Earth %d times, want 0", n)
	}

	// A land put onto the battlefield by an effect (no land play) fires once.
	start = len(e.L.Events)
	e.emit(events.Event{Kind: events.MoveZone, Obj: put, From: state.ZHand, To: state.ZBattlefield})
	e.priorityRound()
	if n := countPushes(e, terra, start); n != 1 {
		t.Fatalf("effect-put land triggered Mold Earth %d times, want 1", n)
	}
}
