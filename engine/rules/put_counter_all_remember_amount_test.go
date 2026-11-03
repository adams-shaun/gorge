package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// rememberAmountSage puts a +1/+1 counter on each creature its controller has
// and draws a card only if two or more counters were put: RememberAmount$ on
// PutCounterAll publishes the total that landed to Count$RememberedNumber.
const rememberAmountSage = "Name:Tally Sage\nManaCost:2 W\nTypes:Creature Advisor\nPT:1/4\n" +
	"A:AB$ PutCounterAll | Cost$ T | ValidCards$ Creature.YouCtrl | CounterType$ P1P1 | CounterNum$ 1 | RememberAmount$ True | SubAbility$ TallyDraw | SpellDescription$ x\n" +
	"SVar:TallyDraw:DB$ Draw | ConditionCheckSVar$ Tally | ConditionSVarCompare$ GE2 | SubAbility$ TallyClear\n" +
	"SVar:TallyClear:DB$ Cleanup | ClearRemembered$ True\n" +
	"SVar:Tally:Count$RememberedNumber\nOracle:x\n"

// TestPutCounterAllRememberAmountGatesAFollowUp: with one creature the sweep
// lands one counter and the draw is skipped; with a second creature it lands
// two and the draw runs.
func TestPutCounterAllRememberAmountGatesAFollowUp(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		bears      []string
		wantCounts int32
		wantDraw   bool
	}{
		{"one creature", nil, 1, false},
		{"two creatures", []string{"Sage Bear"}, 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seat0 := []*cards.Card{card(t, rememberAmountSage)}
			for _, b := range tc.bears {
				seat0 = append(seat0, card(t, counterBear(b)))
			}
			e, cfg := putCounterTable(t, 411, seat0, nil)
			sage := findAndMoveToBattlefield(t, e, 0, "Tally Sage")
			for _, b := range tc.bears {
				findAndMoveToBattlefield(t, e, 0, b)
			}
			e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
			e.priorityRound()
			handBefore := len(e.G.Zone(state.ZHand, 0))

			submitChoices(t, e, abilityOption(t, e, sage, 0).Index)
			passUntilStackEmpty(t, e, 30)

			if got := e.G.Obj(sage).Counter("P1P1"); got != tc.wantCounts {
				t.Fatalf("Tally Sage has %d counters, want %d", got, tc.wantCounts)
			}
			drew := len(e.G.Zone(state.ZHand, 0)) > handBefore
			if drew != tc.wantDraw {
				t.Fatalf("drew = %v, want %v (hand %d -> %d)", drew, tc.wantDraw, handBefore, len(e.G.Zone(state.ZHand, 0)))
			}
			replayCheck(t, e, cfg)
		})
	}
}
