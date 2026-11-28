package oracle

import (
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Famished Worldsire's Devour (CR 702.82): "As this creature enters, you may
// sacrifice any number of lands. It enters with three times that many +1/+1
// counters on it." The real card costs {5}{G}{G}{G}, which no 20-land mirror
// reaches, so its case only proves kgen lowers the printed card; the
// mechanism is driven by "Devour Probe", an inline {1}{G} creature carrying
// the identical K:Devour:3:Land line, whose case FAILS unless some seed has
// it enter after at least one land was sacrificed and wear the counters.
const devourProbeScript = "Name:Devour Probe\nManaCost:1 G\nTypes:Creature Leviathan\nPT:0/0\n" +
	"K:Devour:3:Land\n" +
	"Oracle:Devour land 3 (As this creature enters, you may sacrifice any number of lands. It enters with three times that many +1/+1 counters on it.)\n"

func devourCases() []hostedCmdrCase {
	probeDeck := append(deckOf(12, "Devour Probe"), deckOf(28, "Forest")...)
	worldsireDeck := append(deckOf(4, "Famished Worldsire"), deckOf(36, "Forest")...)
	bearsDeck := append(deckOf(8, "Grizzly Bears"), deckOf(32, "Forest")...)
	return []hostedCmdrCase{
		{
			name:     "devour_probe_land_sacrifice_counters",
			carrier:  "Devour Probe",
			seats:    [2]string{"hck-devour-a", "hck-devour-b"},
			decks:    [2][]string{probeDeck, bearsDeck},
			fixtures: map[string]string{"Devour Probe": devourProbeScript},
			reached:  devourProbeSacrificesLands,
		},
		{
			name:    "famished_worldsire_devour",
			carrier: "Famished Worldsire",
			seats:   [2]string{"hck-worldsire-a", "hck-worldsire-b"},
			decks:   [2][]string{worldsireDeck, bearsDeck},
			surface: true,
		},
	}
}

// devourProbeSacrificesLands: a probe entered the battlefield and received
// +1/+1 counters, the three-per-land grant, after seat 0 sacrificed at
// least one land (the devoured set; seat 0 has no other sacrifice).
func devourProbeSacrificesLands(g *GorgeSide) bool {
	sacrificed := 0
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		if events.IsSacrifice(*e) {
			if o := g.E.G.Obj(e.Obj); o != nil && o.Owner == 0 {
				sacrificed++
			}
			continue
		}
		if e.Kind == events.CounterChange && e.Counter == "P1P1" && e.Amount > 0 && sacrificed > 0 {
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Devour Probe" &&
				o.Zone == state.ZBattlefield {
				return true
			}
		}
	}
	return false
}
