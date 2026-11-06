package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// The compose memo (cost_compose_memo.go) serves a pure-read pricing asked
// twice at the same at-rest state: the per-candidate composition the offer
// gate and the potential walk re-ask for the same question answers from the
// table the first ask stored, and any real state move clears it.
func TestCostComposeMemoServesTheRepeatCompose(t *testing.T) {
	t.Parallel()
	spellCard := card(t, "Name:Test Bolt\nManaCost:2\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Creature | NumDmg$ 1\nOracle:x\n")
	e := handEngine(t, spellCard)
	source := func(name string) state.ObjID {
		return battlePerm(t, e, 0, "Name:"+name+"\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nS:Mode$ ReduceCost | ValidTarget$ Card.Self | Activator$ You | Type$ Spell | Amount$ 1 | Description$ Discount a spell targeting CARDNAME.\nOracle:x\n")
	}
	a, b := source("First Bear"), source("Second Bear")
	spell := e.G.Zone(state.ZHand, 0)[0]
	base := e.parseCost("2")
	if base.Generic != 2 || e.G.Obj(spell).Zone != state.ZHand ||
		e.G.Obj(a).Zone != state.ZBattlefield || e.G.Obj(b).Zone != state.ZBattlefield {
		t.Fatalf("precondition: base=%+v zones=%s,%s,%s", base, e.G.Obj(spell).Zone, e.G.Obj(a).Zone, e.G.Obj(b).Zone)
	}
	statics := e.collectCostStatics()
	if len(statics.reduce) != 2 {
		t.Fatalf("precondition: expected both reducers, got %d", len(statics.reduce))
	}
	// The offer gate's base question (no targets, the unannounced pass) is
	// stored by the first ask and served by the second.
	before := e.costComposeServed
	first := e.costModifiersWithTargetsUsing(statics, 0, spell, spellScope(""), nil, false)
	repeat := e.costModifiersWithTargetsUsing(statics, 0, spell, spellScope(""), nil, false)
	if len(first.Reduces) != 0 || len(repeat.Reduces) != 0 {
		t.Fatalf("precondition: the base pass cannot apply a target-conditional reduction, got %d and %d",
			len(first.Reduces), len(repeat.Reduces))
	}
	if e.costComposeServed != before+1 {
		t.Fatalf("the repeated base compose was recomputed, not served (served %d, want %d)", e.costComposeServed, before+1)
	}
	// The per-candidate question is its own entry: the potential pass earns
	// exactly one reduction per chosen bear, the same on the served repeat.
	pot := e.costModifiersWithTargetsUsing(statics, 0, spell, spellScope(""), []state.Target{{Obj: a}}, true)
	if got := pot.Apply(base).Generic; got != 1 || len(pot.Reduces) != 1 {
		t.Fatalf("precondition: target %d earns exactly ONE reduction, cost=%d reductions=%d", a, got, len(pot.Reduces))
	}
	// A served composition is the entry's own exact-cap backing: a caller
	// appending to it in place (morphTurnUpMods' printed rider) must
	// reallocate, never write through to the entry -- the next serve of the
	// same question still answers the stored composition.
	served := e.costModifiersWithTargetsUsing(statics, 0, spell, spellScope(""), []state.Target{{Obj: a}}, true)
	served.Reduces = append(served.Reduces, costMod{Generic: 7})
	afterAppend := e.costModifiersWithTargetsUsing(statics, 0, spell, spellScope(""), []state.Target{{Obj: a}}, true)
	if got := afterAppend.Apply(base).Generic; got != 1 || len(afterAppend.Reduces) != 1 {
		t.Fatalf("a caller's append corrupted the served entry (cost=%d reductions=%+v)", got, afterAppend.Reduces)
	}
	potAgain := e.costModifiersWithTargetsUsing(statics, 0, spell, spellScope(""), []state.Target{{Obj: a}}, true)
	if got := potAgain.Apply(base).Generic; got != 1 || len(potAgain.Reduces) != 1 || e.costComposeServed != before+4 {
		t.Fatalf("the repeated candidate compose was recomputed, not served (served %d, want %d)", e.costComposeServed, before+4)
	}
	// A different assignment is a different question: it stores its own
	// answer and is served on its own repeat.
	potB := e.costModifiersWithTargetsUsing(statics, 0, spell, spellScope(""), []state.Target{{Obj: b}}, true)
	if got := potB.Apply(base).Generic; got != 1 {
		t.Fatalf("precondition: target %d earns one reduction, cost=%d", b, got)
	}
	if e.costComposeServed != before+4 {
		t.Fatalf("a new question served a neighbour's answer (served %d, want %d)", e.costComposeServed, before+4)
	}
	// A real state move clears the table: with a third reducer on the
	// battlefield the same questions are recomputed (the answer is unchanged
	// -- a Card.Self discount still names only its own bear), and the
	// re-asked question is served only on the next repeat.
	third := battlePerm(t, e, 0, "Name:Third Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nS:Mode$ ReduceCost | ValidTarget$ Card.Self | Activator$ You | Type$ Spell | Amount$ 1 | Description$ Discount a spell targeting CARDNAME.\nOracle:x\n")
	if e.G.Obj(third).Zone != state.ZBattlefield {
		t.Fatalf("precondition: the third bear never landed, zone %s", e.G.Obj(third).Zone)
	}
	moved := e.costComposeServed
	statics = e.collectCostStatics()
	if len(statics.reduce) != 3 {
		t.Fatalf("precondition: expected three reducers after the move, got %d", len(statics.reduce))
	}
	after := e.costModifiersWithTargetsUsing(statics, 0, spell, spellScope(""), []state.Target{{Obj: a}}, true)
	if got := after.Apply(base).Generic; got != 1 || len(after.Reduces) != 1 {
		t.Fatalf("post-move answer changed: cost=%d reductions=%d", got, len(after.Reduces))
	}
	if e.costComposeServed != moved {
		t.Fatalf("a moved state served a stale answer (served %d, want %d)", e.costComposeServed, moved)
	}
	after2 := e.costModifiersWithTargetsUsing(statics, 0, spell, spellScope(""), []state.Target{{Obj: a}}, true)
	if got := after2.Apply(base).Generic; got != 1 || e.costComposeServed != moved+1 {
		t.Fatalf("the post-move repeat was recomputed, not served (served %d, want %d)", e.costComposeServed, moved+1)
	}
}

