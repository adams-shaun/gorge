package rules

// Mana Leak: the UnlessCost$ pay/decline is posed to the CONTROLLER OF THE COUNTERED
// SPELL (not the counterspell's caster), nothing is countered before the
// answer, a paid tax lets the spell resolve, and a decline counters it.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// manaLeakAsk puts seat 1's creature spell on the stack, has seat 0 cast
// the real Mana Leak at it, and returns the unless_pay ask.
func manaLeakAsk(t *testing.T) (*Engine, state.ObjID, *decision.Decision) {
	t.Helper()
	e := kernelEngine(t, 2)
	bear := putCardIn(t, e, 1, cardOfSrc(t, "Name:Leak Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), state.ZStack, false)
	leak := putCardIn(t, e, 0, corpusFixtureCard(t, "Mana Leak"), state.ZHand, false)
	fundMana(e, 1)
	d := castWithMana(t, e, 0, leak)
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("Mana Leak cast = %+v, want its target ask", d)
	}
	d = requireResumeAsk(t, answerDecision(t, e, d, objOptionIndex(t, d, bear)), "unless_pay")
	return e, bear, d
}

func TestManaLeakAsksTheCounteredSpellsController(t *testing.T) {
	t.Parallel()
	e, bear, d := manaLeakAsk(t)
	if d.Kind != decision.KModes || d.Min != 1 || d.Max != 1 {
		t.Fatalf("decision = %+v, want a Min==Max==1 KModes", d)
	}
	if d.Player != 1 {
		t.Fatalf("payer = seat %d, want the countered spell's controller (seat 1), not the caster", d.Player)
	}
	if !strings.Contains(d.Prompt, "Pay 3") {
		t.Fatalf("prompt = %q, want it to name the {3} tax", d.Prompt)
	}
	if len(d.Options) != 2 || d.Options[0].Label != "Pay 3 — don't counter" || d.Options[1].Label != "Don't pay" {
		t.Fatalf("options = %+v, want the pay/decline pair", d.Options)
	}
	if z := e.G.Obj(bear).Zone; z != state.ZStack {
		t.Fatalf("spell left the stack before the answer: %s", z)
	}
}

func TestManaLeakPayDoesNotCounterAndDeclineCounters(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		pay  bool
		want state.Zone
	}{{"pay", true, state.ZBattlefield}, {"decline", false, state.ZGraveyard}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e, bear, d := manaLeakAsk(t)
			idx := 1
			if tc.pay {
				idx = 0
			}
			d = answerDecision(t, e, d, idx)
			for i := 0; d != nil && i < 20; i++ {
				switch d.ResumeKind {
				case unlessManaKind:
					pick := len(d.Options) - 1
					for _, o := range d.Options {
						if o.Kind == "activate" {
							pick = o.Index
							break
						}
					}
					d = answerDecision(t, e, d, pick)
				default:
					t.Fatalf("unexpected decision %+v", d)
				}
			}
			if z := e.G.Obj(bear).Zone; z != tc.want {
				t.Fatalf("%s: Leak Bear is on %s, want %s", tc.name, z, tc.want)
			}
		})
	}
}
