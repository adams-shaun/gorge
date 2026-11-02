package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSkipNextUntap: OpSkipNextUntap keeps a tapped permanent tapped
// through its controller's next untap step and is consumed there (the
// following untap step untaps it); an untapped one consumes it too.
func TestSkipNextUntap(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 2}, {Name: "Plains", Count: 8}})
	fc := id["Valley Floodcaller"]
	a, b := sc.perm(fc, 0), sc.perm(fc, 0)
	sc.s.res.e.ctrl = 0
	for _, o := range []Obj{a, b} {
		sc.s.res.e.obj, sc.s.res.e.card = o, fc
		in := kop.Instr{Op: kop.OpSkipNextUntap, Obj: kop.RefSelf}
		sc.s.exec(sc.k, &in)
	}
	sc.s.perms[sc.slot(a)].flags |= pfTapped
	untap := func() {
		sc.s.active, sc.s.step = 0, kop.StepUntap
		sc.s.beginStep(sc.k)
	}
	untap()
	if !sc.tapped(a) || sc.s.perms[sc.slot(a)].flags&pfSkipUntap != 0 || sc.s.perms[sc.slot(b)].flags&pfSkipUntap != 0 {
		t.Fatal("the first untap step: the restricted permanent untapped, or the one-shot was not consumed")
	}
	untap()
	if sc.tapped(a) {
		t.Fatal("the second untap step: still tapped")
	}
}

// TestSkipUntap: OpSkipUntap (gorge's UntilTheEndOfYourNextUntap Untap
// replacement) keeps the marked permanent, and with C=1 the controller's
// lands, tapped through the controller's next untap step only; the next
// one untaps them.
func TestSkipUntap(t *testing.T) {
	deck := []cardset.Entry{{Name: "Savannah Lions", Count: 2}, {Name: "Plains", Count: 18}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	lc, pc := CardID(255), CardID(255)
	for i := range tbl.Cards {
		switch tbl.Cards[i].Name {
		case "Savannah Lions":
			lc = CardID(i)
		case "Plains":
			pc = CardID(i)
		}
	}
	skip := kop.Instr{Op: kop.OpSkipUntap, Obj: kop.RefSelf}
	lands := kop.Instr{Op: kop.OpSkipUntap, C: 1}
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Freeze", Types: kop.TSorcery, Abilities: []kop.Ability{
		{Kind: kop.AbSpell, Prog: []kop.Instr{skip, lands}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if !sc.k.gen.anySkipUntap {
		t.Fatalf("anySkipUntap %v, want true", sc.k.gen.anySkipUntap)
	}
	s := sc.s
	lion, other := sc.perm(lc, 0), sc.perm(lc, 0)
	land := sc.perm(pc, 0)
	for _, o := range []Obj{lion, other, land} {
		s.perms[sc.slot(o)].flags |= pfTapped
	}
	s.res.e.ctrl, s.res.e.obj, s.res.e.srcVer = 0, lion, s.objs[lion].ver
	for i := 0; i < 3; i++ {
		s.addRegenShield(sc.slot(lion)) // shields beyond the first live in perm.flags (pfRegenX)
	}
	s.exec(sc.k, &skip)
	if n := s.perms[sc.slot(lion)].regenShields(); n != 3 {
		t.Fatalf("the skip mark changed the regeneration shields: %d, want 3", n)
	}
	s.exec(sc.k, &lands)
	s.active, s.step = 0, kop.StepUntap
	s.beginStep(sc.k)
	if n := s.perms[sc.slot(lion)].regenShields(); n != 3 {
		t.Fatalf("the untap step changed the regeneration shields: %d, want 3", n)
	}
	if !sc.tapped(lion) || !sc.tapped(land) || sc.tapped(other) {
		t.Fatalf("next untap step: lion %v land %v other %v tapped, want true true false", sc.tapped(lion), sc.tapped(land), sc.tapped(other))
	}
	s.step = kop.StepUntap
	s.beginStep(sc.k)
	if sc.tapped(lion) || sc.tapped(land) {
		t.Fatal("the untap step after: still tapped")
	}
}
