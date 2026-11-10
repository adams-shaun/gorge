package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

func arenaTable(ops ...kop.Op) *kop.Table {
	var prog []kop.Instr
	for _, op := range ops {
		prog = append(prog, kop.Instr{Op: op})
	}
	return &kop.Table{Cards: []kop.CardDef{{Name: "T", Abilities: []kop.Ability{{Kind: kop.AbActivated, Prog: prog}}}}}
}

// arenaPrep is a prep of tbl's arena and nothing else, with s laid out for it.
func arenaPrep(s *State, tbl *kop.Table) *prep {
	k := &prep{gen: &genPrep{}}
	prepArena(tbl, k.gen, &k.cold)
	k.cold.size()
	s.layOut(&k.cold)
	return k
}

// TestArenaAssign: a table's used mechanics get dense indices in enum
// order, and an unused one reads false through mSl.
func TestArenaAssign(t *testing.T) {
	ix, n, _ := arenaAssign(arenaTable())
	if n != 0 {
		t.Fatalf("empty table: n %d", n)
	}
	for m, v := range ix {
		if v != arenaNone {
			t.Fatalf("empty table: mech %d at %d", m, v)
		}
	}
	// regen alone is the first mask, not mechRegen's enum position
	ix, n, _ = arenaAssign(arenaTable(kop.OpRegenerate))
	if n != 1 || ix[mechRegen] != 0 || ix[mechGoad0] != arenaNone || ix[mechMonstrous] != arenaNone {
		t.Fatalf("regen only: ix %v n %d", ix, n)
	}
	// goad takes two masks (either player may goad) and monstrous one: the arena
	ix, n, _ = arenaAssign(arenaTable(kop.OpGoad, kop.OpMonstrosity))
	if n != 3 || ix[mechGoad0] != 0 || ix[mechGoad1] != 1 || ix[mechMonstrous] != 2 || ix[mechRegen] != arenaNone {
		t.Fatalf("goad+monstrous: ix %v n %d", ix, n)
	}
	// regen and kicker as well take a fifth, as many as the table uses
	five := arenaTable(kop.OpRegenerate, kop.OpGoad, kop.OpMonstrosity)
	five.Cards[0].Keywords = kop.KwKicker
	if _, n, _ = arenaAssign(five); n != 5 {
		t.Fatalf("five slot masks: n %d", n)
	}
	// kicked: Kicker, Bargain or Offspring on a card; exileEx: a damage op with the exile flag
	kk := arenaTable()
	kk.Cards[0].Keywords = kop.KwKicker
	if ix, n, _ = arenaAssign(kk); n != 1 || ix[mechKicked] != 0 || ix[mechExileEx] != arenaNone {
		t.Fatalf("kicker: ix %v n %d", ix, n)
	}
	kk = arenaTable()
	kk.Cards[0].Bargain = true
	if ix, _, _ = arenaAssign(kk); ix[mechKicked] != 0 {
		t.Fatalf("bargain: ix %v", ix)
	}
	kk = arenaTable()
	kk.Cards[0].Offspring = true
	if ix, _, _ = arenaAssign(kk); ix[mechKicked] != 0 {
		t.Fatalf("offspring: ix %v", ix)
	}
	kk = arenaTable(kop.OpDealDamageEx)
	if ix, n, _ = arenaAssign(kk); n != 0 || ix[mechExileEx] != arenaNone {
		t.Fatalf("damage without the exile flag: ix %v n %d", ix, n)
	}
	kk.Cards[0].Abilities[0].Prog[0].C = int16(kop.DmgExileIfDiesThisTurn)
	kk.Cards[0].Offspring = true
	if ix, n, _ = arenaAssign(kk); n != 2 || ix[mechExileEx] != 0 || ix[mechKicked] != 1 {
		t.Fatalf("exileEx+kicked: ix %v n %d", ix, n)
	}
	// suspected: a suspect brings in the mask, an un-suspect (C bit0) or a table
	// that only reads IsSuspected does not
	su := arenaTable()
	su.Cards[0].Abilities[0].Prog = []kop.Instr{kop.ExtInstr(kop.ExtSuspect)}
	if ix, n, _ = arenaAssign(su); n != 1 || ix[mechSuspected] != 0 {
		t.Fatalf("suspect: ix %v n %d", ix, n)
	}
	su.Cards[0].Abilities[0].Prog[0].C = 1
	if ix, n, _ = arenaAssign(su); n != 0 || ix[mechSuspected] != arenaNone {
		t.Fatalf("un-suspect: ix %v n %d", ix, n)
	}
	// mayhem: a Mayhem card brings in the object mask
	my := arenaTable()
	my.Cards[0].Mayhem = true
	if ix, n, no := arenaAssign(my); n != 0 || no != 1 || ix[mechMayhem] != 0 {
		t.Fatalf("mayhem: ix %v n %d no %d", ix, n, no)
	}
	// a trigger alone brings in monstrous
	tt := arenaTable()
	tt.Cards[0].Abilities = append(tt.Cards[0].Abilities, kop.Ability{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigBecomeMonstrous}})
	if ix, n, _ = arenaAssign(tt); n != 1 || ix[mechMonstrous] != 0 {
		t.Fatalf("trigger: ix %v n %d", ix, n)
	}

	// mSl through a prep: the used mechanic holds its bit, an unused one
	// reads false (and set/clr/zero are no-ops on it)
	var s State
	k := arenaPrep(&s, arenaTable(kop.OpRegenerate))
	s.mSlSet(k, mechRegen, 5)
	if !s.mSl(k, mechRegen, 5) || s.mSl(k, mechRegen, 6) {
		t.Fatal("regen bit")
	}
	s.mSlSet(k, mechMonstrous, 5)
	if s.mSl(k, mechMonstrous, 5) || s.mSl(k, mechGoad1, 5) {
		t.Fatal("a mechanic outside the table reads true")
	}
	s.mSlClr(k, mechRegen, 5)
	s.mSlSet(k, mechRegen, 7)
	s.mSlZero(k, mechRegen)
	if s.mSl(k, mechRegen, 5) || s.mSl(k, mechRegen, 7) || s.cold.ms[0].any() {
		t.Fatal("clear/zero")
	}
	if (&State{}).mSl(&prep{}, mechRegen, 1) {
		t.Fatal("a hand-written table (no genPrep) reads true")
	}
}

