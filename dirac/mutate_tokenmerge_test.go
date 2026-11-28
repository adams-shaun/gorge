package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestMutateTokenTopMerge is the kernel mirror of gorge's foldMutate token
// arm (CR 729.2 with CR 400.7; engine/events/apply_copy.go,
// apply_zone.go's component moves): mutating a creature TOKEN you own "on
// top" makes the pile a card permanent (the topmost component is the real
// mutating card), the demoted token card parks as a token object, and when
// the pile leaves the battlefield the mutating card reaches the graveyard as
// a card while the token component ceases to exist instead of following it.

func TestMutateTokenTopMerge(t *testing.T) {
	tbl, mutator, _, _, _ := mutateTable()
	// a non-Human creature card to mint the target token on
	sprite := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Sprite", Types: kop.TCreature,
		Power: 1, Toughness: 1})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	sc.lands(0, v.Forest, 1)
	// an EARLIER token, minted before the mutate target and so lower in the
	// object table: once the pile sheds its ofToken bit the bit-carrying
	// objects are no longer a contiguous suffix, and only the whole-table
	// cease walk still sees this one when it leaves (ceaseTokensGen's
	// tokenScanStart arm, gen.go -- a suffix walk stops at the pile).
	early := sc.s.newToken(sprite, 0)
	sc.s.enter(sc.k, early, 0, false, kop.ZoneNone)
	sc.s.npend = 0
	if sc.s.objs[early].of&ofToken == 0 || !sc.onBF(early) {
		t.Fatalf("precondition: object %d of %#x is not a battlefield token", early, sc.s.objs[early].of)
	}
	tok := sc.s.newToken(sprite, 0)
	sc.s.enter(sc.k, tok, 0, false, kop.ZoneNone)
	sc.s.npend = 0
	if sc.s.objs[tok].of&ofToken == 0 || !sc.onBF(tok) {
		t.Fatalf("precondition: object %d of %#x is not a battlefield token", tok, sc.s.objs[tok].of)
	}
	mut := sc.hand(mutator, 0)
	mutateCast(t, sc, mut, tok, true)
	resolveStack(t, sc)
	if c := sc.s.cardAt(sc.slot(tok)); c != mutator {
		t.Fatalf("precondition: top card %d, want the mutator %d", c, mutator)
	}
	rec := sc.s.mergeRecOf(tok)
	if rec == nil || rec.n != 1 {
		t.Fatalf("precondition: the pile did not form (%+v)", rec)
	}
	comp := Obj(rec.und[0] - 1)
	// CR 729.2: the top component is a card, so the pile is no longer a token
	// (nor a copy of what it was).
	if sc.s.objs[tok].of&ofToken != 0 {
		t.Fatalf("the pile object %d is still ofToken after a card went on top (of %#x)", tok, sc.s.objs[tok].of)
	}
	// the demoted component was the token's own card: it parks as a token
	// object.
	if sc.s.objs[comp].of&ofToken == 0 {
		t.Fatalf("the demoted component %d is not a token object (of %#x)", comp, sc.s.objs[comp].of)
	}
	// what the cease walk's two arms turn on: the earlier token carries its
	// bits BELOW the pile, and the pile's shed bits left what remains of the
	// token suffix holding only the parked component -- a suffix walk stops
	// there and never reaches the earlier token, so only the whole-table walk
	// can still cease it.
	if !(early < tok) {
		t.Fatalf("precondition: earlier token %d is not below the pile %d in the object table", early, tok)
	}
	i0 := int(sc.s.nobj)
	for i0 > 0 && sc.s.objs[i0-1].of&(ofToken|ofCopy) != 0 {
		i0--
	}
	if !(early < Obj(i0)) {
		t.Fatalf("precondition: the earlier token %d sits inside the table's token suffix (starts at %d of %d), so a suffix walk would cease it too and the whole-table walk is not what the last assertions pin",
			early, i0, sc.s.nobj)
	}
	// an object a LIVE pile names is not dead yet: object-id reuse must not
	// hand the component out while its pile is on the battlefield
	// (heldReferenced's merge-component scan, zone_objreuse.go).
	if !sc.s.heldReferenced(comp) {
		t.Fatalf("the token component %d of a live pile is not held by a reference", comp)
	}
	// the pile dies: the card pile reaches the graveyard, the token component
	// ceases (gorge's Move fold leaves it in ZCeased).
	sc.s.leave(sc.k, sc.slot(tok), kop.ZoneGraveyard, false)
	if z := sc.s.objs[tok].zone; z != kop.ZoneGraveyard {
		t.Fatalf("pile zone %d, want the graveyard", z)
	}
	if z := sc.s.objs[comp].zone; z != kop.ZoneNone {
		t.Fatalf("the token component moved to zone %d, want it to stay ceased (CR 400.7)", z)
	}
	// the graveyard holds the card pile and not the token, and the state-based
	// cease pass must leave the card pile where it is. The earlier token dies
	// in the same pass: it is the object below the shed pile that only the
	// whole-table cease walk still reaches.
	sc.s.leave(sc.k, sc.slot(early), kop.ZoneGraveyard, false)
	sc.s.ceaseTokensGen()
	if z := sc.s.objs[tok].zone; z != kop.ZoneGraveyard {
		t.Fatalf("ceaseTokensGen ceased the card pile: zone %d, want the graveyard", z)
	}
	if z := sc.s.objs[early].zone; z != kop.ZoneNone {
		t.Fatalf("the earlier token %d survived the cease pass in zone %d, want it ceased (CR 704.5d)", early, z)
	}
	pl := &sc.s.pl[0]
	for i := 0; i < int(pl.graveN); i++ {
		if pl.grave[i] == comp {
			t.Fatalf("the token component %d is on the graveyard list", comp)
		}
		if pl.grave[i] == early {
			t.Fatalf("the ceased earlier token %d is still on the graveyard list", early)
		}
	}
	// with the pile gone nothing names the component any more: a component of
	// a dead pile must become reusable, not stay held forever (mergeLeave
	// zeroes the record before the split).
	if sc.s.heldReferenced(comp) {
		t.Fatalf("the component %d of a pile that left the battlefield is still held by a reference", comp)
	}
}