// The potential walk's per-assignment enumeration is served on the repeat:
// one walk of a posed decision stores each assignment's composition, and a
// second walk at the same at-rest state (a DecisionAsk writes nothing) with
// the same shared statics collection is served from the table.
func TestCostComposeMemoBridgesTheWalks(t *testing.T) {
	t.Parallel()
	spellCard := card(t, "Name:Test Bolt\nManaCost:2\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Creature | NumDmg$ 1\nOracle:x\n")
	e := handEngine(t, spellCard)
	source := func(name string) state.ObjID {
		return battlePerm(t, e, 0, "Name:"+name+"\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nS:Mode$ ReduceCost | ValidTarget$ Card.Self | Activator$ You | Type$ Spell | Amount$ 1 | Description$ Discount a spell targeting CARDNAME.\nOracle:x\n")
	}
	a, b := source("First Bear"), source("Second Bear")
	spell := e.G.Zone(state.ZHand, 0)[0]
	base := e.parseCost("2")
	cands := e.costPotentialTargets(0, spell, spellScope(""))
	if len(cands) != 2 || cands[0].Obj != a || cands[1].Obj != b {
		t.Fatalf("precondition: candidates=%+v", cands)
	}
	// The first walk stores the base question and each assignment's answer
	// (the walk is refused: no target makes {2} free at an empty pool), with
	// the collection a walk shares across its whole enumeration.
	statics := e.collectCostStatics()
	if e.offerCastableUsing(statics, 0, spell, &base, spellScope(""), false, nil) {
		t.Fatal("precondition: the offer gate admitted a cast it must refuse")
	}
	before := e.costComposeServed
	_, ok := e.potentialCostModsUsing(statics, 0, spell, spellScope(""), cands, 0, func(m costMods) bool {
		return m.Apply(base).Generic == 0
	})
	if ok {
		t.Fatal("candidate composition admitted a free price no target can earn")
	}
	first := e.costComposeServed
	// The second walk of the same at-rest state, same collection, is served:
	// every assignment the first walk stored answers from the table.
	_, ok = e.potentialCostModsUsing(statics, 0, spell, spellScope(""), cands, 0, func(m costMods) bool {
		return m.Apply(base).Generic == 0
	})
	if ok {
		t.Fatal("candidate composition admitted a free price no target can earn")
	}
	if e.costComposeServed < first+uint64(len(cands)) {
		t.Fatalf("the repeated walk recomputed the assignments (served %d, want at least %d)",
			e.costComposeServed, first+uint64(len(cands)))
	}
	if first == before {
		t.Fatalf("the first walk recomputed even the shared questions the offer probe stored (served %d)", first)
	}
}

// Verify mode recomputes every serve: a corrupted entry must panic, not
// serve.
func TestCostComposeMemoVerifyCatchesCorruption(t *testing.T) {
	t.Parallel()
	spellCard := card(t, "Name:Test Bolt\nManaCost:2\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Creature | NumDmg$ 1\nOracle:x\n")
	e := handEngine(t, spellCard)
	battlePerm(t, e, 0, "Name:First Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nS:Mode$ ReduceCost | ValidTarget$ Card.Self | Activator$ You | Type$ Spell | Amount$ 1 | Description$ Discount a spell targeting CARDNAME.\nOracle:x\n")
	spell := e.G.Zone(state.ZHand, 0)[0]
	statics := e.collectCostStatics()
	first := e.costModifiersWithTargetsUsing(statics, 0, spell, spellScope(""), []state.Target{{Obj: e.G.Zone(state.ZBattlefield, 0)[0]}}, true)
	if len(first.Reduces) != 1 || first.Reduces[0].Generic != 1 {
		t.Fatalf("precondition: one reduction of 1, got %+v", first.Reduces)
	}
	// Find the stored entry for the potential question and corrupt its
	// reduction amount.
	cand := e.G.Zone(state.ZBattlefield, 0)[0]
	var corrupted *costComposeEnt
	for ci, chunk := range e.costCompose.chunks {
		n := entChunk
		if ci == e.costCompose.nchunk-1 {
			n = e.costCompose.free
		}
		for k := range chunk[:n] {
			ent := &chunk[k]
			if ent.id == spell && ent.pot && len(ent.tkey) == 1 && ent.tkey[0].Obj == cand {
				corrupted = ent
			}
		}
	}
	if corrupted == nil {
		t.Fatal("precondition: the potential question was never stored")
	}
	corrupted.mods.reduces[0].Generic += 5
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("verify mode served a corrupted entry instead of panicking")
			}
			msg, ok := r.(string)
			if !ok || !strings.Contains(msg, "cost-modifier compose memo is stale") {
				t.Fatalf("unexpected panic %v", r)
			}
		}()
		e.costModifiersWithTargetsUsing(statics, 0, spell, spellScope(""), []state.Target{{Obj: cand}}, true)
	}()
}
