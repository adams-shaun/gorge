package rules

import "testing"

// Vulcan Mind-Meld makes two Effects, each owned by the player who exiled the
// cards it remembers (EffectOwner$): the target opponent exiles the top card
// of your library face down and may play it, and you exile the top two of
// theirs and may play those, spending mana as though it were mana of any type
// (CR 601.2g, 611.2). A may-play grant belongs to the Effect's owner, as the
// Effect's statics do in Forge, not to the player who cast the spell.
func TestEffectOwnerHoldsTheMayPlayGrant(t *testing.T) {
	t.Parallel()
	runScenarioOrFail(t, oracleScenario{
		Name: "mind-meld-each-exiler-plays",
		Setup: map[string]oracleSeat{
			"p0": {
				Hand:        []string{"Vulcan Mind-Meld"},
				Battlefield: []string{"Island", "Island", "Island"},
				LibraryTop:  []string{"Grizzly Bears"},
			},
			"p1": {
				Battlefield: []string{"Forest", "Forest"},
				LibraryTop:  []string{"Shock", "Mountain"},
			},
		},
		Steps: []oracleStep{
			{Op: "cast", Seat: 0, Card: "p0:Vulcan Mind-Meld", Mana: "U", Targets: []string{"p1"}},
			{Op: "resolve", Expect: []oracleExpect{
				{Card: "p0:Grizzly Bears", Zone: "exile"},
				{Card: "p1:Shock", Zone: "exile"},
				{Card: "p1:Mountain", Zone: "exile"},
				{Offered: &oracleOffered{Seat: 0, Kind: "play", Card: "p1:Mountain"}, Want: boolp(true)},
				{Offered: &oracleOffered{Seat: 0, Kind: "cast", Card: "p0:Grizzly Bears"}, Want: boolp(false)},
			}},
			{Op: "cast", Seat: 0, Card: "p1:Shock", Mana: "U", Targets: []string{"p1"}},
			{Op: "resolve", Expect: []oracleExpect{{Life: map[string]int32{"p1": 18}}}},
			{Op: "pass_to", Step: "main1", Active: "p1", Expect: []oracleExpect{
				{Offered: &oracleOffered{Seat: 1, Kind: "play", Card: "p1:Mountain"}, Want: boolp(false)},
			}},
			{Op: "cast", Seat: 1, Card: "p0:Grizzly Bears", Mana: "GG"},
			{Op: "resolve"},
		},
		Expect: []oracleExpect{
			{Count: &oracleCount{Seat: 1, Zone: "battlefield", Name: "Grizzly Bears"}, Eq: intp(1)},
		},
	})
}
