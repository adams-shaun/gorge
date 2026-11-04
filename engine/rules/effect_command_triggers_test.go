package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// runScenarioOrFail plays sc through the oracle runner and fails the test with
// the runner's transcript when any step or expectation does not hold.
func runScenarioOrFail(t *testing.T, sc oracleScenario) *oracleRun {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	fails, transcript, run := runOracleScenario(reg, sc)
	if len(fails) > 0 {
		t.Fatalf("scenario %s failed:\n  %s\ntranscript:\n  %s", sc.Name, strings.Join(fails, "\n  "), strings.Join(transcript, "\n  "))
	}
	return run
}

// The Naked Time's chapter II creates an effect whose own trigger watches from
// the command zone (an Effect with Triggers$ and TriggerZones$ Command): a
// creature its controller has enter later that turn gets a +1/+1 counter and
// haste (CR 714.2b, 611.2c). The effect lasts for the turn only (CR 514.2):
// a creature entering on the next turn gets nothing.
func TestEffectCommandZoneTriggerWatchesThisTurnOnly(t *testing.T) {
	t.Parallel()
	runScenarioOrFail(t, oracleScenario{
		Name: "naked-time-chapter-two",
		Setup: map[string]oracleSeat{
			"p0": {
				Hand:        []string{"The Naked Time", "Grizzly Bears", "Hill Giant"},
				Battlefield: []string{"Mountain", "Mountain", "Forest", "Forest"},
				Library:     []string{"Mountain", "Island", "Island", "Island", "Island", "Island"},
			},
			"p1": {Library: []string{"Island", "Island", "Island", "Island", "Island", "Island"}},
		},
		Steps: []oracleStep{
			{Op: "cast", Seat: 0, Card: "p0:The Naked Time", Mana: "RC"},
			{Op: "resolve"},
			{Op: "pass_to", Step: "main1", Active: "p1"},
			{Op: "pass_to", Step: "main1", Active: "p0"},
			{Op: "resolve", Expect: []oracleExpect{{Card: "p0:The Naked Time", Counters: map[string]int32{"LORE": 2}}}},
			{Op: "cast", Seat: 0, Card: "p0:Grizzly Bears", Mana: "GC"},
			{Op: "resolve", Expect: []oracleExpect{
				{Card: "p0:Grizzly Bears", Zone: "battlefield", PT: "3/3", Keywords: []string{"Haste"}},
			}},
			{Op: "pass_to", Step: "main1", Active: "p1"},
			{Op: "move", Card: "p0:Hill Giant", To: "battlefield"},
		},
		Expect: []oracleExpect{
			{Card: "p0:Hill Giant", Zone: "battlefield", PT: "3/3", NoKeywords: []string{"Haste"}},
			{Card: "p0:Grizzly Bears", PT: "3/3", NoKeywords: []string{"Haste"}},
		},
	})
}

// T'Pring, Never Parted's fight chain makes an Effect only when both chosen
// creatures are still legal targets (a ConditionDefined$ Targeted count gate),
// remembering both fighters; the Effect's dies trigger watches from the command
// zone for the rest of the turn (CR 611.2c, 701.14). Each fighter that dies
// this turn draws a card, whether it died in the fight or later.
func TestEffectRemembersBothFightersForTheTurn(t *testing.T) {
	t.Parallel()
	lib := []string{"Island", "Island", "Island", "Island", "Island", "Island"}
	runScenarioOrFail(t, oracleScenario{
		Name: "tpring-both-fighters-die",
		Setup: map[string]oracleSeat{
			"p0": {Hand: []string{"T'Pring, Never Parted"}, Battlefield: []string{"Grizzly Bears"}, Library: lib},
			"p1": {Battlefield: []string{"Grizzly Bears"}, Library: lib},
		},
		Steps: []oracleStep{
			{Op: "cast", Seat: 0, Card: "p0:T'Pring, Never Parted", Mana: "GUCCC"},
			{Op: "resolve", Targets: []string{"p0:Grizzly Bears", "p1:Grizzly Bears"}},
		},
		Expect: []oracleExpect{
			{Card: "p0:Grizzly Bears", Zone: "graveyard"},
			{Card: "p1:Grizzly Bears", Zone: "graveyard"},
			{HandSize: map[string]int{"p0": 2}},
		},
	})
	runScenarioOrFail(t, oracleScenario{
		Name: "tpring-survivor-dies-later",
		Setup: map[string]oracleSeat{
			"p0": {Hand: []string{"T'Pring, Never Parted", "Lightning Bolt"}, Battlefield: []string{"Hill Giant"}, Library: lib},
			"p1": {Battlefield: []string{"Grizzly Bears", "Savannah Lions"}, Library: lib},
		},
		Steps: []oracleStep{
			{Op: "cast", Seat: 0, Card: "p0:T'Pring, Never Parted", Mana: "GUCCC"},
			{Op: "resolve", Targets: []string{"p0:Hill Giant", "p1:Grizzly Bears"},
				Expect: []oracleExpect{{Card: "p1:Grizzly Bears", Zone: "graveyard"}, {HandSize: map[string]int{"p0": 2}}}},
			{Op: "cast", Seat: 0, Card: "p0:Lightning Bolt", Mana: "R", Targets: []string{"p1:Savannah Lions"}},
			{Op: "resolve", Expect: []oracleExpect{{Card: "p1:Savannah Lions", Zone: "graveyard"}, {HandSize: map[string]int{"p0": 1}}}},
		},
	})
}

