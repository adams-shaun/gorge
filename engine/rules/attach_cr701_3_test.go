package rules

// CR 701.3a/b: an effect that tries to attach an Aura, Equipment or
// Fortification to the object it is already attached to does nothing, and
// one that tries to attach it to an object it can't be attached to leaves it
// where it is. Every card is a real corpus card.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Enormous Energy Blade ("Whenever Enormous Energy Blade becomes attached to
// a creature, tap that creature") is equipped to the Bears, which taps them.
// The Bears untap, and the Blade is equipped to the Bears again: that does
// nothing (CR 701.3b), so the Blade does not "become attached", its trigger
// does not fire and the Bears stay untapped.
func TestReEquipToTheSameCreatureDoesNothing(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Enormous Energy Blade"), lookup(t, reg, "Grizzly Bears")}, nil)
	blade := moveCorpusCard(t, e, "Enormous Energy Blade", 0, state.ZBattlefield)
	bears := moveCorpusCard(t, e, "Grizzly Bears", 0, state.ZBattlefield)
	addMana(t, e, 0, "GG")
	attachActivateResolve(t, e, blade, 0)
	if e.G.Obj(blade).AttachedTo != bears || !e.G.Obj(bears).Tapped {
		t.Fatalf("precondition: Blade attached to %d (want %d), Bears tapped=%v (want true)",
			e.G.Obj(blade).AttachedTo, bears, e.G.Obj(bears).Tapped)
	}
	e.emit(events.Event{Kind: events.Untap, Obj: bears})
	attachedBefore := countEvents(e, func(ev events.Event) bool { return ev.Kind == events.Attach && ev.Obj == blade })
	addMana(t, e, 0, "GG")
	attachActivateResolve(t, e, blade, 0)
	if got := countEvents(e, func(ev events.Event) bool { return ev.Kind == events.Attach && ev.Obj == blade }); got != attachedBefore {
		t.Errorf("CR 701.3b: re-equipping to the same creature logged %d more Attach event(s), want none", got-attachedBefore)
	}
	if e.G.Obj(bears).Tapped {
		t.Errorf("CR 701.3b: re-equipping to the same creature fired the Blade's becomes-attached trigger (Bears tapped)")
	}
	if e.G.Obj(blade).AttachedTo != bears {
		t.Errorf("Blade attached to %d, want still the Bears %d", e.G.Obj(blade).AttachedTo, bears)
	}
	replayCheck(t, e, cfg)
}

// Pacifism enchants the opponent's Bears. Aura Graft ("Attach it to another
// permanent it can enchant") is answered with a Mountain, which Pacifism
// ("Enchant creature") can't enchant: the attach does not happen (CR 701.3b),
// so Pacifism stays on the Bears instead of moving onto the land and being
// put into the graveyard by CR 704.5m.
func TestAttachToAnIllegalPermanentDoesNotMove(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Aura Graft"), lookup(t, reg, "Pacifism")},
		[]*cards.Card{lookup(t, reg, "Grizzly Bears")})
	bears := moveCorpusCard(t, e, "Grizzly Bears", 1, state.ZBattlefield)
	mountain := moveCorpusCard(t, e, "Mountain", 0, state.ZBattlefield)
	pacifism := lossAttach(t, e, "Pacifism", 0, bears)
	graft := moveCorpusCard(t, e, "Aura Graft", 0, state.ZHand)
	addMana(t, e, 0, "UU")
	castFirstOf(t, e, graft)
	offeredMountain := false
	for i := 0; i < 40 && !e.G.Over; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision while casting and resolving Aura Graft")
		}
		if d.Kind == decision.KPriority {
			if len(e.G.Stack) == 0 {
				break
			}
			submitPass(t, e)
			continue
		}
		pick := d.Options[0].Index
		for _, o := range d.Options {
			if d.Kind == decision.KTarget && o.Obj == pacifism {
				pick = o.Index
			}
			if d.ResumeKind == "attach_choice" && o.Obj == mountain {
				pick, offeredMountain = o.Index, true
			}
		}
		submitChoices(t, e, pick)
	}
	p := e.G.Obj(pacifism)
	if p.Zone != state.ZBattlefield {
		t.Fatalf("CR 701.3b: Pacifism is in %s (Mountain offered=%v), want it left on the battlefield attached to the Bears", p.Zone, offeredMountain)
	}
	if p.AttachedTo == mountain {
		t.Errorf("Pacifism is attached to a Mountain it can't enchant")
	}
	if offeredMountain && p.AttachedTo != bears {
		t.Errorf("CR 701.3b: an illegal attach moved Pacifism to %d, want it still on the Bears %d", p.AttachedTo, bears)
	}
	replayCheck(t, e, cfg)
}

// attachActivateResolve activates obj's ability idx, answers every
// non-priority ask (the equip target) with its first option, and resolves
// the stack.
func attachActivateResolve(t *testing.T, e *Engine, obj state.ObjID, idx int) {
	t.Helper()
	submitChoices(t, e, abilityOption(t, e, obj, idx).Index)
	// CR 602.2a: the ability is on the stack before its target is chosen, so
	// the loop runs until the activation's decisions are answered, not until
	// the stack is non-empty.
	for i := 0; i < 5; i++ {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			break
		}
		submitChoices(t, e, d.Options[0].Index)
	}
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack %v after activating %d, want the ability", e.G.Stack, obj)
	}
	passUntilStackEmpty(t, e, 20)
}

// CR 701.3d: an Equipment that leaves the battlefield while attached becomes
// unattached from its bearer, and CR 603.10a makes "becomes unattached"
// triggers look back in time. Grafted Wargear ("Whenever Grafted Wargear
// becomes unattached from a permanent, sacrifice that permanent") equipped to
// the Bears is destroyed: its trigger looks back from the graveyard and the
// Bears are sacrificed. The departure is the unattach; no separate Unattached
// event is logged for it.
func TestAttachedEquipmentLeavingBecomesUnattached(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Grafted Wargear"), lookup(t, reg, "Grizzly Bears"),
		lookup(t, reg, "Llanowar Elves")}, nil)
	wargear := moveCorpusCard(t, e, "Grafted Wargear", 0, state.ZBattlefield)
	bears := moveCorpusCard(t, e, "Grizzly Bears", 0, state.ZBattlefield)
	elves := moveCorpusCard(t, e, "Llanowar Elves", 0, state.ZBattlefield)
	equipOn(t, e, wargear, bears)
	if e.G.Obj(wargear).AttachedTo != bears {
		t.Fatalf("precondition: Grafted Wargear attached to %d, want the Bears %d", e.G.Obj(wargear).AttachedTo, bears)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: wargear, From: state.ZBattlefield, To: state.ZGraveyard})
	e.pending = nil
	e.Advance()
	passUntilStackEmpty(t, e, 30)
	if o := e.G.Obj(bears); o.Zone != state.ZGraveyard {
		t.Errorf("CR 701.3d/603.10a: the Wargear's bearer is in %s, want sacrificed to the graveyard", o.Zone)
	}
	if o := e.G.Obj(elves); o.Zone != state.ZBattlefield {
		t.Errorf("the unequipped creature is in %s, want the battlefield", o.Zone)
	}
	replayCheck(t, e, cfg)
}
