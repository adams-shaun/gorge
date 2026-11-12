package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestAddCounterReposeOffersOnlyUnappliedReplacements: three replacements
// compete for one +1/+1 placement -- Hardened Scales and Conclave Mentor
// (each "that many plus one") and Doubling Season (twice that many). After
// the first answer applies one, CR 616.1e asks again among the ones still
// applicable; a replacement modifies an event at most once (CR 616.1), so
// the re-pose must not offer the one just applied. Offering it let an
// answer pick it again and again, the same two events forever (a Commander
// pod livelock with these three on one board).
func TestAddCounterReposeOffersOnlyUnappliedReplacements(t *testing.T) {
	t.Parallel()
	scales := corpusAlternativeCard(t, "Hardened Scales")
	mentor := corpusAlternativeCard(t, "Conclave Mentor")
	season := corpusAlternativeCard(t, "Doubling Season")
	bear := card(t, "Name:Counter Bearer\nManaCost:0\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	spell := card(t, "Name:Grow One\nManaCost:0\nTypes:Sorcery\n"+
		"A:SP$ PutCounter | ValidTgts$ Creature | CounterType$ P1P1 | CounterNum$ 1 | SpellDescription$ x\nOracle:x\n")
	e, cfg := tokenReplGame(t, 9203, scales, mentor, season, bear, spell)
	withStagedModifiers(t, e, scales, mentor)
	moveSeededCard(t, e, 0, season, state.ZBattlefield)
	bearID := moveSeededCard(t, e, 0, bear, state.ZBattlefield)
	moveSeededCard(t, e, 0, spell, state.ZHand)
	addMana(t, e, 0, "")
	castSpellOption(t, e, "Grow One")
	var offered []int
	for i := 0; i < 60; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision while resolving")
		}
		pick := 0
		switch d.Kind {
		case decision.KPriority:
			if len(e.G.Stack) == 0 {
				i = 60
				continue
			}
			passPriorityOnce(t, e)
			continue
		case decision.KTarget:
			pick = indexOfObjOption(d, bearID)
		case decision.KReplacement:
			if !strings.Contains(d.Prompt, "counters") {
				t.Fatalf("unexpected replacement ask %q", d.Prompt)
			}
			offered = append(offered, len(d.Options))
			if len(offered) > 3 {
				t.Fatalf("the counter order was asked %d times; option counts %v", len(offered), offered)
			}
			// Always the option naming Conclave Mentor while it is offered:
			// the answer the livelocked seat kept giving.
			for j, o := range d.Options {
				if strings.Contains(o.Label, "Conclave Mentor") {
					pick = j
				}
			}
		default:
			t.Fatalf("unexpected decision %+v", d)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{pick}}); err != nil {
			t.Fatal(err)
		}
	}
	if len(offered) != 2 || offered[0] != 3 || offered[1] != 2 {
		t.Fatalf("counter order asks offered %v options, want [3 2] (the re-pose drops the applied one)", offered)
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("the sorcery never left the stack (depth %d)", len(e.G.Stack))
	}
	replayCheck(t, e, cfg)
}
