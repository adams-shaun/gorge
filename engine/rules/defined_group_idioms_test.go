package rules

import "testing"

func boolp(v bool) *bool { return &v }

// Sarek, Federation Ambassador draws a card for every five creature types among
// its controller's non-Borg creatures, then gives the counters only if three or
// more cards were drawn this way (RememberDrawn$ with Count$RememberedSize):
// fifteen types draw three and give two counters to each creature; twelve
// draw two and give none.
func TestDrawnCountGateReadsCardsActuallyDrawn(t *testing.T) {
	t.Parallel()
	lib := []string{"Island", "Island", "Island", "Island", "Island", "Island"}
	for _, tc := range []struct {
		name     string
		board    []string
		hand     int
		counters int32
	}{
		{"fifteen-types", []string{"Sarek, Federation Ambassador", "Monster Mashup", "Yargle and Multani", "Norwood Ranger", "Loxodon Convert"}, 3, 2},
		{"twelve-types", []string{"Sarek, Federation Ambassador", "Monster Mashup", "Yargle and Multani", "Norwood Ranger"}, 2, 0},
	} {
		runScenarioOrFail(t, oracleScenario{
			Name: "sarek-" + tc.name,
			Setup: map[string]oracleSeat{
				"p0": {Battlefield: tc.board, Library: lib},
				"p1": {Library: lib},
			},
			Steps: []oracleStep{
				{Op: "pass_to", Step: "begin-combat", Active: "p0"},
				{Op: "resolve", Expect: []oracleExpect{
					{HandSize: map[string]int{"p0": tc.hand}},
					{Card: "p0:Norwood Ranger", Counters: map[string]int32{"P1P1": tc.counters}},
					{Card: "p0:Sarek, Federation Ambassador", Counters: map[string]int32{"P1P1": tc.counters}},
				}},
			},
		})
	}
}

// Spock, Voice of Reason counts the nontoken permanents its controller has that
// entered this turn (Count$Valid with !token and ThisTurnEntered): a creature
// cast this turn counts, the two tokens made this turn do not.
func TestValidCountSkipsTokensEnteredThisTurn(t *testing.T) {
	t.Parallel()
	runScenarioOrFail(t, oracleScenario{
		Name: "spock-nontoken-entered",
		Setup: map[string]oracleSeat{
			"p0": {Hand: []string{"Grizzly Bears", "Raise the Alarm"}, Battlefield: []string{"Spock, Voice of Reason"},
				Library: []string{"Island", "Island", "Island", "Island"}},
			"p1": {Library: []string{"Island", "Island", "Island", "Island"}},
		},
		Steps: []oracleStep{
			{Op: "cast", Seat: 0, Card: "p0:Grizzly Bears", Mana: "GC"},
			{Op: "resolve"},
			{Op: "cast", Seat: 0, Card: "p0:Raise the Alarm", Mana: "WC"},
			{Op: "resolve"},
			{Op: "activate", Seat: 0, Card: "p0:Spock, Voice of Reason", Mana: "CC"},
			{Op: "resolve"},
		},
		Expect: []oracleExpect{{HandSize: map[string]int{"p0": 1}}},
	})
}

// The Guardian of Forever's cost sacrifices another nontoken creature (a
// "!token" property, not a creature type) and returns that card from wherever
// it went, graveyard or exile, at the next end step (CR 603.7).
func TestNontokenSacrificeReturnsAtEndStep(t *testing.T) {
	t.Parallel()
	runScenarioOrFail(t, oracleScenario{
		Name: "guardian-token-is-no-sacrifice",
		Setup: map[string]oracleSeat{
			"p0": {Battlefield: []string{"The Guardian of Forever"}, Hand: []string{"Raise the Alarm"}},
		},
		Steps: []oracleStep{
			{Op: "cast", Seat: 0, Card: "p0:Raise the Alarm", Mana: "WC"},
			{Op: "resolve", Expect: []oracleExpect{
				{Offered: &oracleOffered{Seat: 0, Kind: "activate", Card: "p0:The Guardian of Forever"}, Want: boolp(false)},
			}},
		},
	})
	for _, extra := range []string{"", "Rest in Peace"} {
		bf := []string{"The Guardian of Forever", "Grizzly Bears"}
		if extra != "" {
			bf = append(bf, extra)
		}
		runScenarioOrFail(t, oracleScenario{
			Name: "guardian-returns-" + extra,
			Setup: map[string]oracleSeat{
				"p0": {Battlefield: bf, Library: []string{"Island", "Island", "Island"}},
				"p1": {Library: []string{"Island", "Island", "Island"}},
			},
			Steps: []oracleStep{
				{Op: "activate", Seat: 0, Card: "p0:The Guardian of Forever", Mana: "CCCC",
					Answers: []oracleAnswer{{Kind: "any", Pick: []string{"p0:Grizzly Bears"}}}},
				{Op: "resolve", Expect: []oracleExpect{{Card: "p0:Grizzly Bears", Zone: map[bool]string{false: "graveyard", true: "exile"}[extra != ""]}}},
				{Op: "pass_to", Step: "end"},
				{Op: "resolve"},
			},
			Expect: []oracleExpect{{Card: "p0:Grizzly Bears", Zone: "battlefield"}},
		})
	}
}

// Rules of Acquisition's third rule gains control of the targeted permanent,
// then its controller at targeting draws three (a Pump remembering the
// targeted permanent's controller, read by the chained Draw).
func TestTargetedControllerRememberedBeforeControlChanges(t *testing.T) {
	t.Parallel()
	runScenarioOrFail(t, oracleScenario{
		Name: "rules-of-acquisition-3rd",
		Setup: map[string]oracleSeat{
			"p0": {Hand: []string{"Rules of Acquisition"}, Library: []string{"Island", "Island", "Island", "Island"}},
			"p1": {Battlefield: []string{"Serra Angel"}, Library: []string{"Island", "Island", "Island", "Island"}},
		},
		Steps: []oracleStep{
			{Op: "cast", Seat: 0, Card: "p0:Rules of Acquisition", Mana: "UUCCC", Targets: []string{"p1:Serra Angel"},
				Answers: []oracleAnswer{{Kind: "modes", Pick: []string{"Follow the 3rd Rule"}}}},
			{Op: "resolve"},
		},
		Expect: []oracleExpect{
			{Count: &oracleCount{Seat: 0, Zone: "battlefield", Name: "Serra Angel"}, Eq: intp(1)},
			{HandSize: map[string]int{"p0": 0, "p1": 3}},
		},
	})
}
