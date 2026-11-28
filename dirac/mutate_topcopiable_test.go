package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestMutateTopCopiable is the kernel mirror of gorge's mutateTopCopiable
// (CR 730.2a; engine/events/apply_copy.go): a card mutated "on top" of a
// token copy whose copy basis is a layer-1 override (cloneTokenBasis: minted
// on the template's printed card, the copied card as ovCopy) shows the
// mutating card, not the demoted token's copied face.
func TestMutateTopCopiable(t *testing.T) {
	tbl, mutator, _, _, _ := mutateTable()
	sprite := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Sprite", Types: kop.TCreature,
		Power: 1, Toughness: 1})
	wurm := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Copied Wurm", Types: kop.TCreature,
		Power: 6, Toughness: 6})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	sc.lands(0, v.Forest, 1)
	// the template a copy-of-a-copy token is minted on: its printed card is
	// the Sprite
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
	mut := sc.hand(mutator, 0)
	mutateCast(t, sc, mut, tok, true)
	resolveStack(t, sc)
	if rec := sc.s.mergeRecOf(tok); rec == nil || rec.n != 1 {
		t.Fatalf("precondition: the pile did not form (%+v)", rec)
	}
	sl := sc.slot(tok)
	if c := sc.s.cardAt(sl); c != mutator {
		t.Fatalf("pile shows card %d, want the mutator %d (CR 730.2a: the old top's copy basis leaves with it)", c, mutator)
	}
	if f := sc.s.perms[sl].ovr.flags; f&ovCopy != 0 {
		t.Fatalf("pile still carries the demoted token's copy override (ovr %#x)", f)
	}
	if !sc.s.isCreature(sc.k, sl) {
		t.Fatal("the pile is not a creature after the merge")
	}
}
