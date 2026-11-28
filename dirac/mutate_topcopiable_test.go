package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestMutateTopCopiable is the kernel mirror of gorge's mutateTopCopiable
// (CR 730.2a; engine/events/apply_copy.go): a card mutated "on top" of a pile
// shows the mutating card. The merge is itself a layer-1 copiable effect
// timestamped at the merge, so it supersedes every copy effect applied before
// it (CR 613.2, 613.7): a token copy's own basis (cloneTokenBasis), a live
// permanent copy marker and an until-end-of-turn copy all leave the pile with
// the demoted top component.
func TestMutateTopCopiable(t *testing.T) {
	tbl, mutator, bird, _, _ := mutateTable()
	sprite := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Sprite", Types: kop.TCreature,
		Power: 1, Toughness: 1})
	wurm := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Copied Wurm", Types: kop.TCreature,
		Power: 6, Toughness: 6})

	newGame := func(t *testing.T) *scen {
		t.Helper()
		sc := newScenTable(t, tbl, Config{NoElide: true})
		sc.lands(0, v.Forest, 1)
		return sc
	}
	// wurmOnBattlefield mints the Copied Wurm token the copy layers read.
	wurmOnBattlefield := func(sc *scen) Obj {
		t.Helper()
		o := sc.s.newToken(wurm, 0)
		sc.s.enter(sc.k, o, 0, false, kop.ZoneNone)
		sc.s.npend = 0
		return o
	}
	// castTop casts the mutator on top of surv and checks the pile formed.
	castTop := func(t *testing.T, sc *scen, surv Obj) {
		t.Helper()
		mut := sc.hand(mutator, 0)
		mutateCast(t, sc, mut, surv, true)
		resolveStack(t, sc)
		if rec := sc.s.mergeRecOf(surv); rec == nil || rec.n != 1 {
			t.Fatalf("precondition: the pile did not form (%+v)", rec)
		}
	}
	// wantMutator checks the pile shows the mutating card and no copy override.
	wantMutator := func(t *testing.T, sc *scen, surv Obj) {
		t.Helper()
		sl := sc.slot(surv)
		if c := sc.s.cardAt(sl); c != mutator {
			t.Fatalf("pile shows card %d, want the mutator %d (CR 730.2a: the merge's copiable effect wins)", c, mutator)
		}
		if f := sc.s.perms[sl].ovr.flags; f&ovCopy != 0 {
			t.Fatalf("pile still carries the demoted top's copy override (ovr %#x)", f)
		}
		if !sc.s.isCreature(sc.k, sl) {
			t.Fatal("the pile is not a creature after the merge")
		}
	}

	t.Run("token copy basis", func(t *testing.T) {
		sc := newGame(t)
		// the template a copy-of-a-copy token is minted on: its printed card
		// is the Sprite
		tmpl := sc.s.newToken(sprite, 0)
		sc.s.enter(sc.k, tmpl, 0, false, kop.ZoneNone)
		tok := sc.s.newToken(wurm, 0)
		sc.s.enter(sc.k, tok, 0, false, kop.ZoneNone)
		sc.s.npend = 0
		sc.s.objs[tok].of |= ofCopy
		sc.s.cloneTokenBasis(sc.slot(tok), tok, sc.slot(tmpl))
		p := &sc.s.perms[sc.slot(tok)]
		if sc.s.cardAt(sc.slot(tok)) != wurm || p.card != sprite || p.ovr.flags&ovCopy == 0 ||
			sc.s.copyTop(sc.slot(tok), NoSlot).kind != copyNone {
			t.Fatalf("precondition: token %d shows %d on printed %d (ovr %#x), want a Wurm copy basis on the Sprite with no copy layer",
				tok, sc.s.cardAt(sc.slot(tok)), p.card, p.ovr.flags)
		}
		castTop(t, sc, tok)
		wantMutator(t, sc, tok)
	})

	t.Run("permanent copy marker", func(t *testing.T) {
		sc := newGame(t)
		host := sc.perm(bird, 0)
		sl := sc.slot(host)
		src := wurmOnBattlefield(sc)
		addCopyPerm(sc, sl, src, 0, 10)
		setCopy(sc, sl, wurm)
		if l := sc.s.copyTop(sl, NoSlot); l.kind != copyPerm || sc.s.cardAt(sl) != wurm {
			t.Fatalf("precondition: host layer kind %d card %d, want a live Wurm clone", l.kind, sc.s.cardAt(sl))
		}
		castTop(t, sc, host)
		wantMutator(t, sc, host)
	})

	t.Run("until-end-of-turn copy", func(t *testing.T) {
		sc := newGame(t)
		host := sc.perm(bird, 0)
		sl := sc.slot(host)
		addCopyEOT(sc, sl, wurm, NoObj, 0, 10)
		setCopy(sc, sl, wurm)
		if l := sc.s.copyTop(sl, NoSlot); l.kind != copyEOT || sc.s.cardAt(sl) != wurm {
			t.Fatalf("precondition: host layer kind %d card %d, want a live Wurm until-end-of-turn copy", l.kind, sc.s.cardAt(sl))
		}
		castTop(t, sc, host)
		wantMutator(t, sc, host)
		// The superseded copy still ends; the merge's later timestamp must
		// keep the mutating card.
		sc.s.cloneCleanup(sc.k)
		if sc.s.copyRec(sl, fxCopyEOT) >= 0 {
			t.Fatal("the ended until-end-of-turn layer is still recorded")
		}
		wantMutator(t, sc, host)
	})
}
