// flicker_unattach_test.go — CR 400.7's attachment half played through the
// CR 704.5m/n state-based actions: a permanent that leaves the battlefield
// and returns under its owner's control (a flicker) is a NEW object, so
// nothing stays attached across the trip. The move fold
// (engine/events/apply_zone.go) breaks every attachment to the departing
// permanent at the move itself — clearing AttachedTo and recording
// LastBearer — because the flickered bearer is back on the battlefield under
// the same ObjID before the next SBA checkpoint: clearing only in
// attachmentSBAs would never see the departure. The SBAs then finish the
// job: the Aura goes to its owner's graveyard (704.5m) and the Equipment
// sits unattached on the battlefield (704.5n), so the returned bearer has
// its base characteristics. Before the fix the returned creature carried its
// Aura, its +1/+0 and its flying across the flicker. All cards are real
// compiled corpus cards; no Forge script text is committed here.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func TestFlickerUnattachesAura(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)

	t.Run("aura to graveyard", func(t *testing.T) {
		e, cfg := manifestEngine(t, reg, "Feather of Flight", "Grizzly Bears")
		bear := putCorpusPermanent(t, e, "Grizzly Bears")
		aura := searchMoveByName(t, e, "Feather of Flight", state.ZHand)

		// Cast the Aura onto the Bear ({1}{W}) and let it resolve and
		// attach; its ETB draw trigger resolves too.
		addMana(t, e, 0, "WW")
		d := e.Pending()
		idx := -1
		for _, o := range d.Options {
			if o.Kind == "cast" && o.Obj == aura {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("precondition: no cast option for Feather of Flight: %+v", d.Options)
		}
		submitChoices(t, e, idx)
		if d = e.Pending(); d != nil && d.Kind == decision.KTarget {
			if len(d.Options) != 1 || d.Options[0].Obj != bear {
				t.Fatalf("precondition: target options = %+v, want exactly the Bear", d.Options)
			}
			submitChoices(t, e, d.Options[0].Index)
		}
		passUntilStackEmpty(t, e, 20)
		if ao := e.G.Obj(aura); ao == nil || ao.Zone != state.ZBattlefield || ao.AttachedTo != bear {
			t.Fatalf("precondition: Aura must be on the battlefield attached to the Bear, got %+v", ao)
		}
		if !e.HasKeyword(bear, "Flying") || e.Power(bear) != 3 || e.Toughness(bear) != 2 {
			t.Fatalf("precondition: enchanted Bear = %d/%d flying=%v, want 3/2 flying",
				e.Power(bear), e.Toughness(bear), e.HasKeyword(bear, "Flying"))
		}

		// Flicker: exile and return under the owner's control, the same
		// ObjID throughout (CR 400.7 makes the returned permanent a new
		// object regardless).
		e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZBattlefield, To: state.ZExile})
		e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZExile, To: state.ZBattlefield})
		e.checkStateBased()

		if bo := e.G.Obj(bear); bo == nil || bo.Zone != state.ZBattlefield {
			t.Fatalf("returned Bear zone = %v, want battlefield", bo)
		}
		if ao := e.G.Obj(aura); ao == nil || ao.Zone != state.ZGraveyard {
			t.Fatalf("BROKE: flickered Aura is in %v, want its owner's graveyard (CR 704.5m)", ao)
		}
		if e.HasKeyword(bear, "Flying") || e.Power(bear) != 2 || e.Toughness(bear) != 2 {
			t.Errorf("BROKE: returned Bear = %d/%d flying=%v, want base 2/2 without flying",
				e.Power(bear), e.Toughness(bear), e.HasKeyword(bear, "Flying"))
		}
		if ao := e.G.Obj(aura); ao.LastBearer != bear {
			t.Errorf("Aura LastBearer = %d, want the departed Bear %d", ao.LastBearer, bear)
		}
		replayCheck(t, e, cfg)
	})

	t.Run("equipment stays unattached", func(t *testing.T) {
		e, cfg := manifestEngine(t, reg, "Swiftfoot Boots", "Grizzly Bears")
		bear := putCorpusPermanent(t, e, "Grizzly Bears")
		boots := searchMoveByName(t, e, "Swiftfoot Boots", state.ZBattlefield)

		e.emit(events.Event{Kind: events.Attach, Obj: boots, IDs: []state.ObjID{bear}})
		if eo := e.G.Obj(boots); eo == nil || eo.Zone != state.ZBattlefield || eo.AttachedTo != bear {
			t.Fatalf("precondition: Boots must be on the battlefield attached to the Bear, got %+v", eo)
		}
		if !e.HasKeyword(bear, "Hexproof") || !e.HasKeyword(bear, "Haste") {
			t.Fatalf("precondition: equipped Bear lacks hexproof/haste")
		}

		e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZBattlefield, To: state.ZExile})
		e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZExile, To: state.ZBattlefield})
		e.checkStateBased()

		if bo := e.G.Obj(bear); bo == nil || bo.Zone != state.ZBattlefield {
			t.Fatalf("returned Bear zone = %v, want battlefield", bo)
		}
		if eo := e.G.Obj(boots); eo == nil || eo.Zone != state.ZBattlefield || eo.AttachedTo != 0 {
			t.Fatalf("BROKE: flickered-out Equipment is %+v, want on the battlefield unattached (CR 704.5n)", eo)
		}
		if eo := e.G.Obj(boots); eo.LastBearer != bear {
			t.Errorf("Boots LastBearer = %d, want the departed Bear %d", eo.LastBearer, bear)
		}
		if e.HasKeyword(bear, "Hexproof") || e.HasKeyword(bear, "Haste") {
			t.Errorf("BROKE: returned Bear still has hexproof/haste from the unattached Boots")
		}
		replayCheck(t, e, cfg)
	})
}
