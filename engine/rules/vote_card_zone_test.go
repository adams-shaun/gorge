package rules

// Custodi Squire's Will of the Council vote reads Zone$ Graveyard: "each
// player votes for an artifact, creature, or enchantment card in your
// graveyard". The card ballot used to walk only the battlefield, so the
// Squire itself was offered and voted for, and the remembered winner was
// never in the graveyard the return ability moves from.
//
// The ETB is driven end to end on the real corpus card: the Squire enters
// through a logged MoveZone, the ChangesZone trigger resolves the compiled
// Vote SA, and every vote ask is answered (the check is the BALLOT, not the
// policy). An empty graveyard ballot must pose no ask at all.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// custodiVoteAsk is a copied vote decision: Pending's memory is retired at
// the next Submit, so the ballot is copied before answering it.
type custodiVoteAsk struct {
	player  state.PlayerID
	options []decision.Option
}

// drainCustodiVotes resolves the Squire's ETB trigger, answering every
// priority and trigger-order ask and recording each card-vote ask before
// taking its first option. It stops once the stack and trigger queue are
// empty.
func drainCustodiVotes(t *testing.T, e *Engine, limit int) []custodiVoteAsk {
	t.Helper()
	var asks []custodiVoteAsk
	for i := 0; i < limit; i++ {
		if len(e.G.Stack) == 0 && len(e.pendingTriggers) == 0 {
			return asks
		}
		d := e.Pending()
		if d == nil {
			if len(e.pendingTriggers) > 0 {
				e.putTriggersOnStack()
				continue
			}
			e.resolveTop()
			continue
		}
		switch d.Kind {
		case decision.KPriority:
			passed := false
			for _, o := range d.Options {
				if o.Kind == "pass" {
					submitChoices(t, e, o.Index)
					passed = true
					break
				}
			}
			if !passed {
				t.Fatalf("priority decision with no pass option: %+v", d.Options)
			}
		case decision.KTriggerOrder:
			order := make([]int, 0, len(d.Options))
			for _, o := range d.Options {
				order = append(order, o.Index)
			}
			submitChoices(t, e, order...)
		case decision.KChoose:
			if d.ResumeKind != "vote" {
				t.Fatalf("unexpected resolution ask %q while draining Custodi Squire", d.ResumeKind)
			}
			asks = append(asks, custodiVoteAsk{
				player:  d.Player,
				options: append([]decision.Option(nil), d.Options...),
			})
			choices := []int{}
			if len(d.Options) > 0 {
				choices = append(choices, d.Options[0].Index)
			}
			submitChoices(t, e, choices...)
		default:
			t.Fatalf("unexpected decision %v while draining Custodi Squire", d.Kind)
		}
	}
	t.Fatalf("Custodi Squire's ETB did not settle (stack %d, queued triggers %d)", len(e.G.Stack), len(e.pendingTriggers))
	return nil
}

// custodiSquireEngine seats the real Custodi Squire in p0's hand and seeds
// the named graveyard cards in p0's graveyard through logged MoveZones.
func custodiSquireEngine(t *testing.T, reg *cards.Registry, graveyard ...string) (*Engine, Config) {
	t.Helper()
	hand := append([]string{"Custodi Squire"}, graveyard...)
	e, cfg := miscHandsEngine(t, reg, hand, nil, nil, nil)
	for _, name := range graveyard {
		miscMoveByName(t, e, 0, name, state.ZGraveyard)
	}
	return e, cfg
}

// TestCustodiSquireVotesFromGraveyard is the Zone$ ballot pin: with a
// creature card and a land in the caster's graveyard, every voter's ballot
// offers only the creature card (VoteCard$ filters out the land) and the
// winner returns to its owner's hand. With no card in the graveyard, the
// ballot is empty, so no vote is asked and the Squire stays put.
func TestCustodiSquireVotesFromGraveyard(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)

	t.Run("ballot is the graveyard card only", func(t *testing.T) {
		e, cfg := custodiSquireEngine(t, reg, "Grizzly Bears", "Mountain")
		bears := miscBoardObjInZone(t, e, 0, "Grizzly Bears", state.ZGraveyard)
		land := miscBoardObjInZone(t, e, 0, "Mountain", state.ZGraveyard)
		// Precondition: both cards really are in the caster's graveyard, so
		// the ballot assertion below is not vacuous.
		if bears == 0 || land == 0 {
			t.Fatal("precondition: the graveyard fixtures are not both in p0's graveyard")
		}

		squire := miscMoveByName(t, e, 0, "Custodi Squire", state.ZBattlefield)
		if o := e.G.Obj(squire); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("Custodi Squire = %+v, want it on the battlefield", o)
		}

		asks := drainCustodiVotes(t, e, 60)
		if len(asks) != 2 {
			t.Fatalf("Custodi Squire posed %d vote asks, want one per player (2)", len(asks))
		}
		for _, ask := range asks {
			if len(ask.options) != 1 || ask.options[0].Obj != bears {
				t.Fatalf("seat %d's ballot = %+v, want exactly the Grizzly Bears (%d) from the graveyard",
					ask.player, ask.options, bears)
			}
			if ask.options[0].Kind != "vote_card" {
				t.Fatalf("seat %d's ballot option = %+v, want a vote_card", ask.player, ask.options[0])
			}
		}

		if o := e.G.Obj(bears); o == nil || o.Zone != state.ZHand || o.Controller != 0 {
			t.Fatalf("the voted Grizzly Bears = %+v, want returned to p0's hand", o)
		}
		if o := e.G.Obj(land); o == nil || o.Zone != state.ZGraveyard {
			t.Fatalf("the unchosen land = %+v, want it left in the graveyard", o)
		}
		if o := e.G.Obj(squire); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("Custodi Squire = %+v, want it still on the battlefield", o)
		}
		replayCheck(t, e, cfg)
	})

	t.Run("empty graveyard ballot asks nothing", func(t *testing.T) {
		e, cfg := custodiSquireEngine(t, reg)
		squire := miscMoveByName(t, e, 0, "Custodi Squire", state.ZBattlefield)
		if got := len(e.G.Zone(state.ZGraveyard, 0)); got != 0 {
			t.Fatalf("precondition: p0's graveyard holds %d cards, want 0", got)
		}

		asks := drainCustodiVotes(t, e, 60)
		if len(asks) != 0 {
			t.Fatalf("an empty graveyard ballot posed %d vote asks, want none: %+v", len(asks), asks)
		}
		if o := e.G.Obj(squire); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("Custodi Squire = %+v, want it still on the battlefield", o)
		}
		replayCheck(t, e, cfg)
	})
}

// miscBoardObjInZone finds the id of p's card named name in zone.
func miscBoardObjInZone(t *testing.T, e *Engine, p state.PlayerID, name string, zone state.Zone) state.ObjID {
	t.Helper()
	for _, id := range e.G.Zone(zone, p) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return id
		}
	}
	t.Fatalf("seat %d's zone %v has no %q", p, zone, name)
	return 0
}
