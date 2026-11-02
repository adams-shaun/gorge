package rules

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The remaining unbatched multi-object battlefield departures.
// Kalitas, Traitor of Ghet leaving in the SAME batch as the opponent's
// creatures still exiles them (CR 614.6, 616.1, 603.10); a dying watcher
// (Psychomancer's Harbinger of Despair trigger, which also names exile)
// sees every simultaneous member leave. One scenario per batch path this
// change wired, plus controls:
//   - multi-target ChangeZone (engine/effects/zone_change.go's mover loop):
//     Curse of the Swine exiling the watcher and another artifact creature.
//     Kalitas cannot exercise an EXILE destination (its replacement replaces
//     would-die, Origin$ Battlefield -> Destination$ Graveyard, and no corpus
//     spell does a multi-target ChangeZone to a graveyard), so the watcher's
//     exile-inclusive death trigger is the observable.
//   - SacrificeAll (engine/effects/sacrificeall.go, batched and
//     pinned here): All Is Dust sweeping both boards with Kalitas present.
const massRestScenarios = `[
 {"name": "curse-of-the-swine-multi-exile-watcher",
  "setup": {"p0": {"battlefield": ["Psychomancer", "Bronze Sable"], "hand": ["Curse of the Swine"]}},
  "steps": [{"op": "cast", "seat": 0, "card": "p0:Curse of the Swine", "mana": "CCUU",
             "answers": [{"kind": "any", "pick": ["2"]}],
             "targets": ["p0:Psychomancer", "p0:Bronze Sable"]}, {"op": "resolve"}],
  "expect": [
   {"card": "p0:Psychomancer", "zone": "exile"},
   {"card": "p0:Bronze Sable", "zone": "exile"},
   {"count": {"seat": 0, "zone": "battlefield", "name": "Boar", "token": true}, "eq": 2},
   {"life": {"p0": 22, "p1": 18}}]},
 {"name": "curse-of-the-swine-watcher-alone-control",
  "setup": {"p0": {"battlefield": ["Psychomancer", "Bronze Sable"], "hand": ["Curse of the Swine"]}},
  "steps": [{"op": "cast", "seat": 0, "card": "p0:Curse of the Swine", "mana": "CUU",
             "answers": [{"kind": "any", "pick": ["1"]}],
             "targets": ["p0:Psychomancer"]}, {"op": "resolve"}],
  "expect": [
   {"card": "p0:Psychomancer", "zone": "exile"},
   {"card": "p0:Bronze Sable", "zone": "battlefield"},
   {"life": {"p0": 21, "p1": 19}}]},
 {"name": "all-is-dust-kalitas-sacrificeall-batch",
  "setup": {"p0": {"battlefield": ["Kalitas, Traitor of Ghet"], "hand": ["All Is Dust"]},
            "p1": {"battlefield": ["Wall of Roots", "Grizzly Bears"]}},
  "steps": [{"op": "cast", "seat": 0, "card": "p0:All Is Dust", "mana": "CCCCCCC"}, {"op": "resolve"}],
  "expect": [
   {"count": {"seat": 0, "zone": "graveyard", "name": "Kalitas, Traitor of Ghet"}, "eq": 1},
   {"count": {"seat": 1, "zone": "exile", "name": "Wall of Roots"}, "eq": 1},
   {"count": {"seat": 1, "zone": "exile", "name": "Grizzly Bears"}, "eq": 1},
   {"count": {"seat": 0, "zone": "battlefield", "name": "Zombie", "token": true}, "eq": 2},
   {"graveyard_size": {"p1": 0}}]},
 {"name": "all-is-dust-no-kalitas-control",
  "setup": {"p0": {"hand": ["All Is Dust"]},
            "p1": {"battlefield": ["Wall of Roots", "Grizzly Bears"]}},
  "steps": [{"op": "cast", "seat": 0, "card": "p0:All Is Dust", "mana": "CCCCCCC"}, {"op": "resolve"}],
  "expect": [
   {"count": {"seat": 1, "zone": "graveyard", "name": "Wall of Roots"}, "eq": 1},
   {"count": {"seat": 1, "zone": "graveyard", "name": "Grizzly Bears"}, "eq": 1},
   {"graveyard_size": {"p1": 2}}]}
]`

func TestMassRestDeparturesKeepTriggersAndReplacements(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	var scs []oracleScenario
	dec := json.NewDecoder(strings.NewReader(massRestScenarios))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&scs); err != nil {
		t.Fatal(err)
	}
	for _, sc := range scs {
		sc := sc
		t.Run(sc.Name, func(t *testing.T) {
			t.Parallel()
			fails, transcript, run := runOracleScenario(reg, sc)
			if run != nil && run.e != nil {
				if diff := diffGames(run.e.G, replayFromLog(t, run.cfg, run.e.L.Events)); diff != "" {
					t.Errorf("log-only replay differs:\n%s", diff)
				}
			}
			if len(fails) > 0 {
				t.Errorf("FAIL: %s\n  transcript:\n    %s", strings.Join(fails, "\n  FAIL: "), strings.Join(transcript, "\n    "))
			}
		})
	}
}
