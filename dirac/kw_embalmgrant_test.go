package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// embalmOffered reports whether the next decision offers p's granted embalm
// of the graveyard card o.
func (sc *scen) embalmOffered(o Obj) bool {
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && a.Source() == TCode(o) && a.Mode() == EmbalmGrantMode {
			return true
		}
	}
	return false
}

// embalmBattlefield counts the battlefield's permanents and the tapped ones.
func (sc *scen) embalmBattlefield() (n, tapped int) {
	for o := Obj(0); int(o) < int(sc.s.nobj); o++ {
		if sc.s.objs[o].zone == kop.ZoneBattlefield {
			n++
			if sc.s.perms[sc.slot(o)].flags&pfTapped != 0 {
				tapped++
			}
		}
	}
	return
}

// embalmBoard is Cursecloth Wrappings and forests Forests on p0's battlefield
// and a creature card in p0's graveyard with the pump resolved on it: the
// state a granted embalm is offered in. The preconditions the assertions
// depend on are checked here: nothing is offered or granted before the pump,
// and the grant is there after it.
func embalmBoard(t *testing.T, tbl *kop.Table, id map[string]CardID, forests int) (sc *scen, bears Obj) {
	t.Helper()
	sc = newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	if !sc.k.anyEmbalmGrant {
		t.Fatal("anyEmbalmGrant unset: the table lowers no OpGrantEmbalmOwnCost")
	}
	cw := sc.perm(id["Cursecloth Wrappings"], 0)
	for i := 0; i < forests; i++ {
		sc.perm(id["Forest"], 0)
	}
	bears = sc.grave(id["Grizzly Bears"], 0, 1)[0]
	if sc.s.embalmGranted(sc.k, bears) || sc.embalmOffered(bears) {
		t.Fatal("precondition: the card holds a granted embalm before the pump")
	}
	sc.do(t, AkActivate, TCode(cw), scAny)
	sc.do(t, AkTarget, scAny, TCode(bears))
	sc.do(t, AkPass, scAny, scAny)
	sc.do(t, AkPass, scAny, scAny)
	if !sc.s.embalmGranted(sc.k, bears) {
		t.Fatal("the pump resolved without granting the embalm")
	}
	return sc, bears
}

// CheckEmbalmGrant (CR 702.128a, Cursecloth Wrappings): the granted embalm is
// offered for the creature card in its owner's graveyard at sorcery speed
// when the card's own mana cost ({1}{G}) is payable, and only then; it is not
// offered at instant speed, one Forest short, after the card leaves the
// graveyard (a new object, CR 400.7) or after the cleanup ends the grant.
// Activating it charges the card's own mana cost, exiles the card and puts a
// token copy on the battlefield.
func CheckEmbalmGrant(t *testing.T, tbl *kop.Table, id map[string]CardID) {
	t.Helper()

	t.Run("offered at sorcery speed for the cards own cost", func(t *testing.T) {
		sc, bears := embalmBoard(t, tbl, id, 2)
		if !sc.embalmOffered(bears) {
			t.Fatal("granted embalm not offered with {1}{G} available at sorcery speed")
		}
		sc.s.step = kop.StepBeginCombat
		if sc.embalmOffered(bears) {
			t.Fatal("granted embalm offered outside a main phase")
		}
	})

	t.Run("one Forest short", func(t *testing.T) {
		sc, bears := embalmBoard(t, tbl, id, 1)
		if sc.embalmOffered(bears) {
			t.Fatal("granted embalm offered with one Forest: the cost is {1}{G}, not the body's placeholder")
		}
	})

	t.Run("activation pays, exiles the card and mints a token", func(t *testing.T) {
		sc, bears := embalmBoard(t, tbl, id, 2)
		perms, _ := sc.embalmBattlefield()
		sc.do(t, AkActivate, TCode(bears), TCode(0))
		sc.do(t, AkPass, scAny, scAny)
		sc.do(t, AkPass, scAny, scAny)
		sc.s.Step()
		if sc.s.objs[bears].zone == kop.ZoneGraveyard {
			t.Fatal("the embalmed card is still in the graveyard: the exile-self cost was not paid")
		}
		after, tapped := sc.embalmBattlefield()
		var tok Obj
		for o := Obj(0); int(o) < int(sc.s.nobj); o++ {
			if o != bears && sc.s.objs[o].card == sc.s.objs[bears].card && sc.s.objs[o].zone == kop.ZoneBattlefield {
				tok = o
			}
		}
		if sc.s.sp != 0 {
			t.Fatalf("the embalm ability is still on the stack (%d entries)", sc.s.sp)
		}
		if after != perms+1 || tok == 0 {
			t.Fatalf("battlefield permanents %d -> %d, token %d: want one new copy of the card", perms, after, tok)
		}
		if tapped != 3 { // Cursecloth's pump and the two Forests paying {1}{G}
			t.Fatalf("%d permanents tapped, want 3 (the pump and {1}{G})", tapped)
		}
	})

	t.Run("the copy is a white Zombie with no mana cost", func(t *testing.T) {
		c, ab, ok := embalmBody(tbl)
		if !ok {
			t.Fatal("the table has no embalm body")
		}
		var ov *kop.Override
		for _, in := range tbl.Cards[c].Abilities[ab].Prog {
			if x := copyExcOf(&in); x > 0 {
				ov = &tbl.Overrides[x-1]
			}
		}
		if ov == nil {
			t.Fatal("the embalm body's copy carries no exception record")
		}
		if !ov.HasColors || ov.SetColors != kop.ColW || ov.AddSub == (kop.SubMask{}) || !ov.NoCost {
			t.Fatalf("copy exception %+v: want colour white, an added subtype (Zombie) and no mana cost", *ov)
		}
	})

	t.Run("a zone change ends the grant", func(t *testing.T) {
		sc, bears := embalmBoard(t, tbl, id, 2)
		sc.s.place(bears, kop.ZoneHand, false)
		sc.s.place(bears, kop.ZoneGraveyard, false)
		if sc.s.embalmGranted(sc.k, bears) || sc.embalmOffered(bears) {
			t.Fatal("the grant outlived a trip through the hand")
		}
	})

	t.Run("the cleanup ends the grant", func(t *testing.T) {
		sc, bears := embalmBoard(t, tbl, id, 2)
		sc.s.cleanupGen(sc.k)
		if sc.s.embalmGranted(sc.k, bears) || sc.embalmOffered(bears) {
			t.Fatal("the grant outlived the cleanup step")
		}
	})
}
