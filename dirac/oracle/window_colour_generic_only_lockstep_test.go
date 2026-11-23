package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/events"
)

// TestWindowColourGenericOnlyReprosLockstep replays two autopay-surface games
// whose activation cost is generic mana plus a sacrifice, paid by Combo
// lands. Gorge pays through its manual mana window, whose colour answer takes
// the first of the source's ask order, so Luxury Suite ("Combo B R") and Foul
// Orchard ("Combo B G") float {B}; the kernel's planner took its own Produces
// preference (R and G), which the projection saw only in the pool.
func TestWindowColourGenericOnlyReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "window_colour_generic_only")
	stdReproNamed(t, "window_colour_generic_only_b")
}

// windowColourPaymentTally counts the runs of a sacrifice activation whose
// mana comes from a Combo land: an AbilityPush of one of the carriers, then a
// ManaAdd from Luxury Suite or Foul Orchard before the next priority. Both
// carriers' abilities cost generic mana alone, so the mechanism reached is
// the generic-only payment from a Combo source the rule answers.
type windowColourPaymentTally struct {
	last      int
	inAbility bool
	lastTap   string
	payments  int
}

// check is the four-seat Lockstep hook: it returns no divergence and only
// reads gorge's log.
func (w *windowColourPaymentTally) check(g *GorgeSide, _ Side) *Divergence {
	evs := g.E.L.Events
	for ; w.last < len(evs); w.last++ {
		e := &evs[w.last]
		switch e.Kind {
		case events.AbilityPush:
			o := g.E.G.Obj(e.Obj)
			w.inAbility = o != nil && o.Face() != nil &&
				(o.Face().Name == "Pious Evangel" || o.Face().Name == "Ravenous Harpy")
		case events.Priority:
			w.inAbility = false
		case events.Tap:
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil {
				w.lastTap = o.Face().Name
			}
		case events.ManaAdd:
			if w.inAbility && (w.lastTap == "Luxury Suite" || w.lastTap == "Foul Orchard") {
				w.payments++
			}
		}
	}
	return nil
}

// windowColourDeck carries both sacrifice activations, both Combo lands and
// creatures to sacrifice, with enough lands to pay the generic costs.
func windowColourDeck() []string {
	return deckOf(4, "Pious Evangel",
		deckOf(4, "Ravenous Harpy",
			deckOf(4, "Luxury Suite",
				deckOf(4, "Foul Orchard",
					deckOf(6, "Strandwalker",
						deckOf(4, "Vaporkin",
							deckOf(6, "Swamp",
								deckOf(8, "Plains")...)...)...)...)...)...)...)
}

// TestWindowColourGenericOnlyFourSeat plays the tail-32 pod with the sacrifice
// activations and both Combo lands: 20 seeds, no divergence, and the
// generic-only payment from a Combo source reached at least once (a test that
// never reaches it cannot see the rule it pins).
func TestWindowColourGenericOnlyFourSeat(t *testing.T) {
	tally := &windowColourPaymentTally{}
	deck := windowColourDeck()
	set := cardsettest.Synth(t, "tail-32")
	play := fourSeatSetPlay(t, set, [4][]string{deck, deck, deck, deck},
		nil, tally.check, nil, func(seed uint64) Policy { return castActivatePolicy(seed) })
	const seeds = 20
	fourSeatAtkClean(t, play, seeds)
	t.Logf("window colour payments from a Combo source: %d over %d seeds", tally.payments, seeds)
	if tally.payments == 0 {
		t.Fatal("no generic-only window payment from a Combo source was reached")
	}
}

// TestWindowColourGenericOnlyMutation restores the planner's Produces
// preference for the window-paid plan: the recorded game must diverge again,
// so the rule the repro pins is the colour the plan's taps take.
func TestWindowColourGenericOnlyMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	old := dirac.DebugWindowColourPlanner
	dirac.DebugWindowColourPlanner = true
	defer func() { dirac.DebugWindowColourPlanner = old }()
	r, err := ReadRepro(filepath.Join("testdata", "repro", "window_colour_generic_only.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, &ScriptPolicy{Actions: r.Actions})
	if res.Div == nil {
		t.Fatalf("mutation: no divergence in %d decisions", res.Decisions)
	}
	t.Logf("mutation: divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
}
