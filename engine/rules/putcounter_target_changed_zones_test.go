package rules

import "testing"

// Deadly Complication may aim both modes at one suspected creature. The
// destroy mode resolves first, so the card in the graveyard is a new object
// (CR 400.7) and the +1/+1 counter mode does nothing to it: no counter rides
// the card back when it is later returned to the battlefield.
func TestPutCounterSkipsATargetThatChangedZones(t *testing.T) {
	t.Parallel()
	runScenarioOrFail(t, oracleScenario{
		Name: "deadly-complication-destroy-then-counter",
		Setup: map[string]oracleSeat{
			"p0": {
				Hand:        []string{"Rune-Brand Juggler", "Deadly Complication"},
				Battlefield: []string{"Swamp", "Swamp", "Swamp", "Mountain", "Mountain"},
				Library:     []string{"Island", "Island", "Island", "Island"},
			},
			"p1": {Library: []string{"Island", "Island", "Island", "Island"}},
		},
		Steps: []oracleStep{
			{Op: "cast", Seat: 0, Card: "p0:Rune-Brand Juggler", Mana: "BR"},
			{Op: "resolve", Answers: []oracleAnswer{{Kind: "any", Pick: []string{"p0:Rune-Brand Juggler"}}}},
			{Op: "resolve", Expect: []oracleExpect{{Card: "p0:Rune-Brand Juggler", Zone: "battlefield", Keywords: []string{"Menace"}, Counters: map[string]int32{"P1P1": 0}}}},
			{Op: "cast", Seat: 0, Card: "p0:Deadly Complication", Mana: "BRC", Answers: []oracleAnswer{
				{Kind: "modes", Pick: []string{"Destroy target creature", "Put a +1/+1 counter"}},
				{Kind: "any", Pick: []string{"p0:Rune-Brand Juggler", "p0:Rune-Brand Juggler"}},
			}},
			{Op: "resolve"},
		},
		Expect: []oracleExpect{{Card: "p0:Rune-Brand Juggler", Zone: "graveyard", Counters: map[string]int32{"P1P1": 0}}},
	})
}
