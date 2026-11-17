package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestMartyrOfSandsRevealsXWhiteCards: "{1}, Reveal X white cards from your
// hand, Sacrifice: gain three times X life". The reveal election announces X
// (CR 601.2b); the revealed cards stay in hand (CR 701.20a).
func TestMartyrOfSandsRevealsXWhiteCards(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	sc := oracleScenario{
		Setup: map[string]oracleSeat{"p0": {
			Battlefield: []string{"Martyr of Sands", "Plains"},
			Hand:        []string{"Suntail Hawk", "Savannah Lions", "Serra Angel", "Grizzly Bears"},
		}},
		Steps: []oracleStep{
			{Op: "activate", Seat: 0, Card: "p0:Martyr of Sands", Ability: "gain three times X life", Mana: "W",
				Answers: []oracleAnswer{{Kind: "choose", Pick: []string{"p0:Suntail Hawk", "p0:Savannah Lions", "p0:Serra Angel"}}}},
			{Op: "resolve"},
		},
		Expect: []oracleExpect{
			{Life: map[string]int32{"p0": 29}},
			{Card: "p0:Martyr of Sands", Zone: "graveyard"},
			{Card: "p0:Suntail Hawk", Zone: "hand"},
			{Card: "p0:Savannah Lions", Zone: "hand"},
			{Card: "p0:Serra Angel", Zone: "hand"},
			{HandSize: map[string]int{"p0": 4}},
		},
	}
	fails, _, _ := runOracleScenario(reg, sc)
	if len(fails) != 0 {
		t.Fatalf("scenario failed: %v", fails)
	}
}
