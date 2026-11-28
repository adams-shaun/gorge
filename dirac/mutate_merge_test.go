package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// Mutate review-round tests (docs/design/mutate.md; gorge
// engine/rules/mutate.go, stack.go's resolveTop diversion and
// events.foldMutate/Move): the cast-order and resolution-diversion
// lockstep points, the under-card trigger union, and the pile leaving
// beneath a graveyard-to-exile replacement.

// mutateSelfTrig gives card c "whenever this creature mutates, put a quest
// counter on it" (TfSelf, Kologran's shape).
func mutateSelfTrig(tbl *kop.Table, c CardID) {
	tbl.Cards[c].Abilities = append(tbl.Cards[c].Abilities, kop.Ability{Kind: kop.AbTriggered,
		Trig: kop.Trigger{Event: kop.TrigMutates, Flags: kop.TfSelf},
		Prog: []kop.Instr{{Op: kop.OpPutCounter, Obj: kop.RefTriggerObj,
			A: kop.Const(1), C: int16(kop.CtrQuest)}}})
}

// TestMutateTrigUnionFires: CR 702.140d — the pile has the triggers of every
// component, so "whenever this creature mutates" on a card UNDER the top
// (the mutating card cast under, or a demoted top) fires on the merge.
func TestMutateTrigUnionFires(t *testing.T) {
	t.Run("mutating card cast under", func(t *testing.T) {
		tbl, mutator, bird, _, _ := mutateTable()
		mutateSelfTrig(tbl, mutator)
		sc := newScenTable(t, tbl, Config{NoElide: true})
		sc.lands(0, v.Forest, 1)
		b := sc.perm(bird, 0)
		mut := sc.hand(mutator, 0)
		mutateCast(t, sc, mut, b, false)
		resolveStack(t, sc)
		if c := sc.s.cardAt(sc.slot(b)); c != bird {
			t.Fatalf("precondition: top card %d, want the bird %d (cast under)", c, bird)
		}
		if n := sc.s.perms[sc.slot(b)].ctrLow(int(kop.CtrQuest)); n != 1 {
			t.Fatalf("under-card trigger counters %d, want 1", n)
		}
	})
	t.Run("demoted top", func(t *testing.T) {
		tbl, mutator, bird, _, _ := mutateTable()
		mutateSelfTrig(tbl, bird)
		sc := newScenTable(t, tbl, Config{NoElide: true})
		sc.lands(0, v.Forest, 1)
		b := sc.perm(bird, 0)
		mut := sc.hand(mutator, 0)
		mutateCast(t, sc, mut, b, true)
		resolveStack(t, sc)
		if c := sc.s.cardAt(sc.slot(b)); c != mutator {
			t.Fatalf("precondition: top card %d, want the mutator %d (cast over)", c, mutator)
		}
		if n := sc.s.perms[sc.slot(b)].ctrLow(int(kop.CtrQuest)); n != 1 {
			t.Fatalf("demoted-top trigger counters %d, want 1", n)
		}
	})
}

// TestMutateNoEntryAsks: a mutating spell never enters, so no entry
// machinery runs (gorge diverts to resolveMutate before it): a printed
// Riot on the mutating card poses no election when it merges.
func TestMutateNoEntryAsks(t *testing.T) {
	tbl, mutator, bird, _, _ := mutateTable()
	tbl.Cards[mutator].Riot = true
	sc := newScenTable(t, tbl, Config{NoElide: true})
	sc.lands(0, v.Forest, 1)
	b := sc.perm(bird, 0)
	mut := sc.hand(mutator, 0)
	mutateCast(t, sc, mut, b, true)
	for i := 0; i < 16 && sc.s.sp > 0; i++ {
		d := sc.s.Step()
		if d.Kind != DkPriority {
			t.Fatalf("entry ask of kind %d while a mutating spell resolves", d.Kind)
		}
		sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	if sc.s.mergeRecOf(b) == nil {
		t.Fatalf("precondition: the spell did not merge")
	}
}

// TestMutateLeaveUnderExileReplacement: a merged permanent dying under an
// any-zone graveyard-to-exile replacement (Rest in Peace) exiles every
// component, as gorge's rewritten move does (leave's dieZone settles the zone
// before removePerm, so the components follow the top). A graveyard move that
// reaches removePerm unsettled would exile the top through placeFromK while
// the parked components reached the graveyard: as meldLeave does, the kernel
// stalls rather than split the pile.
func TestMutateLeaveUnderExileReplacement(t *testing.T) {
	build := func(t *testing.T) (*scen, Obj, Obj) {
		tbl, mutator, bird, _, _ := mutateTable()
		cf := kop.FilterID(len(tbl.Filters))
		tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
		rest := CardID(len(tbl.Cards))
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Rest", Types: kop.TEnchantment,
			Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplExileOnLeave, Filter: cf, C: 1}}}})
		sc := newScenTable(t, tbl, Config{NoElide: true})
		if sc.k.gen == nil || sc.k.gen.exileAnyC == nil {
			t.Fatalf("precondition: no any-zone exile replacement in the table")
		}
		sc.lands(0, v.Forest, 1)
		b := sc.perm(bird, 0)
		mut := sc.hand(mutator, 0)
		mutateCast(t, sc, mut, b, true)
		resolveStack(t, sc)
		rec := sc.s.mergeRecOf(b)
		if rec == nil || rec.n != 1 {
			t.Fatalf("precondition: the pile did not form")
		}
		comp := Obj(rec.und[0] - 1)
		sc.perm(rest, 0)
		return sc, b, comp
	}
	t.Run("leave settles the zone", func(t *testing.T) {
		sc, b, comp := build(t)
		sc.s.leave(sc.k, sc.slot(b), kop.ZoneGraveyard, false)
		if zt, zc := sc.s.objs[b].zone, sc.s.objs[comp].zone; zt != kop.ZoneExile || zc != kop.ZoneExile {
			t.Fatalf("top zone %d, component zone %d, want both exile", zt, zc)
		}
	})
	t.Run("unsettled graveyard move stalls", func(t *testing.T) {
		sc, b, comp := build(t)
		sc.s.removePerm(sc.k, sc.slot(b), kop.ZoneGraveyard, false)
		if !sc.s.Stalled() || sc.s.StallReason() != StallMerge {
			t.Fatalf("stalled %v reason %d, want StallMerge", sc.s.Stalled(), sc.s.StallReason())
		}
		if z := sc.s.objs[comp].zone; z != kop.ZoneNone {
			t.Fatalf("component moved to zone %d under a stalled split", z)
		}
	})
}
