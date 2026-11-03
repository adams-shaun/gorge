package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// WARNING: pins drift. A synthetic pair is two decks dealt from the lowered card list, so lowering one
// more card moves what every pair holds: a pin by index goes silently empty, or red
// on a game that no longer has the card the test is named for. A pin therefore names
// what its pairs must reach, through synthPairsReaching, and fails when one does not;
// a regression whose deck must not move at all is a recorded repro instead
// (stdReproLockstep).

// synthPairsReaching is synthPairsOn that also fails any pair whose game never
// satisfies one of the reached predicates at a decision. Each predicate is judged
// on its own, so the conjunction is "every one held at some point in the game".
func synthPairsReaching(t *testing.T, name string, cfg Config, pairs []int, reached ...func(ref *GorgeSide) bool) {
	t.Helper()
	synthPairsReachingRun(t, name, RunConfig{Config: cfg}, 1, pairs, nil, reached...)
}

// synthPairsReachingRun is synthPairsReaching on the run rc, with the mulligans each
// player may take and check run on every finished pair.
func synthPairsReachingRun(t *testing.T, name string, rc RunConfig, mulligans int, pairs []int, check func(pair int, res *Result), reached ...func(ref *GorgeSide) bool) {
	t.Helper()
	hit := make([]bool, len(reached))
	inner := rc.Check
	rc.Check = func(ref *GorgeSide, ker Side) *Divergence {
		for i, r := range reached {
			if !hit[i] && r(ref) {
				hit[i] = true
			}
		}
		if inner != nil {
			return inner(ref, ker)
		}
		return nil
	}
	synthPairsMulligans(t, name, rc, mulligans, pairs, func(pair int, res *Result) {
		for i := range hit {
			if !hit[i] {
				t.Errorf("pair %d of %s never reaches condition %d of %d: the pin tests nothing (a lowering moved the deal; re-pin by the condition)", pair, name, i+1, len(hit))
			}
			hit[i] = false
		}
		if check != nil {
			check(pair, res)
		}
	})
}

// cardOut is the condition that a card of the given name is on the stack, the
// battlefield, a graveyard or in exile at a decision: it was cast or entered.
func cardOut(name string) func(ref *GorgeSide) bool {
	return func(ref *GorgeSide) bool {
		g := ref.E.G
		if inZoneNamed(g, state.ZStack, 0, name) {
			return true
		}
		for p := range g.Players {
			for _, z := range []state.Zone{state.ZBattlefield, state.ZGraveyard, state.ZExile} {
				if inZoneNamed(g, z, state.PlayerID(p), name) {
					return true
				}
			}
		}
		return false
	}
}

// onBattlefield is the condition that a card of the given name is on the battlefield.
func onBattlefield(name string) func(ref *GorgeSide) bool {
	return func(ref *GorgeSide) bool {
		for p := range ref.E.G.Players {
			if inZoneNamed(ref.E.G, state.ZBattlefield, state.PlayerID(p), name) {
				return true
			}
		}
		return false
	}
}

// onStack is the condition that a card or ability of the given card name is on the stack.
func onStack(name string) func(ref *GorgeSide) bool {
	return func(ref *GorgeSide) bool { return inZoneNamed(ref.E.G, state.ZStack, 0, name) }
}

func inZoneNamed(g *state.Game, z state.Zone, p state.PlayerID, name string) bool {
	for _, id := range g.Zone(z, p) {
		if o := g.Obj(id); o != nil {
			if f := o.Face(); f != nil && f.Name == name {
				return true
			}
		}
	}
	return false
}

// cardSeen is the condition that a card of the given name is in a hand, or has been
// cast or entered: it was dealt to a player who could use it.
func cardSeen(name string) func(ref *GorgeSide) bool {
	out := cardOut(name)
	return func(ref *GorgeSide) bool {
		if out(ref) {
			return true
		}
		for p := range ref.E.G.Players {
			if inZoneNamed(ref.E.G, state.ZHand, state.PlayerID(p), name) {
				return true
			}
		}
		return false
	}
}

// anyOf is the condition that one of conds holds.
func anyOf(conds ...func(ref *GorgeSide) bool) func(ref *GorgeSide) bool {
	return func(ref *GorgeSide) bool {
		for _, c := range conds {
			if c(ref) {
				return true
			}
		}
		return false
	}
}

// anyOut is the condition that a card of one of the given names was cast or entered.
func anyOut(names ...string) func(ref *GorgeSide) bool {
	conds := make([]func(ref *GorgeSide) bool, len(names))
	for i, n := range names {
		conds[i] = cardOut(n)
	}
	return anyOf(conds...)
}

// madeRestrictedMana is the condition that mana carrying a spending restriction has
// been added to a pool.
func madeRestrictedMana(ref *GorgeSide) bool {
	for i := range ref.E.L.Events {
		ev := &ref.E.L.Events[i]
		if ev.Kind == events.ManaAdd && ev.Amount > 0 && strings.HasPrefix(ev.Text, "mana-restriction:") {
			return true
		}
	}
	return false
}

// tapTriggerFromManaTap is the condition that a mana tap put a Taps trigger on gorge's
// stack: a trigger_push for a permanent whose tap, followed by the mana it added, sits
// just before it (an attack's tap adds no mana). A non-empty listener also requires the
// trigger to be that card's. It remembers how far into the log it has read, so it serves
// one game.
func tapTriggerFromManaTap(listener string) func(ref *GorgeSide) bool {
	seen := 0
	return func(ref *GorgeSide) bool {
		evs := ref.E.L.Events
		for ; seen < len(evs); seen++ {
			if evs[seen].Kind != events.TriggerPush {
				continue
			}
			if listener != "" {
				o := ref.E.G.Obj(evs[seen].Obj)
				if o == nil || o.Face() == nil || o.Face().Name != listener {
					continue
				}
			}
			mana := false
			for j := seen - 1; j >= max(0, seen-8); j-- {
				switch evs[j].Kind {
				case events.ManaAdd:
					mana = true
				case events.Tap:
					if mana && evs[j].Obj == evs[seen].Obj {
						return true
					}
				}
			}
		}
		return false
	}
}