func stashTable() *kop.Table {
	t := arenaTable(kop.OpExileEx)
	t.Cards[0].Abilities[0].Prog[0].B.N = int16(kop.CtrStash)
	return t
}

// TestArenaAssignObj: the object-width mechanics get their own dense
// indices (stash; the two flashback planes together), and an ExileEx that is
// not a stash and a grant that is not flashback use none.
func TestArenaAssignObj(t *testing.T) {
	ix, ns, no := arenaAssign(stashTable())
	if ns != 0 || no != 1 || ix[mechStash] != 0 || ix[mechFBLo] != arenaNone || ix[mechFBHi] != arenaNone {
		t.Fatalf("stash only: ix %v ns %d no %d", ix, ns, no)
	}
	ix, ns, no = arenaAssign(fbGrantTable())
	if no != 2 || ix[mechFBLo] != 0 || ix[mechFBHi] != 1 || ix[mechStash] != arenaNone {
		t.Fatalf("flashback only: ix %v no %d", ix, no)
	}
	// stash plus both planes, beside slot masks
	both := stashTable()
	both.Cards[0].Abilities[0].Prog = append(both.Cards[0].Abilities[0].Prog, fbGrantTable().Cards[0].Abilities[0].Prog...)
	both.Cards[0].Abilities[0].Prog = append(both.Cards[0].Abilities[0].Prog, kop.Instr{Op: kop.OpRegenerate})
	ix, ns, no = arenaAssign(both)
	if ns != 1 || no != 3 || ix[mechStash] != 0 || ix[mechFBLo] != 1 || ix[mechFBHi] != 2 || ix[mechRegen] != 0 {
		t.Fatalf("stash+flashback+regen: ix %v ns %d no %d", ix, ns, no)
	}
	// the flashback helpers read both planes off fbLo's index (fbIx)
	if ix[mechFBHi] != ix[mechFBLo]+1 {
		t.Fatalf("fbHi %d is not right after fbLo %d", ix[mechFBHi], ix[mechFBLo])
	}
	for _, in := range []kop.Instr{{Op: kop.OpExileEx}, {Op: kop.OpGrantKeyword, B: kop.ValueRef{N: int16(kop.KwFlying)}}} {
		tt := arenaTable()
		tt.Cards[0].Abilities[0].Prog = []kop.Instr{in}
		if _, _, no = arenaAssign(tt); no != 0 {
			t.Fatalf("op %v: no %d, want no object mask", in.Op, no)
		}
	}
}

