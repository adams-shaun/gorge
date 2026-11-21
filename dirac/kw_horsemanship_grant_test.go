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
	// Riding the Dilu Horse's Duration$ Permanent grant: an override record
	// carrying Horsemanship (no mark: the cleanup step clears those)
	permOv := len(tbl.Overrides)
	tbl.Overrides = append(tbl.Overrides, kop.Override{Horsemanship: true}, kop.Override{LoseAbilities: true}, kop.Override{Horsemanship: true})
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

	// a permanent grant (kop.Override.Horsemanship, Riding the Dilu Horse)
	// survives the cleanup step and ends as the permanent leaves (CR 400.7).
	// Fresh permanents: the mark section above left bear2 holding a mark and
	// bear and the loss flag behind.
	c, d := sc.slot(sc.perm(n, 0)), sc.slot(sc.perm(n, 1))
	if !can(d, c) {
		t.Fatal("precondition: no horsemanship anywhere on the fresh pair, want blockable")
	}
	sc.s.applyOverrideFl(sc.k, c, permOv, n, sc.s.ovDurFl(kop.DurPermanent))
	if sc.s.evasionAt(sc.k, c)&kop.EvHorsemanship == 0 {
		t.Fatal("the permanent record did not grant EvHorsemanship")
	}
	if can(d, c) {
		t.Error("a permanently granted attacker was blocked by a creature without horsemanship")
	}
	if !can(h, c) {
		t.Error("a permanently granted attacker was not blockable by a printed horsemanship creature")
	}
	sc.s.kwMarkClear(sc.k)
	sc.s.step = kop.StepMain2
	sc.s.cleanupGen(sc.k)
	if sc.s.evasionAt(sc.k, c)&kop.EvHorsemanship == 0 || can(d, c) {
		t.Error("the cleanup step ended a permanent horsemanship grant")
	}
	sc.s.removePerm(sc.k, c, kop.ZoneGraveyard, false)
	c2 := sc.slot(sc.perm(n, 0))
	if c2 != c {
		t.Fatalf("precondition: the new permanent took slot %d, want the freed slot %d", c2, c)
	}
	if sc.s.evasionAt(sc.k, c2)&kop.EvHorsemanship != 0 || !can(d, c2) {
		t.Error("a new permanent in the freed slot inherited the grant (CR 400.7)")
	}
	// layer 6: a record older than a loss of abilities grants nothing
	sc.s.applyOverrideFl(sc.k, c2, permOv, n, sc.s.ovDurFl(kop.DurPermanent))
	if sc.s.evasionAt(sc.k, c2)&kop.EvHorsemanship == 0 {
		t.Fatal("precondition: the second record did not grant EvHorsemanship")
	}
	sc.s.applyOverrideFl(sc.k, c2, permOv+1, n, sc.s.ovDurFl(kop.DurPermanent))
	if _, lost := sc.s.lossAt(sc.k, c2); !lost {
		t.Fatal("precondition: the loss-of-abilities record did not take hold")
	}
	if sc.s.evasionAt(sc.k, c2)&kop.EvHorsemanship != 0 {
		t.Error("a permanent grant older than a loss of abilities still held")
	}
	sc.s.applyOverrideFl(sc.k, c2, permOv+2, n, sc.s.ovDurFl(kop.DurPermanent)) // a second row: a re-application merges in place
	if sc.s.evasionAt(sc.k, c2)&kop.EvHorsemanship == 0 {
		t.Error("a permanent grant made after the loss of abilities did not hold (CR 613.7)")
	}
}
