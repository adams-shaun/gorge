package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// Gabriel Lorca, Mirror Menace shape: a FacesDilemma trigger that works from the
// graveyard (TriggerZones$) with an optional {B} ChangeZone body (Cost$ B,
// Origin$ Graveyard, Destination$ Hand).
const lorcaCard = "Name:Lorca Probe\nManaCost:4 B\nTypes:Legendary Creature Human\nPT:5/4\n" +
	"T:Mode$ FacesDilemma | ValidPlayer$ You | TriggerZones$ Graveyard | Execute$ BackToHand | TriggerDescription$ x\n" +
	"SVar:BackToHand:AB$ ChangeZone | Defined$ Self | Destination$ Hand | Origin$ Graveyard | Cost$ B | StackDescription$ x\nOracle:x\n"

// lorcaInstantCharm is dilemmaCharm at instant speed, so the opponent can face
// a dilemma while seat 0 holds the graveyard card.
const lorcaInstantCharm = "Name:Quick Roads\nManaCost:U\nTypes:Instant\n" +
	"A:SP$ Charm | CharmNum$ 2 | MinCharmNum$ 0 | Choices$ RoadA,RoadB\n" +
	"SVar:RoadA:DB$ GainLife | Defined$ You | LifeAmount$ 1 | SpellDescription$ Gain one.\n" +
	"SVar:RoadB:DB$ GainLife | Defined$ You | LifeAmount$ 2 | SpellDescription$ Gain two.\nOracle:x\n"

// lorcaTriggers counts the stack objects that are the card's trigger.
func lorcaTriggers(e *Engine, lorca state.ObjID) int {
	n := 0
	for _, id := range e.G.Stack {
		if o := e.G.Obj(id); o != nil && o.Source == lorca {
			n++
		}
	}
	return n
}

// lorcaResolve passes priority until only floor stack objects remain, answering
// the trigger's pay question with pay (option 0) or a decline (option 1) and
// counting the questions asked.
func lorcaResolve(t *testing.T, e *Engine, floor int, pay bool) (asks int) {
	t.Helper()
	for i := 0; i < 20 && len(e.G.Stack) > floor; i++ {
		d := e.Pending()
		if d.Kind == decision.KPriority {
			passPriority(t, e)
			continue
		}
		asks++
		want := "trigger_cost_decline"
		if pay {
			want = "trigger_cost_pay"
		}
		idx := -1
		for _, o := range d.Options {
			if o.Kind == want {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("ask %q has no %s option: %+v", d.Prompt, want, d.Options)
		}
		submitChoices(t, e, idx)
	}
	if len(e.G.Stack) > floor {
		t.Fatalf("stack %v did not shrink to %d", e.G.Stack, floor)
	}
	return asks
}

// TestLorcaGorge: a FacesDilemma trigger works from the graveyard (TriggerZones$
// says where it functions), fires once per dilemma however many modes were chosen,
// only for its controller's dilemmas, and the optional {B} lets the card return
// to hand; declining leaves it.
func TestLorcaGorge(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		modes    []int
		pay      bool
		wantZone state.Zone
		wantMana int32
	}{
		{"two modes, paid", []int{0, 1}, true, state.ZHand, 0},
		{"one mode, declined", []int{1}, false, state.ZGraveyard, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e, _, spell := newFixtureDeck(t, 961, dilemmaCharm)
			lorca := putGraveyard(t, e, 0, lorcaCard)
			addMana(t, e, 0, "UB")
			e.Advance()
			castDilemmaCharm(t, e, spell, tc.modes...)
			if got := lorcaTriggers(e, lorca); got != 1 {
				t.Fatalf("graveyard triggers on the stack = %d, want 1 for %d modes (stack %v)", got, len(tc.modes), e.G.Stack)
			}
			if z := e.G.Obj(lorca).Zone; z != state.ZGraveyard {
				t.Fatalf("card is in %v before its trigger resolves, want the graveyard", z)
			}
			if asks := lorcaResolve(t, e, 1, tc.pay); asks != 1 {
				t.Fatalf("%d pay questions, want 1", asks)
			}
			if z := e.G.Obj(lorca).Zone; z != tc.wantZone {
				t.Fatalf("card is in %v, want %v", z, tc.wantZone)
			}
			if got := e.G.Players[0].Pool.Total(); got != tc.wantMana {
				t.Fatalf("mana left %d, want %d", got, tc.wantMana)
			}
		})
	}
}

// TestLorcaGorgeTwoDilemmas: two dilemmas in one turn are two triggers; the second
// finds the card already in hand and does nothing.
func TestLorcaGorgeTwoDilemmas(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 962, dilemmaCharm, lorcaInstantCharm)
	lorca := putGraveyard(t, e, 0, lorcaCard)
	second := putInHand(t, e, 0, card(t, lorcaInstantCharm))
	addMana(t, e, 0, "UUB")
	e.Advance()
	castDilemmaCharm(t, e, spell, 0)
	submitChoices(t, e, miscCastOption(t, e, second))
	submitChoices(t, e, 1)
	if got := lorcaTriggers(e, lorca); got != 2 {
		t.Fatalf("triggers on the stack = %d, want 2 (one per dilemma): %v", got, e.G.Stack)
	}
	if got := e.G.Players[0].DilemmasFacedThisTurn; got != 2 {
		t.Fatalf("dilemmas faced %d, want 2", got)
	}
}

// TestLorcaGorgeOpponent: the opponent's dilemma is not "you face".
func TestLorcaGorgeOpponent(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 963, lorcaInstantCharm)
	lorca := putGraveyard(t, e, 0, lorcaCard)
	opp := putInHand(t, e, 1, card(t, lorcaInstantCharm))
	addMana(t, e, 1, "U")
	e.Advance()
	passPriority(t, e) // seat 0 passes; seat 1 has priority
	if d := e.Pending(); d == nil || d.Player != 1 {
		t.Fatalf("pending %+v, want seat 1's priority", d)
	}
	castDilemmaCharm(t, e, opp, 0)
	if got := e.G.Players[1].DilemmasFacedThisTurn; got != 1 {
		t.Fatalf("seat 1 faced %d dilemmas, want 1", got)
	}
	if got := lorcaTriggers(e, lorca); got != 0 {
		t.Fatalf("opponent's dilemma queued %d graveyard triggers, want 0", got)
	}
}
