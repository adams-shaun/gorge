package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

const (
	exhBear   CardID = 0
	exhGoblin CardID = 1 // 2/1 haste
	exhLamp   CardID = 2 // carries the ExtExHaste instruction (genPrep.anyExHaste)
)

var exhIn = kop.Instr{Op: kop.OpExt, Obj: kop.RefTarget0, A: kop.Const(int(kop.ExtExHaste))}

func exhScen(t *testing.T) *scen {
	t.Helper()
	tbl := &kop.Table{
		Cards: []kop.CardDef{
			{Name: "Bear", Types: kop.TCreature, Power: 2, Toughness: 2},
			{Name: "Goblin", Types: kop.TCreature, Power: 2, Toughness: 1, Keywords: kop.KwHaste},
			{Name: "Lamp", Types: kop.TArtifact, Abilities: []kop.Ability{{
				Kind: kop.AbActivated, Prog: []kop.Instr{exhIn},
			}}},
		},
		Filters: []kop.Filter{{}},
		Conds:   []kop.Cond{},
	}
	sc := newScenTableLib(t, tbl, exhBear, Config{})
	if sc.k.gen == nil || !sc.k.gen.anyExHaste {
		t.Fatal("the table is not a generated one with anyExHaste")
	}
	return sc
}

// exhGrant runs ExtExHaste over os the way execEachTarget does.
func exhGrant(sc *scen, os ...Obj) {
	in := exhIn
	for _, o := range os {
		sc.s.res.e.tgt[0], sc.s.res.e.tver[0] = TCode(o), sc.s.objs[o].ver
		sc.s.exHaste(sc.k, &in)
	}
}

// TestExHasteBlock: a recorded attacker is blockable only by creatures whose
// CURRENT keywords hold haste: a hasty creature may block, a haste-less one
// may not until it is given haste this turn, and the attacker's own haste
// (the Pump half) changes nothing.
func TestExHasteBlock(t *testing.T) {
	sc := exhScen(t)
	a, c := sc.perm(exhBear, 0), sc.perm(exhBear, 0)
	b, g := sc.perm(exhBear, 1), sc.perm(exhGoblin, 1)
	sl := sc.slot
	can := func(blk, atk Obj) bool { return sc.s.canBlock(sc.k, sl(blk), sl(atk)) }
	if !can(b, a) || !can(g, a) {
		t.Fatal("blockable before the record")
	}
	h0 := sc.s.genViewHash(0)
	exhGrant(sc, a)
	if can(b, a) {
		t.Error("a haste-less creature blocks the recorded attacker")
	}
	if !can(g, a) {
		t.Error("a hasty creature can't block the recorded attacker")
	}
	if !can(b, c) {
		t.Error("an unrecorded attacker is restricted")
	}
	sc.s.grantEOT(sl(a), kop.KwHaste) // Agility Bobblehead's Pump: the attacker's own haste
	if can(b, a) {
		t.Error("the attacker's haste lifts the restriction")
	}
	sc.s.grantEOT(sl(b), kop.KwHaste) // the blocker's current keywords decide
	if !can(b, a) {
		t.Error("a blocker given haste can't block")
	}
	if sc.s.genViewHash(0) == h0 {
		t.Error("the record is not in the state hash")
	}
}

// TestExHasteOutlivesAPartner: a record goes on when another permanent the effect
// named leaves (CR 611.2c), and a recorded permanent's slot taken by another one is
// no attacker of the record.
func TestExHasteOutlivesAPartner(t *testing.T) {
	sc := exhScen(t)
	a1, a2 := sc.perm(exhBear, 0), sc.perm(exhBear, 0)
	b := sc.perm(exhBear, 1)
	exhGrant(sc, a1, a2)
	slot := sc.slot(a1)
	sc.s.leave(sc.k, slot, kop.ZoneGraveyard, false)
	if !sc.s.exhOf(sc.slot(a2)) || sc.s.canBlock(sc.k, sc.slot(b), sc.slot(a2)) {
		t.Error("a2 lost its record with its partner's departure")
	}
	fresh := sc.perm(exhBear, 0)
	if sc.s.exhOf(sc.slot(fresh)) {
		t.Errorf("a creature that took slot %d (was %d) inherits the record", sc.slot(fresh), slot)
	}
}

// TestExHasteCleanup: the cleanup step ends the records and the state hash returns to its unrecorded value.
func TestExHasteCleanup(t *testing.T) {
	sc := exhScen(t)
	a1, a2 := sc.perm(exhBear, 0), sc.perm(exhBear, 0)
	b := sc.perm(exhBear, 1)
	h0 := sc.s.genViewHash(0)
	exhGrant(sc, a1, a2)
	if sc.s.canBlock(sc.k, sc.slot(b), sc.slot(a1)) {
		t.Fatal("the grant did not take")
	}
	sc.s.finishCleanup(sc.k)
	if !sc.s.canBlock(sc.k, sc.slot(b), sc.slot(a1)) || !sc.s.canBlock(sc.k, sc.slot(b), sc.slot(a2)) || sc.s.exhAny() {
		t.Errorf("cleanup left records %v", sc.s.cold.exH)
	}
	if sc.s.genViewHash(0) != h0 {
		t.Error("the hash differs from the unrecorded one after cleanup")
	}
}

// TestExHasteCapacity: a seventeenth record is a loud stall, never a dropped
// restriction.
func TestExHasteCapacity(t *testing.T) {
	sc := exhScen(t)
	var os []Obj
	for i := 0; i < maxExH+1; i++ {
		os = append(os, sc.perm(exhBear, 0))
	}
	exhGrant(sc, os...)
	if sc.s.StallReason() != StallExHaste {
		t.Errorf("stall = %d, want StallExHaste", sc.s.StallReason())
	}
}
