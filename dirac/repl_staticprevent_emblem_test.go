package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// embPrevScen is a table whose one emblem (Table.Emblems index 1, Ajani
// Steadfast's) prevents all but 1 of the damage to its player, beside the
// given hosts; the emblem is put into player 0's slot.
func embPrevScen(t *testing.T, hosts ...kop.Repl) (*scen, []CardID) {
	t.Helper()
	tb := newSPTable()
	var ids []CardID
	for _, r := range hosts {
		ids = append(ids, tb.host("Test Host", r))
	}
	tb.tbl.Emblems = append(append([]kop.Emblem{}, v.Table.Emblems...), kop.Emblem{Abilities: []kop.Ability{{
		Kind: kop.AbReplacement,
		Repl: kop.Repl{Kind: kop.ReplPreventDamageN, C: kop.ReplSPYou | kop.ReplSPAllBut, A: 1},
	}}})
	sc := tb.scen(t)
	if !sc.k.embPrev || !sc.k.gen.uses.has(useStaticPrev) {
		t.Fatal("precondition: the emblem's prevention did not reach the table's census")
	}
	idx := uint8(len(tb.tbl.Emblems))
	sc.s.g.emb[0][0], sc.s.g.embC[0][0], sc.s.g.embO[0][0], sc.s.g.embN[0][0], sc.s.g.embT[0][0] = idx, 0, NoObj, 0, 1
	sc.s.g.nemb[0] = 1
	return sc, ids
}

// TestEmbPrevAllButOne: an emblem's all-but-1 prevention reduces the damage to
// its player to 1 (and a 1-damage event stands), leaves the other player's
// alone, and survives every cleanup: it carries none of the lifetime flags.
func TestEmbPrevAllButOne(t *testing.T) {
	const you, them = TCode(PlayerTarget0), TCode(PlayerTarget0 + 1)
	sc, _ := embPrevScen(t)
	giant := sc.perm(v.HillGiant, 1)
	if got := spDeal(sc, 1, giant, you, 3); got != 1 {
		t.Fatalf("3 damage to the emblem's player = %d, want 1", got)
	}
	if got := spDeal(sc, 1, giant, you, 1); got != 1 {
		t.Fatalf("1 damage to the emblem's player = %d, want 1", got)
	}
	if got := spDeal(sc, 1, giant, them, 3); got != 3 {
		t.Fatalf("3 damage to the other player = %d, want 3", got)
	}
	for _, active := range []uint8{0, 1, 0} {
		sc.s.active = active
		sc.s.cleanupGen(sc.k)
	}
	if sc.s.g.nemb[0] != 1 {
		t.Fatalf("%d emblems after three cleanups, want 1: the permanent emblem was dropped", sc.s.g.nemb[0])
	}
	if got := spDeal(sc, 1, giant, you, 3); got != 1 {
		t.Fatalf("3 damage after the cleanups = %d, want 1", got)
	}
	DebugNoEmblemPrev = true
	defer func() { DebugNoEmblemPrev = false }()
	if got := spDeal(sc, 1, giant, you, 3); got != 3 {
		t.Fatalf("3 damage with the emblem never collected = %d, want 3", got)
	}
}

// TestEmbPrevNeverOrdered: an emblem candidate beside a second one stalls
// StallReplCands: no order ask names an emblem.
func TestEmbPrevNeverOrdered(t *testing.T) {
	const you = TCode(PlayerTarget0)
	sc, _ := embPrevScen(t, kop.Repl{Kind: kop.ReplPreventDamageN, C: kop.ReplSPYou | kop.ReplSPAllBut, A: 2})
	h := sc.perm(CardID(len(sc.k.t.Cards)-1), 0)
	giant := sc.perm(v.HillGiant, 1)
	if !sc.onBF(h) {
		t.Fatal("precondition: the host is not in play")
	}
	if got := spDeal(sc, 1, giant, you, 5); got != 5 {
		t.Fatalf("damage through a stalled competition = %d, want it unchanged", got)
	}
	if st := sc.s.g.stalled; st != StallReplCands {
		t.Fatalf("an emblem and a host candidate for one event stalled %s, want %s", StallName(st), StallName(StallReplCands))
	}
}

// TestEmbPrevStackable: a second -7 makes no second emblem (Stackable$
// False), so the one emblem applies alone.
func TestEmbPrevStackable(t *testing.T) {
	sc, _ := embPrevScen(t)
	if !sc.k.embIdem {
		t.Fatal("precondition: OpCreateEmblem does not read idemEmblemHeld on this table")
	}
	if !sc.s.idemEmblemHeld(sc.k, 0, uint8(len(sc.k.t.Emblems))) {
		t.Fatal("the held emblem is not read as held")
	}
	if sc.s.idemEmblemHeld(sc.k, 1, uint8(len(sc.k.t.Emblems))) {
		t.Fatal("the other player holds no such emblem")
	}
}
