package oracle

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// newSynthPlayerOf deals the synthetic pool name once and returns a function that
// plays one of its pairs in lockstep, with the pair's matchup and seed, and the error
// of a pair the set refuses. Only recordPins plays pairs by number: a test pins a
// recorded game (pinnedGames), never a pair number, which a re-deal moves.
func newSynthPlayerOf(t *testing.T, name string, rc RunConfig, mulligans int) func(pair int) (*Matchup, uint64, Result, error) {
	t.Helper()
	cfg := rc.Config
	rc.MaxDecisions, rc.MaxTurns = 20000, 100
	c := &Corpus{Reg: synthReg(t), decks: map[string][]*cards.Card{}}
	set := cardsettest.Synth(t, name)
	pool, files, err := SynthPool(c.Reg, set, 1, SynthDecks)
	if err != nil {
		t.Fatal(err)
	}
	c.AddDecks(files)
	return func(i int) (*Matchup, uint64, Result, error) {
		p := PairingAt(pool, i, 1)
		m, tbl, err := SetMatchup(set, c, p.A, p.B, mulligans)
		if err != nil {
			return nil, 0, Result{}, err
		}
		k, err := SetSide(set, tbl, cfg)(m, p.Seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, p.Seed, k, RandomPolicy(p.Seed), rc)
		CloseSide(k)
		return m, p.Seed, res, nil
	}
}

// parkedPairs plays recorded games with two Rests in Peace out and a replacement
// order ask posed, and fails any in which a card reaches a graveyard under both.
func parkedPairs(t *testing.T, also ...func(ref *GorgeSide) bool) {
	t.Helper()
	rc := RunConfig{Config: Config{GorgeAutopaySurface: true}, Check: func(ref *GorgeSide, _ Side) *Divergence {
		if msg, ok := gravesAfterAnswers(ref); ok && msg != "" {
			return &Divergence{What: "parked move", Field: "graveyard", Detail: msg}
		}
		return nil
	}}
	settled := func(ref *GorgeSide) bool { _, ok := gravesAfterAnswers(ref); return ok }
	pinnedGamesRun(t, "std-big", rc, 1, append([]func(ref *GorgeSide) bool{settled, replacementAsk}, also...)...)
}

// replacementAsk is the condition that a replacement order ask is pending.
func replacementAsk(ref *GorgeSide) bool {
	if d := ref.Pending(); d != nil {
		for _, o := range d.Options {
			if o.Kind == "replacement" {
				return true
			}
		}
	}
	return false
}

// gravesAfterAnswers reads the board at a priority with the stack empty and two
// Rests in Peace out: the first's enter trigger emptied the graveyards, so whichever
// way each order ask was answered, a card there means a parked move went wrong. ok
// says the board was such a one.
func gravesAfterAnswers(ref *GorgeSide) (msg string, ok bool) {
	g := ref.E.G
	if len(g.Stack) != 0 || ref.E.Pending() == nil || ref.E.Pending().Kind != decision.KPriority {
		return "", false
	}
	rests := 0
	for p := range g.Players {
		for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
			if f := g.Obj(id).Face(); f != nil && f.Name == "Rest in Peace" {
				rests++
			}
		}
	}
	if rests < 2 {
		return "", false
	}
	for p := range g.Players {
		for _, id := range g.Zone(state.ZGraveyard, state.PlayerID(p)) {
			return fmt.Sprintf("turn %d: %s reached a graveyard under two Rests in Peace", g.Turn, g.Obj(id).Face().Name), true
		}
	}
	return "", true
}

// Two Rests in Peace compete for every move to a graveyard. A move paid as a
// cost, or a state-based death, waits on the CR 616.1 ask with the object where
// it was: the discarded or sacrificed card sits in its zone with the ability
// already on the stack.
func TestParkedMoveCostAndSBA(t *testing.T) {
	t.Parallel()
	parkedPairs(t)
}

// A casualty's Equipment stays attached to it until the state-based pass ends, and
// the next casualty's order ask is still that pass (CR 704.3): Lost Jitte's bearer is
// in exile with the Jitte still on it when the ask comes.
func TestParkedPassKeepsAttachments(t *testing.T) {
	t.Parallel()
	parkedPairs(t, cardOut("Lost Jitte"))
}

// A move made while a resolution runs waits too, and the resolution with it: a
// spell going to a graveyard stays on the stack through the ask, and so does the
// ability whose effect sacrificed or discarded.
func TestParkedMoveResolve(t *testing.T) {
	t.Parallel()
	parkedPairs(t)
}

// A Treasure sacrificed while a cost is tapped for poses the order ask with the
// earlier mana floating and its own not yet made (CR 605.3b).
func TestManaSacrificeAsksMidPayment(t *testing.T) {
	parkedPairs(t)
}

// A discard to hand size under two exile-instead replacements poses the order ask,
// and the last answer finishes the cleanup step.
func TestCleanupDiscardAsksReplacementOrder(t *testing.T) {
	parkedPairs(t)
}

// Several discards take all their picks before the first order ask.
func TestCleanupDiscardPicksBeforeOrderAsks(t *testing.T) {
	parkedPairs(t)
}

