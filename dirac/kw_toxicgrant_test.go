package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestGrantedToxicReads: toxicAt adds every grant to the printed toxic (CR
// 702.164c). A mark (ExtKwMark KwMarkToxic, N in C's high byte) adds N and a
// second mark adds again, the cleanup step ends the marks, an override
// record's Toxic (Monument to Perfection) survives the cleanup step and ends
// with the permanent, a record older than a loss of abilities grants nothing
// and a mark made while the abilities were lost stalls (StallToxicGrant).
func TestGrantedToxicReads(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	n := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Toxic Bear", Types: kop.TCreature, Power: 2, Toughness: 2, Toxic: 2},
		kop.CardDef{Name: "Test Granter", Types: kop.TInstant, Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{
			{Op: kop.OpExt, A: kop.Const(int(kop.ExtKwMark)), Obj: kop.RefTarget0, C: kop.KwMarkToxic | 1<<kop.KwMarkToxicShift}}}}})
	permOv := len(tbl.Overrides)
	tbl.Overrides = append(tbl.Overrides, kop.Override{Toxic: 9}, kop.Override{LoseAbilities: true}, kop.Override{Toxic: 3})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if !sc.k.gen.anyInfectGrant || !sc.k.gen.anyInfect {
		t.Fatalf("anyInfectGrant %v anyInfect %v, want both", sc.k.gen.anyInfectGrant, sc.k.gen.anyInfect)
	}
	a := sc.slot(sc.perm(n, 0))
	tox := func(sl Slot) int { return sc.s.toxicAt(sc.k, sl) }
	if tox(a) != 2 {
		t.Fatalf("printed toxic 2: toxicAt %d", tox(a))
	}
	sc.s.kwMark(sc.k, a, kop.KwMarkToxic)
	sc.s.toxicMark(sc.k, a, 1)
	sc.s.toxicMark(sc.k, a, 2)
	if tox(a) != 5 {
		t.Errorf("printed 2 + marks 1 and 2: toxicAt %d, want 5", tox(a))
	}
	DebugNoGrantedToxic = true
	got := tox(a)
	DebugNoGrantedToxic = false
	if got != 2 {
		t.Errorf("DebugNoGrantedToxic: toxicAt %d, want the printed 2", got)
	}
	sc.s.toxicMark(sc.k, a, 5)
	if !sc.s.Stalled() || sc.s.StallReason() != StallToxicGrant {
		t.Errorf("a mark past seven: stalled %v reason %s, want %s", sc.s.Stalled(), StallName(sc.s.StallReason()), StallName(StallToxicGrant))
	}
	sc.s.g.stalled = 0
	sc.s.step = kop.StepMain2
	sc.s.finishCleanup(sc.k)
	if tox(a) != 2 {
		t.Errorf("after the cleanup step: toxicAt %d, want the printed 2", tox(a))
	}

	// an override record's grant survives the cleanup step
	b := sc.slot(sc.perm(n, 1))
	sc.s.applyOverrideFl(sc.k, b, permOv, n, sc.s.ovDurFl(kop.DurPermanent))
	if tox(b) != 2+9 {
		t.Fatalf("printed 2 + record 9: toxicAt %d, want 11", tox(b))
	}
	sc.s.finishCleanup(sc.k)
	if tox(b) != 2+9 {
		t.Errorf("the cleanup step ended a permanent toxic grant: toxicAt %d", tox(b))
	}
	sc.s.removePerm(sc.k, b, kop.ZoneGraveyard, false)
	b2 := sc.slot(sc.perm(n, 0))
	if b2 != b || tox(b2) != 2 {
		t.Fatalf("a new permanent in the freed slot %d (was %d): toxicAt %d, want 2 (CR 400.7)", b2, b, tox(b2))
	}
	// layer 6: a record older than the loss grants nothing, a later one does
	sc.s.applyOverrideFl(sc.k, b2, permOv, n, sc.s.ovDurFl(kop.DurPermanent))
	sc.s.applyOverrideFl(sc.k, b2, permOv+1, n, sc.s.ovDurFl(kop.DurPermanent))
	if _, lost := sc.s.lossAt(sc.k, b2); !lost {
		t.Fatal("precondition: the loss-of-abilities record did not take hold")
	}
	if tox(b2) != 0 {
		t.Errorf("a loss of abilities with an older toxic grant: toxicAt %d, want 0", tox(b2))
	}
	sc.s.applyOverrideFl(sc.k, b2, permOv+2, n, sc.s.ovDurFl(kop.DurPermanent))
	if tox(b2) != 3 {
		t.Errorf("a toxic grant made after the loss: toxicAt %d, want 3 (CR 613.7)", tox(b2))
	}
	sc.s.toxicMark(sc.k, b2, 1)
	if sc.s.StallReason() != StallToxicGrant {
		t.Errorf("a mark while the abilities are lost: stall %s, want %s", StallName(sc.s.StallReason()), StallName(StallToxicGrant))
	}
}

// TestToxicGrantTableStall: a granted toxic alone does not make the table an
// infect table for the damage replacement-order ask (toxic is a trigger, CR
// 702.164b, no conversion to order), but still stalls beside an any-kind
// counter-added trigger and beside a printed infect.
func TestToxicGrantTableStall(t *testing.T) {
	grant := kop.CardDef{Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatContinuous, Toxic: 1}}}}
	ctr := kop.CardDef{Abilities: []kop.Ability{{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigCounterAddedOnce, N: kop.TrigNAnyCounter}}}}
	lore := kop.CardDef{Abilities: []kop.Ability{{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigCounterAddedOnce, N: 1}}}}
	run := func(repl uint32, cs ...kop.CardDef) genPrep {
		g := genPrep{anyRepl: repl}
		prepInfect(&kop.Table{Cards: cs}, &g)
		return g
	}
	if g := run(rbPrevNC, grant); !g.anyInfect || !g.anyInfectGrant || g.infectStall {
		t.Errorf("toxic grant + prevention: any %v grant %v stall %v, want true true false", g.anyInfect, g.anyInfectGrant, g.infectStall)
	}
	if g := run(0, grant, ctr); !g.infectStall {
		t.Error("toxic grant + any-kind counter-added trigger: no stall")
	}
	if g := run(0, grant, lore); g.infectStall {
		t.Error("toxic grant + a one-kind counter-added trigger (a Saga's lore): stall")
	}
	if g := run(0, kop.CardDef{Infect: true}, lore); !g.infectStall {
		t.Error("printed infect + a one-kind counter-added trigger: no stall")
	}
	if g := run(rbPrevNC, grant, kop.CardDef{Infect: true}); !g.infectStall {
		t.Error("toxic grant + printed infect + prevention: no stall")
	}
}
