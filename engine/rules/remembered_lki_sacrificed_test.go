package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestRememberedLKISacrificedReadsDerivedToughness pins CR 608.2h for the
// count path's `RememberedLKI$CardPower`/`CardToughness` heads after
// `Sacrifice … RememberSacrificed$ True`: the sacrificed creature is read
// from the last-known-information capture taken before it left the
// battlefield, not from its printed face in the graveyard. Tribute to Hunger
// gains the anthem-boosted toughness 3; Kylox sizes its dig with the pumped
// total power.
func TestRememberedLKISacrificedReadsDerivedToughness(t *testing.T) {
	reg := testutil.CorpusRegistry(t)

	t.Run("tribute to hunger gains the anthem toughness", func(t *testing.T) {
		sc := oracleScenario{
			Setup: map[string]oracleSeat{
				"p0": {Hand: []string{"Tribute to Hunger"}},
				"p1": {Battlefield: []string{"Grizzly Bears", "Glorious Anthem"}},
			},
			Steps: []oracleStep{
				{Op: "cast", Seat: 0, Card: "p0:Tribute to Hunger", Mana: "CCB",
					Targets: []string{"p1"},
					Expect:  []oracleExpect{{Card: "p1:Grizzly Bears", PT: "3/3"}}},
				{Op: "resolve"},
			},
			Expect: []oracleExpect{
				{Card: "p1:Grizzly Bears", Zone: "graveyard"},
				{Life: map[string]int32{"p0": 23}},
			},
		}
		if fails, transcript, _ := runOracleScenario(reg, sc); len(fails) > 0 {
			t.Fatalf("%v\ntranscript:\n%v", fails, transcript)
		}
	})

	t.Run("kylox sums the pumped power", func(t *testing.T) {
		sc := oracleScenario{
			Setup: map[string]oracleSeat{
				"p0": {
					Battlefield: []string{"Kylox, Visionary Inventor", "Grizzly Bears", "Glorious Anthem"},
					LibraryTop:  []string{"Forest", "Forest", "Forest", "Forest"},
				},
				"p1": {},
			},
			Steps: []oracleStep{
				{Op: "attack", Seat: 0, Attackers: []string{"p0:Kylox, Visionary Inventor"}, Defender: "p1"},
				{Op: "resolve", Answers: []oracleAnswer{{Kind: "choose", Pick: []string{"Grizzly Bears"}}}},
			},
			Expect: []oracleExpect{
				{Card: "p0:Grizzly Bears", Zone: "graveyard"},
				{Count: &oracleCount{Seat: 0, Zone: "exile", Name: "Forest"}, Eq: intPtr(3)},
			},
		}
		if fails, transcript, _ := runOracleScenario(reg, sc); len(fails) > 0 {
			t.Fatalf("%v\ntranscript:\n%v", fails, transcript)
		}
	})
}

// TestRememberedLKIGreatestCardPowerAfterSacrifice pins the same capture for
// the `RememberedLKI$GreatestCardPower` head: Shadowgrange Archfiend's life
// gain reads the anthem-boosted power 3 of the creature its opponent
// sacrificed, not the printed 2 of the Grizzly Bears in the graveyard.
func TestRememberedLKIGreatestCardPowerAfterSacrifice(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	sc := oracleScenario{
		Setup: map[string]oracleSeat{
			"p0": {Hand: []string{"Shadowgrange Archfiend"}},
			"p1": {Battlefield: []string{"Grizzly Bears", "Glorious Anthem"}},
		},
		Steps: []oracleStep{
			{Op: "cast", Seat: 0, Card: "p0:Shadowgrange Archfiend", Mana: "CCCCCCB",
				Expect: []oracleExpect{{Card: "p1:Grizzly Bears", PT: "3/3"}}},
			{Op: "resolve", Answers: []oracleAnswer{{Kind: "choose", Pick: []string{"p1:Grizzly Bears"}}}},
		},
		Expect: []oracleExpect{
			{Card: "p1:Grizzly Bears", Zone: "graveyard"},
			{Life: map[string]int32{"p0": 23}},
		},
	}
	if fails, transcript, _ := runOracleScenario(reg, sc); len(fails) > 0 {
		t.Fatalf("%v\ntranscript:\n%v", fails, transcript)
	}
}

func intPtr(n int) *int { return &n }