// TestMObj: the used object mechanic holds its bit, an unused one reads
// false and its set/clr/zero are no-ops, and a hand-written table (no
// genPrep) reads false.
func TestMObj(t *testing.T) {
	var s State
	k := arenaPrep(&s, stashTable())
	s.mObjSet(k, mechStash, 5)
	if !s.mObj(k, mechStash, 5) || s.mObj(k, mechStash, 6) {
		t.Fatal("stash bit")
	}
	s.mObjSet(k, mechFBLo, 5)
	if s.mObj(k, mechFBLo, 5) || s.mObj(k, mechFBHi, 5) {
		t.Fatal("a mechanic outside the table reads true")
	}
	s.mObjClr(k, mechStash, 5)
	s.mObjSet(k, mechStash, 7)
	s.mObjZero(k, mechStash)
	if s.mObj(k, mechStash, 5) || s.mObj(k, mechStash, 7) || s.cold.mo[0] != (objBits{}) {
		t.Fatal("clear/zero")
	}
	if (&State{}).mObj(&prep{}, mechStash, 1) {
		t.Fatal("a hand-written table (no genPrep) reads true")
	}
}

// TestArenaSlotReuseClears: a kicked creature (also marked exile-if-dies)
// leaves, a new permanent takes its slot in the same turn, and it reads
// not-kicked and not-exileEx: enterGen's one arenaClrSlot loop ends every
// mechanic's lifetime when the slot takes a new permanent (Fiery
// Annihilation's exile mark and Gnarlid Colony's kick share FDN's arena).
func TestArenaSlotReuseClears(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Gnarlid Colony", Count: 1}, {Name: "Fiery Annihilation", Count: 1}, {Name: "Forest", Count: 8}})
	k := sc.k
	for _, m := range []mech{mechKicked, mechExileEx} {
		if k.gen.ix[m] == arenaNone {
			t.Fatalf("%s is not in the table's arena", mechName[m])
		}
	}
	for i := 0; i < 5; i++ {
		sc.perm(id["Forest"], 0)
	}
	vl := sc.hand(id["Gnarlid Colony"], 0)
	sc.castMode(t, vl, 1)
	sc.resolveAll(t)
	sl := sc.slot(vl)
	if !sc.onBF(vl) || !sc.s.mSl(k, mechKicked, sl) {
		t.Fatal("a kicked cast did not enter marked kicked")
	}
	sc.s.mSlSet(k, mechExileEx, sl)
	sc.s.removePerm(k, sl, kop.ZoneGraveyard, false)
	sw := sc.perm(id["Forest"], 0)
	if sc.slot(sw) != sl {
		t.Fatalf("the new permanent took slot %d, want the vacated %d", sc.slot(sw), sl)
	}
	if sc.s.mSl(k, mechKicked, sl) || sc.s.mSl(k, mechExileEx, sl) {
		t.Fatal("a new permanent in a reused slot inherits kicked or exileEx")
	}
}

// TestArenaSlotReuseEndsExileRecords: the exile-instead record a dead creature
// left on its slot ends when a new permanent takes the slot (CR 400.7), so the
// new permanent's own mark is the only one its death competes with.
func TestArenaSlotReuseEndsExileRecords(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Gnarlid Colony", Count: 1}, {Name: "Fiery Annihilation", Count: 1}, {Name: "Forest", Count: 8}})
	k := sc.k
	for i := 0; i < 5; i++ {
		sc.perm(id["Forest"], 0)
	}
	old := sc.perm(id["Forest"], 0)
	sl := sc.slot(old)
	sc.s.markExileEx(k, sl)
	if sc.s.exMarkCount(sl) != 1 {
		t.Fatalf("precondition: %d records on the slot, want 1", sc.s.exMarkCount(sl))
	}
	sc.s.removePerm(k, sl, kop.ZoneExile, false)
	sw := sc.perm(id["Forest"], 0)
	if sc.slot(sw) != sl {
		t.Fatalf("the new permanent took slot %d, want the vacated %d", sc.slot(sw), sl)
	}
	sc.s.markExileEx(k, sl)
	if n := sc.s.exMarkCount(sl); n != 1 {
		t.Fatalf("%d exile-instead records on the reused slot, want only the new one", n)
	}
}
