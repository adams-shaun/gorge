package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEDHKeywordsRampagePumpsPerExtraBlocker plays Marhault Elsdragon's
// printed Rampage 1 (CR 702.23) through a real combat: one blocker is the
// baseline (no pump), each blocker beyond the first pumps +1/+1 until end of
// turn when the becomes-blocked trigger resolves.
func TestEDHKeywordsRampagePumpsPerExtraBlocker(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	marhault, ok := reg.Lookup("Marhault Elsdragon")
	if !ok {
		t.Fatal("Marhault Elsdragon not in the corpus")
	}
	if got, ok := marhault.Faces[0].KeywordParam("Rampage"); !ok || got != "1" {
		t.Fatalf("Marhault Elsdragon keyword param = %q (%v), want Rampage 1", got, ok)
	}
	const bear = "Name:Runeclaw Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

	// Two blockers: 4/6 becomes 5/6 when the trigger resolves.
	e := combatEngine(t)
	attacker := onBoardCard(t, e, 0, marhault)
	e.G.Obj(attacker).SummonSick = false
	b1 := onBoard(t, e, 1, bear)
	b2 := onBoard(t, e, 1, bear)
	e.askAttackers()
	submitAttackersOnly(t, e, attacker)
	drainCombatPriority(t, e)
	if d := e.Pending(); d == nil || d.Kind != decision.KBlockers {
		t.Fatalf("expected a blockers decision, got %+v", d)
	}
	submitBlockersOnly(t, e, b1, b2)
	if n := len(e.G.Stack); n != 1 {
		t.Fatalf("stack = %d, want one Rampage trigger", n)
	}
	if e.Power(attacker) != 4 || e.Toughness(attacker) != 6 {
		t.Fatalf("before resolution = %d/%d, want 4/6", e.Power(attacker), e.Toughness(attacker))
	}
	e.resolveTop()
	if e.Power(attacker) != 5 || e.Toughness(attacker) != 7 {
		t.Fatalf("after resolution = %d/%d, want 5/7 (one blocker beyond the first pumps +1/+1)", e.Power(attacker), e.Toughness(attacker))
	}

	// One blocker: no pump at all.
	e2 := combatEngine(t)
	alone := onBoardCard(t, e2, 0, marhault)
	e2.G.Obj(alone).SummonSick = false
	blk := onBoard(t, e2, 1, bear)
	e2.askAttackers()
	submitAttackersOnly(t, e2, alone)
	drainCombatPriority(t, e2)
	if d := e2.Pending(); d == nil || d.Kind != decision.KBlockers {
		t.Fatalf("expected a blockers decision, got %+v", d)
	}
	submitBlockersOnly(t, e2, blk)
	if n := len(e2.G.Stack); n != 1 {
		t.Fatalf("single-blocker stack = %d, want one Rampage trigger", n)
	}
	e2.resolveTop()
	if e2.Power(alone) != 4 || e2.Toughness(alone) != 6 {
		t.Fatalf("single blocker after resolution = %d/%d, want 4/6", e2.Power(alone), e2.Toughness(alone))
	}
}

// TestEDHKeywordsTransfigureTutorsSameManaValue plays Fleshwrither's printed
// Transfigure 1BB (CR 702.71) as a real activation: the cost sacs the
// creature and the search offers exactly the mana-value-four creatures from
// the library, the chosen one entering the battlefield.
func TestEDHKeywordsTransfigureTutorsSameManaValue(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	fleshwrither, ok := reg.Lookup("Fleshwrither")
	if !ok {
		t.Fatal("Fleshwrither not in the corpus")
	}
	if got, ok := fleshwrither.Faces[0].KeywordParam("Transfigure"); !ok || got != "1 B B" {
		t.Fatalf("Fleshwrither keyword param = %q (%v), want Transfigure 1 B B", got, ok)
	}
	e := handEngine(t, fleshwrither)
	fw := e.G.Zone(state.ZHand, 0)[0]
	placeOnBattlefield(t, e, fw)
	// handEngine's library is all Mountains (mana value zero, not creatures),
	// so the search would correctly find nothing unmodified. Seed a
	// mana-value-four creature for it to find.
	mv4 := card(t, "Name:Mannequin Giant\nManaCost:2 B B\nTypes:Creature Giant\nPT:4/4\nOracle:x\n")
	sObj := e.G.AddObject(mv4, 0)
	e.G.SetZone(state.ZLibrary, 0, append([]state.ObjID{sObj.ID}, e.G.Zone(state.ZLibrary, 0)...))
	wanted := sObj.ID
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MB] = 1, 2
	var opt decision.Option
	for _, o := range e.legalActions(0) {
		if o.Kind == "ability" && o.Obj == fw {
			opt = o
			break
		}
	}
	if opt.Kind != "ability" {
		t.Fatal("Fleshwrither Transfigure activation not offered")
	}
	e.beginActivation(0, opt)
	if e.G.Obj(fw).Zone != state.ZGraveyard {
		t.Fatalf("Transfigure sacrifice zone=%s", e.G.Obj(fw).Zone)
	}
	e.pending = nil // the kernel probe serves asks only with no decision posed
	e.resolveTop()
	d := e.Pending()
	choice := -1
	if d != nil {
		for i, o := range d.Options {
			if o.Obj == wanted {
				choice = i
				break
			}
		}
	}
	if choice < 0 {
		t.Fatalf("Transfigure did not offer the mana-value-four library card: %+v", d)
	}
	submitChoices(t, e, choice)
	if e.G.Obj(wanted).Zone != state.ZBattlefield {
		t.Fatalf("Transfigure target zone=%s, want battlefield", e.G.Obj(wanted).Zone)
	}
}
