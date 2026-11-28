package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestSquanderedResourcesPrefixFundsACast pins the potential walk's pricing of
// a ManaReflected ability whose Valid$ is Defined.Sacrificed (Squandered
// Resources). Two tapped Forests pay nothing themselves; the only line that
// pays {1}{G} is Squandered Resources activated twice, sacrificing both lands
// and adding one of each land's producible symbols. The engine's offer gate
// already lists the activation (mana_reflected_sac.go); the potential walk and
// its prefix search must price the cast after those two activations, or the
// play is invisible to the seat's planner while the board can pay it.
func TestSquanderedResourcesPrefixFundsACast(t *testing.T) {
	t.Parallel()
	bears := card(t, "Name:Probe Bears\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e := handEngine(t, bears)
	bearsID := e.G.Zone(state.ZHand, 0)[0]
	sq := corpusAlternativeCard(t, "Squandered Resources")
	sqid := e.G.AddObject(sq, 0)
	sqid.Zone = state.ZBattlefield
	bf := []state.ObjID{sqid.ID}
	for i := 0; i < 2; i++ {
		l := e.G.AddObject(card(t, squanderedForest), 0)
		l.Zone = state.ZBattlefield
		l.Tapped = true
		bf = append(bf, l.ID)
	}
	e.G.SetZone(state.ZBattlefield, 0, bf)
	// The precondition the assertion depends on: no untapped land can tap for
	// the cost, so only Squandered Resources' sacrifice line can pay it.
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Tapped {
			if hasActivateOptionFor(e.legalActions(0), id) {
				t.Fatalf("fixture: tapped land %d is still offered for mana", id)
			}
		}
	}
	if got := e.G.Players[0].Pool[state.MG] + e.G.Players[0].Pool[state.MC]; got != 0 {
		t.Fatalf("fixture: pool already holds %d colourless/green mana", got)
	}
	e.priorityRound()
	if !hasActivateOption(e, sqid.ID) {
		t.Fatalf("Squandered Resources not offered with two tapped lands to sacrifice: %+v", e.Pending().Options)
	}
	hyp := e.PotentialMana(0)
	potential := e.legalActionsPriced(0, &hyp)
	if !hasActivateOptionFor(potential, sqid.ID) {
		t.Fatalf("potential walk does not list Squandered Resources' activation: %+v", potential)
	}
	found := false
	for _, o := range potential {
		if o.Kind == "cast" && o.Obj == bearsID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("potential walk does not list Probe Bears though Squandered Resources' two sacrifices pay {1}{G}: %+v", potential)
	}
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Kind != "cast" || pp.Action.Obj != bearsID {
			continue
		}
		if len(pp.Script) == 0 {
			t.Fatalf("Probe Bears plan carries no script: reason=%q detail=%q", pp.Reason, pp.Detail)
		}
		return
	}
	t.Fatalf("no potential plan for Probe Bears")
}
