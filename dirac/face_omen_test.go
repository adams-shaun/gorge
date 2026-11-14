package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Omen cards (face_omen.go) on the generated omen pool: the face cast
// from the hand, the bottom-of-library rest and shuffle, the graveyard rest
// of a countered or fizzled Omen and the face reset on a reversal.

func omenScen(t *testing.T, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 8}}
	set := cardsettest.Set(t, "omen")
	for _, n := range names {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Fatalf("%s not supported in the omen table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	if !sc.k.gen.anyFaces {
		t.Fatal("anyFaces not set")
	}
	return sc, id
}

// plainCasts reports whether the plain (front-face) cast of o is offered.
func plainCasts(d *Decision, o Obj) bool {
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(o)) && a.Mode()&castFace == 0 {
			return true
		}
	}
	return false
}

// TestOmenOffered: with the Omen face's cost floating, an Omen card in
// hand offers its face cast; the front's plain cast is still offered when
// affordable; a sorcery face is not offered at instant speed, an instant
// face is.
func TestOmenOffered(t *testing.T) {
	sc, id := omenScen(t, "Dirgur Island Dragon", "Twinmaw Stormbrood", "Grizzly Bears", "Island")
	for i := 0; i < 6; i++ {
		sc.perm(id["Island"], 0)
	}
	sc.perm(id["Grizzly Bears"], 0) // Charring Bite targets a creature without flying
	dirgur := sc.hand(id["Dirgur Island Dragon"], 0)
	twinmaw := sc.hand(id["Twinmaw Stormbrood"], 0)
	sc.s.pl[0].pool[kop.U] = 2
	sc.s.pl[0].pool[kop.R] = 2
	d := sc.s.Step()
	if got := faceCasts(&d, dirgur); len(got) != 1 || got[0] != 1 {
		t.Fatalf("Dirgur face casts %v, want [1]", got)
	}
	if !plainCasts(&d, dirgur) {
		t.Fatal("Dirgur's plain cast is not offered with six Islands")
	}
	if got := faceCasts(&d, twinmaw); len(got) != 1 || got[0] != 1 {
		t.Fatalf("sorcery face casts %v at sorcery speed, want [1]", got)
	}
	sc.s.step = kop.StepUpkeep
	d = sc.s.Step()
	if got := faceCasts(&d, dirgur); len(got) != 1 || got[0] != 1 {
		t.Fatalf("instant face not offered at instant speed: %v", got)
	}
	if got := faceCasts(&d, twinmaw); len(got) != 0 {
		t.Fatalf("sorcery face offered at instant speed: %v", got)
	}
}

// TestOmenFlashFace: a front with flash (Whirlwing Stormbrood) does not lend it
// to its sorcery Omen face (CR 715.3a, 720.3a).
func TestOmenFlashFace(t *testing.T) {
	sc, id := omenScen(t, "Whirlwing Stormbrood", "Grizzly Bears")
	sc.perm(id["Grizzly Bears"], 0) // Dynamic Soar targets a creature you control
	card := sc.hand(id["Whirlwing Stormbrood"], 0)
	sc.s.pl[0].pool[kop.G] = 3
	sc.s.step = kop.StepUpkeep
	d := sc.s.Step()
	if got := faceCasts(&d, card); len(got) != 0 {
		t.Fatalf("flash front's sorcery face at instant speed: %v", got)
	}
}