// U.S.S. Farragut's landfall puts a creature from hand onto the battlefield and
// sets up the end-step trigger only when one was put there. At the next end step
// the trigger puts that creature on the bottom of its owner's library, and "when
// you do" is a reflexive trigger (CR 603.12) that draws only if it moved. The
// delayed trigger names the creature as it last was on the battlefield, so a
// creature that died in the meantime is not found again (CR 400.7).
func TestReflexiveDrawFollowsTheDelayedMove(t *testing.T) {
	t.Parallel()
	lib := []string{"Island", "Island", "Island", "Island", "Island", "Island"}
	setup := func(hand ...string) map[string]oracleSeat {
		return map[string]oracleSeat{
			"p0": {Hand: hand, Battlefield: []string{"U.S.S. Farragut, Bold Explorer"}, Library: lib},
			"p1": {Library: lib},
		}
	}
	runScenarioOrFail(t, oracleScenario{
		Name:  "farragut-moves-and-draws",
		Setup: setup("Forest", "Grizzly Bears"),
		Steps: []oracleStep{
			{Op: "play", Seat: 0, Card: "p0:Forest"},
			{Op: "resolve", Answers: []oracleAnswer{{Kind: "any", Pick: []string{"p0:Grizzly Bears"}}},
				Expect: []oracleExpect{{Card: "p0:Grizzly Bears", Zone: "battlefield", Keywords: []string{"Haste"}}}},
			{Op: "pass_to", Step: "end"},
			{Op: "resolve"},
		},
		Expect: []oracleExpect{
			{Card: "p0:Grizzly Bears", Zone: "library"},
			{HandSize: map[string]int{"p0": 1}},
		},
	})
	runScenarioOrFail(t, oracleScenario{
		Name:  "farragut-creature-died-no-draw",
		Setup: setup("Forest", "Grizzly Bears", "Lightning Bolt"),
		Steps: []oracleStep{
			{Op: "play", Seat: 0, Card: "p0:Forest"},
			{Op: "resolve", Answers: []oracleAnswer{{Kind: "any", Pick: []string{"p0:Grizzly Bears"}}}},
			{Op: "cast", Seat: 0, Card: "p0:Lightning Bolt", Mana: "R", Targets: []string{"p0:Grizzly Bears"}},
			{Op: "resolve"},
			{Op: "pass_to", Step: "end"},
			{Op: "resolve"},
		},
		Expect: []oracleExpect{
			{Card: "p0:Grizzly Bears", Zone: "graveyard"},
			{HandSize: map[string]int{"p0": 0}},
		},
	})
	runScenarioOrFail(t, oracleScenario{
		Name:  "farragut-nothing-put-no-trigger",
		Setup: setup("Forest", "Grizzly Bears"),
		Steps: []oracleStep{
			{Op: "play", Seat: 0, Card: "p0:Forest"},
			{Op: "resolve"},
			{Op: "pass_to", Step: "end"},
			{Op: "resolve"},
		},
		Expect: []oracleExpect{
			{Card: "p0:Grizzly Bears", Zone: "hand"},
			{HandSize: map[string]int{"p0": 1}},
		},
	})
}

// Temporal Anomaly mills four, then lets its controller put a permanent card
// from among them into their hand from wherever the mill put them: a milled card
// a replacement sent to exile instead is still among them (CR 614.6, 701.13a).
func TestMilledCardFoundInGraveyardOrExile(t *testing.T) {
	t.Parallel()
	for _, extra := range []string{"", "Rest in Peace"} {
		bf := []string{"Temporal Anomaly"}
		if extra != "" {
			bf = append(bf, extra)
		}
		runScenarioOrFail(t, oracleScenario{
			Name: "temporal-anomaly-" + extra,
			Setup: map[string]oracleSeat{
				"p0": {Battlefield: bf, LibraryTop: []string{"Grizzly Bears", "Lightning Bolt", "Island", "Island"},
					Library: []string{"Island", "Island", "Island"}},
				"p1": {Library: []string{"Island", "Island", "Island"}},
			},
			Steps: []oracleStep{
				{Op: "activate", Seat: 0, Card: "p0:Temporal Anomaly", AbilityIndex: intp(1), Mana: "GCCC"},
				{Op: "resolve", Answers: []oracleAnswer{{Kind: "any", Pick: []string{"p0:Grizzly Bears"}}}},
			},
			Expect: []oracleExpect{
				{Card: "p0:Grizzly Bears", Zone: "hand"},
				{Count: &oracleCount{Seat: 0, Zone: "battlefield", Name: "New Planet"}, Eq: intp(1)},
			},
		})
	}
}
