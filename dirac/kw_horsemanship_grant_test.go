package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestGrantedHorsemanship: an until-end-of-turn horsemanship mark
// (ExtKwMark KwMarkHorsemanship, Riding Red Hare) makes its permanent
// unblockable except by a creature with horsemanship (CR 702.31b), printed or
// granted; the mark's holder blocks a horsemanship attacker like any
// horsemanship creature; the cleanup step ends the mark (CR 514.2); and a
// mark made while its permanent had lost its abilities holds through the loss
// (CR 613.7) where a mark made before it does not.
func TestGrantedHorsemanship(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	n := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2},
		kop.CardDef{Name: "Test Horse Blocker", Types: kop.TCreature, Power: 2, Toughness: 2, Evasion: kop.EvHorsemanship},
		// the mark's source: its program arms the arena's horsemanship masks
		kop.CardDef{Name: "Test Granter", Types: kop.TInstant, Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{
			{Op: kop.OpExt, A: kop.Const(int(kop.ExtKwMark)), Obj: kop.RefTarget0, C: kop.KwMarkHorsemanship}}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	bear, bear2, horse := sc.perm(n, 0), sc.perm(n, 1), sc.perm(n+1, 1)
	a, b, h := sc.slot(bear), sc.slot(bear2), sc.slot(horse)
	can := func(blk, att Slot) bool { return sc.s.canBlock(sc.k, blk, att) }
	if !can(b, a) || !can(h, a) {
		t.Fatal("no horsemanship anywhere: want both blockable")
	}
	sc.s.kwMark(sc.k, a, kop.KwMarkHorsemanship)
	if sc.s.evasionAt(sc.k, a)&kop.EvHorsemanship == 0 {
		t.Fatal("the mark did not grant EvHorsemanship")
	}
	if can(b, a) {
		t.Error("a marked attacker was blocked by a creature without horsemanship")
	}
	if !can(h, a) {
		t.Error("a marked attacker was not blockable by a printed horsemanship creature")
	}
	sc.s.kwMark(sc.k, b, kop.KwMarkHorsemanship)
	if !can(b, a) {
		t.Error("a creature granted horsemanship could not block a marked attacker")
	}
	sc.s.kwMarkClear(sc.k)
	if sc.s.evasionAt(sc.k, a)&kop.EvHorsemanship != 0 || !can(b, a) {
		t.Error("the cleanup step left the mark in force")
	}
	// the printed keyword is the horse's own: it is still an attacker's restriction
	if can(b, h) {
		t.Error("a printed horsemanship attacker was blocked by a creature without it")
	}
	sc.s.perms[a].ovr.flags |= ovLoseAllEOT
	sc.s.kwMark(sc.k, a, kop.KwMarkHorsemanship)
	if sc.s.evasionAt(sc.k, a)&kop.EvHorsemanship == 0 {
		t.Error("a mark made after the loss of abilities did not hold (CR 613.7)")
	}
}