// TestOmenResolveShuffles: a resolved Omen is in its owner's library as its
// front, not the graveyard, at the bottom before its owner's shuffle; the
// resulting library equals the pre-resolution library plus the card at the
// bottom, shuffled with the same rng state.
func TestOmenResolveShuffles(t *testing.T) {
	sc, id := omenScen(t, "Riling Dawnbreaker")
	front := id["Riling Dawnbreaker"]
	card := sc.hand(front, 0)
	sc.s.pl[0].pool[kop.W] = 2
	d := sc.s.Step()
	if got := faceCasts(&d, card); len(got) != 1 || got[0] != 1 {
		t.Fatalf("face casts %v, want [1]", got)
	}
	sc.s.Apply(MakeAction(AkCast, TCode(card), 1, castFace))
	sc.buildToStack(t)
	pl := &sc.s.pl[0]
	before := append([]Obj(nil), pl.lib[pl.libLo:pl.libHi]...)
	rngBefore := sc.s.rng
	sc.resolveAll(t)
	ob := &sc.s.objs[card]
	if ob.zone != kop.ZoneLibrary || ob.card != front {
		t.Fatalf("resolved Omen: zone %d, front %v", ob.zone, ob.card == front)
	}
	if pl.libLen() != len(before)+1 {
		t.Fatalf("library %d, want %d", pl.libLen(), len(before)+1)
	}
	exp := append(append([]Obj(nil), before...), card)
	r := rngBefore
	for i := len(exp) - 1; i > 0; i-- {
		j := r.IntN(i + 1)
		exp[i], exp[j] = exp[j], exp[i]
	}
	got := pl.lib[pl.libLo:pl.libHi]
	for i := range exp {
		if got[i] != exp[i] {
			t.Fatalf("library[%d] = %d, want %d", i, got[i], exp[i])
		}
	}
}

// TestOmenCounteredToGraveyard: a countered Omen goes to its owner's
// graveyard as its front, with no shuffle.
func TestOmenCounteredToGraveyard(t *testing.T) {
	sc, id := omenScen(t, "Dirgur Island Dragon")
	front := id["Dirgur Island Dragon"]
	card := sc.hand(front, 0)
	sc.s.pl[0].pool[kop.U] = 2
	sc.s.Step()
	sc.s.Apply(MakeAction(AkCast, TCode(card), 1, castFace))
	sc.buildToStack(t)
	rngBefore := sc.s.rng
	sc.s.counterAt(sc.k, 0)
	ob := &sc.s.objs[card]
	if ob.zone != kop.ZoneGraveyard || ob.card != front {
		t.Fatalf("countered Omen: zone %d, front %v", ob.zone, ob.card == front)
	}
	if sc.s.rng != rngBefore {
		t.Fatal("a countered Omen shuffled")
	}
}

// TestOmenFizzledToGraveyard: a fizzled Omen goes to its owner's graveyard
// as its front, with no shuffle.
func TestOmenFizzledToGraveyard(t *testing.T) {
	sc, id := omenScen(t, "Dirgur Island Dragon")
	front := id["Dirgur Island Dragon"]
	face := sc.k.t.Cards[front].Faces[1]
	o := sc.obj(front, 0)
	sc.s.objs[o].card = face
	sc.s.objs[o].zone = kop.ZoneStack
	e := stackEntry{kind: seSpell, obj: o, ctrl: 0}
	sc.s.sp = 1
	sc.s.stack[0] = e
	sc.s.res = resCtx{e: e, active: true, lastTok: NoObj, eachObj: NoObj}
	rngBefore := sc.s.rng
	sc.s.finishResolve(sc.k, true)
	ob := &sc.s.objs[o]
	if ob.zone != kop.ZoneGraveyard || ob.card != front {
		t.Fatalf("fizzled Omen: zone %d, front %v", ob.zone, ob.card == front)
	}
	if sc.s.rng != rngBefore {
		t.Fatal("a fizzled Omen shuffled")
	}
}

// TestOmenReversalTurnsFront: an undone Omen face cast returns the card to
// the hand as its front (gorge pc.faceBefore, CR 733.1).
func TestOmenReversalTurnsFront(t *testing.T) {
	sc, id := omenScen(t, "Dirgur Island Dragon")
	front := id["Dirgur Island Dragon"]
	card := sc.hand(front, 0)
	sc.s.pl[0].pool[kop.U] = 2
	sc.s.Step()
	sc.s.Apply(MakeAction(AkCast, TCode(card), 1, castFace))
	sc.s.Step()
	sc.s.reverseCast(sc.k)
	if ob := &sc.s.objs[card]; ob.zone != kop.ZoneHand || ob.card != front {
		t.Fatalf("undone Omen: zone %d, front %v", ob.zone, ob.card == front)
	}
}
