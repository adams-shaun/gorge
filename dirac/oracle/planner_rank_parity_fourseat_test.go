package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPlannerRankParityFourSeat is the four-seat pod over the planner's
// source ranking (blk-a04): a filter land's non-tap-only extra alternative
// ({1},{T}: any colour, Surveillance Room) beside identical basics must rank
// the same in every seat, so a planner-paid activation never splits the
// engines on which permanent's projection flags moved. The check hook counts
// every priority decision whose acting player holds a payment plan that
// activates mana sources while a Surveillance Room of theirs sits untapped:
// exactly the plan shape the rank keys decide.
func TestPlannerRankParityFourSeat(t *testing.T) {
	deck := deckOf(4, "The Unbeatable Squirrel Girl",
		deckOf(4, "Surveillance Room",
			deckOf(2, "Shang-Chi, Master of Kung Fu",
				deckOf(13, "Forest",
					deckOf(8, "Island",
						deckOf(5, "Mountain", deckOf(4, "Plains")...)...)...)...)...)...)
	set := cardsettest.Synth(t, "std-msh")
	var planned int
	check := func(g *GorgeSide, _ Side) *Divergence {
		d := g.Pending()
		if d == nil {
			return nil
		}
		p := d.Player
		var filterUntapped bool
		for _, id := range g.E.G.Zone(state.ZBattlefield, p) {
			o := g.E.G.Obj(id)
			if o != nil && o.Face() != nil && o.Face().Name == "Surveillance Room" && !o.Tapped {
				filterUntapped = true
				break
			}
		}
		if !filterUntapped {
			return nil
		}
		for _, pa := range g.E.EnsurePaymentActions() {
			for i := range pa.Plans {
				if len(pa.Plans[i].Activations) > 0 {
					planned++
					return nil
				}
			}
		}
		return nil
	}
	play := fourSeatSetPlay(t, set, [4][]string{deck, deck, deck, deck},
		nil, check, nil, func(seed uint64) Policy { return combatPolicy(seed, nil) })
	const seeds = 20
	fourSeatAtkClean(t, play, seeds)
	t.Logf("PlannerRankParity four-seat: %d planner-paid payments with an untapped filter land", planned)
	if planned == 0 {
		t.Error("no planner-paid activation with an untapped filter land: the mechanism is unexercised")
	}
}
