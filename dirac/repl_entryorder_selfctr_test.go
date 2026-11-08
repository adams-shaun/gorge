package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The CR 616.1 stall for an entry beside a body that enters the entrant
// attacking (ExtEnterAttacking, Kira Nerys): gorge composes every updated
// body, the entrant's own enters-with-counters included, and asks because
// "other" commutes with nothing. entryComp leaves the entrant's own bodies
// out of cand, so the stall count must include them (selfLive), and a
// face-down entrant has no own bodies at all (CR 708.2).
type entSelfFix struct {
	sc                             *scen
	kira, ctrCarrier, gatedCarrier CardID
	plain, ctrCreature             CardID
}

func newEntSelfFix(t *testing.T) *entSelfFix {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	var pl CardID = 255
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "Plains" {
			pl = CardID(i)
		}
	}
	if pl == 255 {
		t.Fatal("no Plains in the fdn table")
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou, Zone: kop.ZoneBattlefield})
	f := kop.FilterID(len(tbl.Filters) - 1)
	base := CardID(len(tbl.Cards))
	hard := []kop.Instr{
		{Op: kop.OpEnterTapped},
		kop.ExtInstr(kop.ExtEnterAttacking),
	}
	ctr := []kop.Instr{{Op: kop.OpEnterWithCounters, Obj: kop.RefReplaced, A: kop.Const(1), C: int16(kop.CtrP1P1)}}
	tbl.Conds = append(tbl.Conds, kop.Cond{Kind: kop.CondAlways, Neg: true})
	never := kop.CondID(len(tbl.Conds) - 1)
	carrier := func(name string, prog []kop.Instr, cond kop.CondID) kop.CardDef {
		return kop.CardDef{Name: name, Types: kop.TArtifact, Abilities: []kop.Ability{
			{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplOthersEnterProgram, Filter: f, Cond: cond}, Prog: prog}}}
	}
	tbl.Cards = append(tbl.Cards,
		carrier("Kira Test", hard, 0),
		carrier("Counter Carrier", ctr, 0),
		carrier("Gated Carrier", ctr, never),
		kop.CardDef{Name: "Plain Test Beast", Types: kop.TCreature, Power: 2, Toughness: 2},
		kop.CardDef{Name: "Counter Test Beast", Types: kop.TCreature, Power: 2, Toughness: 2,
			Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplEntersWithCounters, A: 1, C: int16(kop.CtrP1P1)}}}},
	)
	sc := newScenTableLib(t, tbl, pl, Config{NoElide: true})
	if sc.k.gen == nil || !sc.k.gen.anyEntAttack {
		t.Fatal("the fixture table has no enters-attacking carrier")
	}
	sc.s.g.on = true
	return &entSelfFix{sc: sc, kira: base, ctrCarrier: base + 1, gatedCarrier: base + 2,
		plain: base + 3, ctrCreature: base + 4}
}

// enter puts card c from the hand onto the battlefield under p and returns
// the object.
func (e *entSelfFix) enter(c CardID, p uint8) Obj {
	o := e.sc.obj(c, p)
	e.sc.s.enter(e.sc.k, o, p, false, kop.ZoneHand)
	return o
}

// TestEntryOrderSelfCounterBodyBesideEnterAttacking: the entrant's own
// enters-with-counters body is a second live candidate beside Kira's
// enters-attacking body, so the entry stalls (gorge asks the order). A lone
// enters-attacking body does not stall.
func TestEntryOrderSelfCounterBodyBesideEnterAttacking(t *testing.T) {
	e := newEntSelfFix(t)
	sc := e.sc
	kira := sc.perm(e.kira, 0)
	if !sc.onBF(kira) || sc.k.cards[e.ctrCreature].enterCtr == 0 {
		t.Fatal("the entrant has no own enters-with-counters body")
	}
	o := e.enter(e.ctrCreature, 0)
	if !sc.onBF(o) {
		t.Fatalf("the entrant is not on the battlefield (zone %d)", sc.s.objs[o].zone)
	}
	if sc.s.g.stalled != StallEntryOrder {
		t.Fatalf("stalled %d, want StallEntryOrder: gorge asks the order of Kira's body and the entrant's own counters", sc.s.g.stalled)
	}
}

