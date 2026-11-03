package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestRestrictedManaFundsAFilterFeeForAnUnadmittedSpell: Hydraulic Helper's
// {U} "can't be spent to cast a nonartifact spell", but a mana ability's
// activation cost is no spell (CR 106.6, 605.3), so it pays Surveillance
// Room's {1} and the Room's mana of any colour casts the {U} nonartifact
// creature. The potential walk prices the Room's fee from the other sources'
// mana, restricted mana the activation may spend included.
func TestRestrictedManaFundsAFilterFeeForAnUnadmittedSpell(t *testing.T) {
	reg := searchTestRegistry(t)
	hh := searchCorpusCard(t, reg, "Hydraulic Helper")
	room := searchCorpusCard(t, reg, "Surveillance Room")
	e, _ := dotlessEngine(t, reg, hh, room, card(t, dotlessCreatureU))
	searchMoveByName(t, e, "Hydraulic Helper", state.ZBattlefield)
	searchMoveByName(t, e, "Surveillance Room", state.ZBattlefield)
	whelp := searchMoveByName(t, e, "Blue Whelp", state.ZHand)
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: e.G.Turn + 1})
	e.pending = nil
	e.priorityRound()
	for i := 0; i < 30 && (len(e.G.Stack) > 0 || e.Pending().Kind != decision.KPriority); i++ { // the Room's surveil
		if d := e.Pending(); d.Kind == decision.KPriority {
			for _, o := range d.Options {
				if o.Kind == "pass" {
					submitChoices(t, e, o.Index)
					break
				}
			}
		} else {
			submitChoices(t, e)
		}
	}
	e.pending = nil
	e.priorityRound()
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Kind == "cast" && pp.Action.Obj == whelp {
			if pp.Reason != "" {
				t.Fatalf("the cast is priced %q/%q, want a payable play", pp.Reason, pp.Detail)
			}
			return
		}
	}
	t.Fatal("the nonartifact cast is not a potential play: the Room's fee was priced without the Helper's restricted mana")
}
