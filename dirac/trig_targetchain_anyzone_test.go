package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCastTargetsAnyZone: castTargetsAll reads a kop.ZoneAny TargetsValid$
// object alternative off the battlefield through the card matcher, with the
// stack entry's controller for a spell and the owner otherwise (Danitha,
// Spear of Agony's Creature.OppCtrl); without the marker behaviour is
// unchanged.
func TestCastTargetsAnyZone(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Downwind Ambusher", Count: 2}, {Name: "Plains", Count: 8}})
	creature := id["Downwind Ambusher"]
	// the chains kgen builds for Danitha's "Opponent,Creature.OppCtrl": the
	// object clause points at filterPlayerAlt's player sentinel (an FPlayer
	// alternative that names no object)
	chain := func(zone kop.Zone) kop.FilterID {
		sc.k.t.Filters = append(sc.k.t.Filters, kop.Filter{Ctrl: kop.CtrlOpp, Flags: kop.FPlayer | kop.FSelf | kop.FOther})
		sentinel := kop.FilterID(len(sc.k.t.Filters) - 1)
		sc.k.t.Filters = append(sc.k.t.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlOpp, Zone: zone, Or: sentinel})
		return kop.FilterID(len(sc.k.t.Filters) - 1)
	}
	anyZone := chain(kop.ZoneAny)
	battlefieldOnly := chain(kop.ZoneNone)

	src := sc.perm(creature, 0)
	l := trigListener{obj: src, sl: sc.slot(src), ctrl: 0}

	// pushSpell puts an object on the stack under ctrl.
	pushSpell := func(o Obj, ctrl uint8) {
		sc.s.objs[o].zone = kop.ZoneStack
		sc.s.stack[sc.s.sp] = stackEntry{kind: seSpell, obj: o, ctrl: ctrl}
		sc.s.sp++
	}
	var e stackEntry
	e.nt, e.tgt[0] = 1, 0

	// A creature permanent: an opponent's matches (the burn-spell case), the
	// listener's own does not.
	oppPerm := sc.perm(creature, 1)
	e.tgt[0] = TCode(oppPerm)
	if !sc.s.castTargetsAll(sc.k, anyZone, &e, l) || !sc.s.castTargetsAll(sc.k, battlefieldOnly, &e, l) {
		t.Fatal("an opponent's creature permanent did not match")
	}
	minePerm := sc.perm(creature, 0)
	e.tgt[0] = TCode(minePerm)
	if sc.s.castTargetsAll(sc.k, anyZone, &e, l) {
		t.Fatal("the listener's own creature permanent matched an opponent filter")
	}

	// A creature spell the opponent controls: the target spell's stack entry
	// says who controls it, not its owner (seat 0 owns this one).
	spell := sc.obj(creature, 0)
	pushSpell(spell, 1)
	e.tgt[0] = TCode(spell)
	if !sc.s.castTargetsAll(sc.k, anyZone, &e, l) {
		t.Fatal("a creature spell an opponent controls did not match")
	}
	if sc.s.castTargetsAll(sc.k, battlefieldOnly, &e, l) {
		t.Fatal("the marker ignored: a battlefield-only alternative matched a spell")
	}

	// The same creature spell controlled by the listener does not match.
	sc.s.stack[sc.s.sp-1].ctrl = 0
	if sc.s.castTargetsAll(sc.k, anyZone, &e, l) {
		t.Fatal("a creature spell the listener controls matched an opponent filter")
	}
	sc.s.sp--

	// A noncreature spell an opponent controls does not match Creature: the
	// chain's player sentinel (Ctrl opp, no types) names no object.
	land := sc.obj(id["Plains"], 1)
	pushSpell(land, 1)
	e.tgt[0] = TCode(land)
	if sc.s.castTargetsAll(sc.k, anyZone, &e, l) {
		t.Fatal("a noncreature spell matched a creature alternative")
	}
	sc.s.sp--

	// A card in a graveyard is controlled by its owner.
	grave := sc.obj(creature, 1)
	sc.s.objs[grave].zone = kop.ZoneGraveyard
	e.tgt[0] = TCode(grave)
	if !sc.s.castTargetsAll(sc.k, anyZone, &e, l) {
		t.Fatal("an opponent's creature card in a graveyard did not match")
	}
	graveMine := sc.obj(creature, 0)
	sc.s.objs[graveMine].zone = kop.ZoneGraveyard
	e.tgt[0] = TCode(graveMine)
	if sc.s.castTargetsAll(sc.k, anyZone, &e, l) {
		t.Fatal("the listener's own creature card in a graveyard matched an opponent filter")
	}
}
