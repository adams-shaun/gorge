package oracle

import (
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The conditions a pinned game must reach (pinnedGames, pinned_test.go). A condition
// is judged at each decision and holds for the game once it has been true at any.

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

// counterAtLeast is the condition that a permanent of the given name holds at least
// n counters of the given kind (a Spacecraft's CHARGE counters reaching its station
// threshold, CR 702.184a).
func counterAtLeast(name, counter string, n int) func(ref *GorgeSide) bool {
	return func(ref *GorgeSide) bool {
		g := ref.E.G
		for p := range g.Players {
			for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
				if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name && int(o.Counter(counter)) >= n {
					return true
				}
			}
		}
		return false
	}
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

// activated is the condition that an ability of a card of the given name was put on
// the stack at some point in the game.
func activated(name string) func(ref *GorgeSide) bool {
	return func(ref *GorgeSide) bool {
		for i := range ref.E.L.Events {
			ev := &ref.E.L.Events[i]
			if ev.Kind != events.AbilityPush {
				continue
			}
			if o := ref.E.G.Obj(ev.Obj); o != nil && o.Face() != nil && o.Face().Name == name {
				return true
			}
		}
		return false
	}
}

