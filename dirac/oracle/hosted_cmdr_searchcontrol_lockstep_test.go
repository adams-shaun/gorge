package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// searchProbeScript is a {G} tutor: search your library for a creature card,
// reveal it, put it into your hand, then shuffle.
const searchProbeScript = "Name:Search Probe\nManaCost:G\nTypes:Sorcery\n" +
	"A:SP$ ChangeZone | Origin$ Library | Destination$ Hand | ChangeType$ Creature | ChangeNum$ 1 | SpellDescription$ Search your library for a creature card, reveal it, put it into your hand, then shuffle.\n" +
	"Oracle:Search your library for a creature card, reveal it, put it into your hand, then shuffle.\n"

// agentExiledFound: over the whole game the searching seat's own card left
// its library for exile while the other seat (agent) controlled an Opposition
// Agent (nothing else in these decks exiles from a library: the Agent's
// FoundSearchingLibrary$ replacement did it, on a search the agent seat
// answered).
func agentExiledFound(agent state.PlayerID) func(g *GorgeSide) bool {
	return func(g *GorgeSide) bool { return agentExiledFoundBy(g, agent) }
}

func agentExiledFoundBy(g *GorgeSide, agent state.PlayerID) bool {
	agents := map[state.ObjID]bool{}
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		if e.Kind != events.MoveZone {
			continue
		}
		o := g.E.G.Obj(e.Obj)
		if o == nil || o.Face() == nil {
			continue
		}
		if o.Face().Name == "Opposition Agent" && o.Owner == agent {
			if e.To == state.ZBattlefield {
				agents[e.Obj] = true
			} else if e.From == state.ZBattlefield {
				delete(agents, e.Obj)
			}
			continue
		}
		if o.Owner != agent && e.From == state.ZLibrary && e.To == state.ZExile && len(agents) > 0 {
			return true
		}
	}
	return false
}

// TestHostedCmdrSearchControlLockstep plays Opposition Agent decision by
// decision against gorge: one seat flashes in Agents, the other casts Search
// Probes for Grizzly Bears, so the Agent seat answers the other's search, the
// found card is exiled instead (asked which Agent applies when several are
// out), and the Agent seat may cast it with any mana. It runs with the Agents
// on seat 0 and again on seat 1 (a seat-0 searcher is PlayerID 0, which
// gorge's search scope must still read as a search in flight). The searching
// seat runs once with Search Probe (to hand) and once with Worldly Tutor
// (shuffle, then put it on top): gorge exiles the found card at its first
// move, before the shuffle, so the shuffled library no longer holds it.
func TestHostedCmdrSearchControlLockstep(t *testing.T) {
	for _, tutor := range []string{"Search Probe", "Worldly Tutor"} {
		for agent := state.PlayerID(0); agent < 2; agent++ {
			searchControlLockstep(t, tutor, agent)
		}
	}
}

func searchControlLockstep(t *testing.T, tutor string, agent state.PlayerID) {
	agentDeck := append(deckOf(14, "Opposition Agent"), deckOf(26, "Swamp")...)
	searchDeck := append(append(deckOf(14, tutor), deckOf(8, "Grizzly Bears")...), deckOf(18, "Forest")...)
	st := &dirac.SearchControlStats{}
	dirac.DebugSearchControlStats = st
	decks := [2][]string{agentDeck, searchDeck}
	if agent == 1 {
		decks = [2][]string{searchDeck, agentDeck}
	}
	runHostedCmdrShape(t, hostedCmdrShape{
		hostedCmdrCase: hostedCmdrCase{
			name:     "opposition_agent",
			carrier:  "Opposition Agent",
			seats:    [2]string{"hcs-agent-a", "hcs-agent-b"},
			decks:    decks,
			fixtures: map[string]string{"Search Probe": searchProbeScript},
		},
		real:    []string{"Opposition Agent", "Swamp", "Forest", "Grizzly Bears", "Worldly Tutor"},
		reached: agentExiledFound(agent),
	})
	dirac.DebugSearchControlStats = nil
	if st.Redirected == 0 || st.Exiled == 0 || st.Asked == 0 {
		t.Fatalf("%s, agent seat %d: kernel redirected %d picks, exiled %d found cards, asked %d orders; want each non-zero",
			tutor, agent, st.Redirected, st.Exiled, st.Asked)
	}
}
