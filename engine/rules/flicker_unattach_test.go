// flicker_unattach_test.go — CR 400.7's attachment half played through the
// CR 704.5m/n state-based actions: a permanent that leaves the battlefield
// and returns under its owner's control (a flicker) is a NEW object, so
// nothing stays attached across the trip. The move fold
// (engine/events/apply_zone.go) marks every attachment to the departing
// permanent UnattachPending at the move itself — recording LastBearer while
// deliberately leaving the live AttachedTo link standing, so the
// mid-resolution window before the next checkpoint still shows the
// attachment fastened — because the flickered bearer is back on the
// battlefield under the same ObjID before the next SBA checkpoint: the
// bearer-left arm in attachmentSBAs reads the live link and could never see
// the departure, only the queued mark survives the round trip. The SBAs then
// finish the job: the Aura goes to its owner's graveyard (704.5m) and the
// Equipment sits unattached on the battlefield (704.5n), so the returned
// bearer has its base characteristics. Before the fix the returned creature
// carried its Aura, its +1/+0 and its flying across the flicker. All cards
// are real compiled corpus cards; no Forge script text is committed here.
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

	// castAuraOntoBear casts the on-hand Aura onto the battlefield Bear
	// through the real cast path and asserts the attachment and the pumped
	// 3/2 flying bearer, so a setup that never attached cannot reach the
	// flicker assertions.
	castAuraOntoBear := func(t *testing.T, e *Engine, aura, bear state.ObjID) {
		t.Helper()
		addMana(t, e, 0, "WW")
		d := e.Pending()
		idx := -1
		for _, o := range d.Options {
			if o.Kind == "cast" && o.Obj == aura {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("precondition: no cast option for the Aura: %+v", d.Options)
		}
		submitChoices(t, e, idx)
		if d = e.Pending(); d != nil && d.Kind == decision.KTarget {
			if len(d.Options) != 1 || d.Options[0].Obj != bear {
				t.Fatalf("precondition: target options = %+v, want exactly the Bear", d.Options)
			}
			submitChoices(t, e, d.Options[0].Index)
		}
		passUntilStackEmpty(t, e, 20)
		ao := e.G.Obj(aura)
		if ao == nil || ao.Zone != state.ZBattlefield || ao.AttachedTo != bear {
			t.Fatalf("precondition: Aura must be on the battlefield attached to the Bear, got %+v", ao)
		}
		if !e.HasKeyword(bear, "Flying") || e.Power(bear) != 3 || e.Toughness(bear) != 2 {
			t.Fatalf("precondition: enchanted Bear = %d/%d flying=%v, want 3/2 flying",
				e.Power(bear), e.Toughness(bear), e.HasKeyword(bear, "Flying"))
		}
	}

	// assertFlickerOutcome pins the post-flicker board: the Aura is in its
	// owner's graveyard, the returned Bear (same ObjID, CR 400.7 new object)
	// is base 2/2 without flying, and the LastBearer record names the Bear.
	assertFlickerOutcome := func(t *testing.T, e *Engine, aura, bear state.ObjID) {
		t.Helper()
		bo := e.G.Obj(bear)
		if bo == nil || bo.Zone != state.ZBattlefield {
			t.Fatalf("returned Bear zone = %v, want battlefield", bo)
		}
		ao := e.G.Obj(aura)
		if ao == nil || ao.Zone != state.ZGraveyard {
			t.Fatalf("BROKE: flickered Aura is in %v, want its owner's graveyard (CR 704.5m)", ao)
		}
		if e.HasKeyword(bear, "Flying") || e.Power(bear) != 2 || e.Toughness(bear) != 2 {
			t.Errorf("BROKE: returned Bear = %d/%d flying=%v, want base 2/2 without flying",
				e.Power(bear), e.Toughness(bear), e.HasKeyword(bear, "Flying"))
		}
		if ao := e.G.Obj(aura); ao.LastBearer != bear {
			t.Errorf("Aura LastBearer = %d, want the departed Bear %d", ao.LastBearer, bear)
		}
	}

	t.Run("aura to graveyard", func(t *testing.T) {
		e, cfg := manifestEngine(t, reg, "Feather of Flight", "Grizzly Bears")
		bear := putCorpusPermanent(t, e, "Grizzly Bears")
		aura := searchMoveByName(t, e, "Feather of Flight", state.ZHand)
		castAuraOntoBear(t, e, aura, bear)

		// Flicker: exile and return under the owner's control, the same
		// ObjID throughout (CR 400.7 makes the returned permanent a new
		// object regardless). Between the two moves the fold's window
		// contract holds: the Aura is still on the battlefield fastened to
		// the departed bearer, with the detach queued for the next
		// state-based check.
		e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZBattlefield, To: state.ZExile})
		if ao := e.G.Obj(aura); ao.Zone != state.ZBattlefield || ao.AttachedTo != bear || !ao.UnattachPending {
			t.Fatalf("mid-window: Aura zone=%v attached=%d queued=%v, want battlefield/%d/true (the link stands until the SBA publishes the queued detach)",
				ao.Zone, ao.AttachedTo, ao.UnattachPending, bear)
		}
		e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZExile, To: state.ZBattlefield})
		e.checkStateBased()

		assertFlickerOutcome(t, e, aura, bear)
		replayCheck(t, e, cfg)
	})

	// The card-driven subtest plays the same flicker through a real corpus
	// card: Ephemerate's one resolution (SP$ ChangeZone exiles the bearer,
	// its SubAbility returns it), the shape Lilysplash Mentor's activation
	// reports. It pins the fold against the real ChangeZone path, not just
	// raw MoveZone events.
	t.Run("card-driven flicker (Ephemerate)", func(t *testing.T) {
		e, cfg := manifestEngine(t, reg, "Ephemerate", "Feather of Flight", "Grizzly Bears")
		bear := putCorpusPermanent(t, e, "Grizzly Bears")
		aura := searchMoveByName(t, e, "Feather of Flight", state.ZHand)
		eph := searchMoveByName(t, e, "Ephemerate", state.ZHand)
		castAuraOntoBear(t, e, aura, bear)

		addMana(t, e, 0, "W")
		d := e.Pending()
		idx := -1
		for _, o := range d.Options {
			if o.Kind == "cast" && o.Obj == eph {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("precondition: no cast option for Ephemerate: %+v", d.Options)
		}
		submitChoices(t, e, idx)
		if d = e.Pending(); d == nil || d.Kind != decision.KTarget {
			t.Fatalf("precondition: Ephemerate must ask a target, got %+v", d)
		}
		if len(d.Options) != 1 || d.Options[0].Obj != bear {
			t.Fatalf("precondition: Ephemerate target options = %+v, want exactly the Bear", d.Options)
		}
		submitChoices(t, e, d.Options[0].Index)
		passUntilStackEmpty(t, e, 30)

		if bo := e.G.Obj(bear); bo == nil || bo.Zone != state.ZBattlefield || bo.ID != bear {
			t.Fatalf("returned Bear = %+v, want the same ObjID back on the battlefield under its owner's control", bo)
		}
		if controller := e.G.Obj(bear).Controller; controller != e.G.Obj(bear).Owner {
			t.Fatalf("returned Bear controller %d, want owner %d", controller, e.G.Obj(bear).Owner)
		}
		assertFlickerOutcome(t, e, aura, bear)
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
		if eo := e.G.Obj(boots); eo.Zone != state.ZBattlefield || eo.AttachedTo != bear || !eo.UnattachPending {
			t.Fatalf("mid-window: Boots zone=%v attached=%d queued=%v, want battlefield/%d/true (the link stands until the SBA publishes the queued detach)",
				eo.Zone, eo.AttachedTo, eo.UnattachPending, bear)
		}
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
