package rules

// The Rakdos-params brief, gap 8: AddKeyword$ PayLifeInsteadOf:B -- K'rrik,
// Son of Yawgmoth's "For each {B} in a cost, you may pay 2 life rather than
// pay that mana." The grant is an S:Mode$ Continuous static whose Affected$
// You names the controller as a player, so the payment machinery consumes it:
// every plain {B} pip in every cost the granting player pays also accepts 2
// life, under the same deterministic prefer-mana-then-life assignment the
// printed {B/P} pips use. The offer gate (castable), the X bounds, the mana
// window and payMana all go through payerPayable/payerGrantsPayLifeInsteadOfB,
// so a cost payable only by life is offered and charged consistently.
//
// K'rrik's own printed {B/P} pips were already native; this is the GRANTED
// side, tested on a second spell's plain {B} pip.

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func krrikEngine(t *testing.T) (*Engine, state.ObjID) {
	t.Helper()
	e := handEngine(t, corpusAlternativeCard(t, "K'rrik, Son of Yawgmoth"))
	for _, o := range e.G.Zone(state.ZHand, 0) {
		if e.G.Obj(o).Face().Name == "K'rrik, Son of Yawgmoth" {
			e.emit(events.Event{Kind: events.MoveZone, Obj: o, From: state.ZHand, To: state.ZBattlefield})
			break
		}
	}
	o := e.G.AddObject(card(t, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
	e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
	bolt := e.G.AddObject(card(t, "Name:Doom Blade\nManaCost:1 B\nTypes:Instant\nA:SP$ Destroy | ValidTgts$ Creature.nonBlack\nOracle:x\n"), 0)
	e.G.SetZone(state.ZHand, 0, append(e.G.Zone(state.ZHand, 0), bolt.ID))
	addMana(t, e, 0, "GG")
	return e, o.ID
}

func castDoomBlade(t *testing.T, e *Engine, target state.ObjID) {
	t.Helper()
	hand := e.G.Zone(state.ZHand, 0)
	doom := hand[len(hand)-1]
	d := e.Pending()
	opt := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == doom {
			opt = o.Index
		}
	}
	if opt < 0 {
		t.Fatalf("Doom Blade not offered: %+v", d.Options)
	}
	submitChoices(t, e, opt)
	dt := e.Pending()
	if dt == nil || dt.Kind != decision.KTarget {
		t.Fatalf("target ask %+v", dt)
	}
	idx := -1
	for _, o := range dt.Options {
		if o.Obj == target {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("target options missing %d: %+v", target, dt.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 20)
}

func TestKrrikGrantPaysBlackPipWithLife(t *testing.T) {
	e, bear := krrikEngine(t)
	e.G.Players[0].Life = 20
	// The pool holds only green: the {1} generic comes from it, the {B} pip
	// from 2 life.
	castDoomBlade(t, e, bear)
	if life := e.G.Players[0].Life; life != 18 {
		t.Fatalf("life=%d, want 18 (the {B} pip paid with 2 life)", life)
	}
	if pool := e.G.Players[0].Pool; pool.Total() != 1 {
		t.Fatalf("pool=%+v, want 1 (one green spent on the {1}, the other untouched)", pool)
	}
	if e.G.Obj(bear).Zone != state.ZGraveyard {
		t.Fatalf("bear zone=%s, want graveyard (Doom Blade resolved)", e.G.Obj(bear).Zone)
	}
}

func TestKrrikGrantPrefersManaWhenItExists(t *testing.T) {
	e, bear := krrikEngine(t)
	e.G.Players[0].Life = 20
	addMana(t, e, 0, "B")
	castDoomBlade(t, e, bear)
	// The pip prefers the pool's black; life untouched.
	if life := e.G.Players[0].Life; life != 20 {
		t.Fatalf("life=%d, want 20 (the pip preferred the pool's {B})", life)
	}
	if e.G.Obj(bear).Zone != state.ZGraveyard {
		t.Fatalf("bear zone=%s", e.G.Obj(bear).Zone)
	}
}

func TestKrrikGrantIsTheGrantorsCostsOnly(t *testing.T) {
	// Without K'rrik on the battlefield the same cast is NOT offered on the
	// same board: the {B} pip has no mana and no life route.
	e := handEngine(t, corpusAlternativeCard(t, "K'rrik, Son of Yawgmoth"))
	o := e.G.AddObject(card(t, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
	e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
	bolt := e.G.AddObject(card(t, "Name:Doom Blade\nManaCost:1 B\nTypes:Instant\nA:SP$ Destroy | ValidTgts$ Creature.nonBlack\nOracle:x\n"), 0)
	e.G.SetZone(state.ZHand, 0, append(e.G.Zone(state.ZHand, 0), bolt.ID))
	addMana(t, e, 0, "GG")
	d := e.Pending()
	for _, x := range d.Options {
		if x.Kind == "cast" && x.Obj == bolt.ID {
			t.Fatalf("Doom Blade offered without the grant: %+v", d.Options)
		}
	}
}

func TestKrrikGrantDoesNotCoverOtherColours(t *testing.T) {
	// The grant names {B} only: a {R} pip stays mana-only.
	e := handEngine(t, corpusAlternativeCard(t, "K'rrik, Son of Yawgmoth"))
	for _, o := range e.G.Zone(state.ZHand, 0) {
		if e.G.Obj(o).Face().Name == "K'rrik, Son of Yawgmoth" {
			e.emit(events.Event{Kind: events.MoveZone, Obj: o, From: state.ZHand, To: state.ZBattlefield})
			break
		}
	}
	shock := e.G.AddObject(card(t, "Name:Shock\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 2\nOracle:x\n"), 0)
	e.G.SetZone(state.ZHand, 0, append(e.G.Zone(state.ZHand, 0), shock.ID))
	addMana(t, e, 0, "GG")
	d := e.Pending()
	for _, x := range d.Options {
		if x.Kind == "cast" && x.Obj == shock.ID {
			t.Fatalf("a {R} pip payable via the B-only grant: %+v", d.Options)
		}
	}}