// TestEntryOrderLoneEnterAttackingDoesNotStall: only one body applies, so
// gorge poses no order and the kernel must not stall.
func TestEntryOrderLoneEnterAttackingDoesNotStall(t *testing.T) {
	e := newEntSelfFix(t)
	sc := e.sc
	sc.perm(e.kira, 0)
	o := e.enter(e.plain, 0)
	if !sc.onBF(o) {
		t.Fatalf("the entrant is not on the battlefield (zone %d)", sc.s.objs[o].zone)
	}
	if sc.s.g.stalled != 0 {
		t.Fatalf("stalled %d, want none beside the lone enters-attacking body", sc.s.g.stalled)
	}
	if sc.s.perms[sc.slot(o)].flags&pfTapped == 0 {
		t.Fatal("Kira's body did not apply (the entrant should be tapped)")
	}
}

// TestEntryOrderGatedOtherBodyBesideEnterAttacking: a second body whose
// IsPresent$ gate fails is not a candidate (runOthersEnterPrograms skips it,
// and gorge's replacementMatches does too), so a lone live Kira body asks
// nothing and the entry must not stall.
func TestEntryOrderGatedOtherBodyBesideEnterAttacking(t *testing.T) {
	e := newEntSelfFix(t)
	sc := e.sc
	sc.perm(e.kira, 0)
	sc.perm(e.gatedCarrier, 0)
	o := e.enter(e.plain, 0)
	if sc.s.g.stalled != 0 {
		t.Fatalf("stalled %d, want none: the gated body does not apply", sc.s.g.stalled)
	}
	if !sc.onBF(o) || sc.s.perms[sc.slot(o)].flags&pfTapped == 0 {
		t.Fatal("Kira's body did not apply (the entrant should be on the battlefield tapped)")
	}
}

// TestEntryOrderFaceDownEntrantStalls: a face-down entrant runs no own enter
// body, but Kira's body and a second permanent's counter body both apply to
// it (runOthersEnterPrograms reads it as a 2/2 creature), so gorge asks the
// order and the kernel stalls.
func TestEntryOrderFaceDownEntrantStalls(t *testing.T) {
	e := newEntSelfFix(t)
	sc := e.sc
	sc.perm(e.kira, 0)
	sc.perm(e.ctrCarrier, 0)
	o := sc.obj(e.plain, 0)
	sl := sc.s.enterFaceDown(sc.k, o, 0, kop.MorphMegamorph, kop.ZoneLibrary)
	if !sc.s.faceDown(sc.k, sl) {
		t.Fatal("the entrant is not face down")
	}
	if sc.s.g.stalled != StallEntryOrder {
		t.Fatalf("stalled %d, want StallEntryOrder: two other bodies apply to the face-down entrant", sc.s.g.stalled)
	}
}

// TestEntryOrderFaceDownOwnBodyDoesNotCompete: a face-down permanent has no
// abilities (CR 708.2), so its own enters-with-counters body is not a
// candidate; beside the lone Kira body the entry must not stall.
func TestEntryOrderFaceDownOwnBodyDoesNotCompete(t *testing.T) {
	e := newEntSelfFix(t)
	sc := e.sc
	sc.perm(e.kira, 0)
	o := sc.obj(e.ctrCreature, 0)
	sl := sc.s.enterFaceDown(sc.k, o, 0, kop.MorphMegamorph, kop.ZoneLibrary)
	if !sc.s.faceDown(sc.k, sl) {
		t.Fatal("the entrant is not face down")
	}
	if sc.s.g.stalled != 0 {
		t.Fatalf("stalled %d, want none: a face-down permanent has no own enter body", sc.s.g.stalled)
	}
}
