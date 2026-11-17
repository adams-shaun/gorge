package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 707.2: a P/T (and type/keyword) choice made by an "as this enters"
// ability is part of the permanent's copiable values, so a later copy reads
// the CHOSEN values, not the printed */* and not its own printed P/T.
func TestBecomingACopyOfPrimalClayTakesItsChosenPT(t *testing.T) {
	t.Parallel()
	sc := oracleScenario{
		Name: "primal-clay-becomes-copy",
		CR:   []string{"707.2"},
		Why:  "Identity Thief becoming a copy of a Primal Clay that chose 3/3 must be 3/3",
		Setup: map[string]oracleSeat{
			"p0": {Hand: []string{"Primal Clay"}},
			"p1": {Battlefield: []string{"Identity Thief"}},
		},
		Steps: []oracleStep{
			{Op: "cast", Seat: 0, Card: "p0:Primal Clay", Mana: "CCCC"},
			{Op: "resolve", Seat: 0, Answers: []oracleAnswer{{Kind: "modes", Pick: []string{"CARDNAME is 3/3."}}}},
			{Op: "pass_to", Seat: 1, Step: "main1", Active: "p1"},
			{Op: "attack", Seat: 1, Attackers: []string{"p1:Identity Thief"}, Defender: "p0",
				Answers: []oracleAnswer{
					{Kind: "target", Pick: []string{"p0:Primal Clay"}},
				}},
			{Op: "resolve", Seat: 1,
				Answers: []oracleAnswer{
					{Kind: "any", Pick: []string{"yes"}},
				}},
		},
		Expect: []oracleExpect{{Card: "p1:Identity Thief", Zone: "battlefield", PT: "3/3"}},
	}
	fails, tr, _ := runOracleScenario(testutil.CorpusRegistry(t), sc)
	if len(fails) > 0 {
		t.Fatalf("%s\n  transcript:\n    %s", strings.Join(fails, "\n"), strings.Join(tr, "\n    "))
	}
}

// CR 707.5/707.6: a Clone that enters as a copy of a permanent whose copiable
// values include an "as this enters" replacement re-runs that replacement for
// the entering copy, and the copy's controller makes a NEW choice.
func TestCloneEnteringAsPrimalPlasmaChoosesAgain(t *testing.T) {
	t.Parallel()
	sc := oracleScenario{
		Name: "primal-plasma-clone-chooses-again",
		CR:   []string{"707.5", "707.6"},
		Why:  "Clone entering as a copy of a Primal Plasma that chose 1/6 must choose again, not inherit 1/6",
		Setup: map[string]oracleSeat{
			"p0": {Hand: []string{"Primal Plasma"}},
			"p1": {Hand: []string{"Clone"}},
		},
		Steps: []oracleStep{
			{Op: "cast", Seat: 0, Card: "p0:Primal Plasma", Mana: "CCCU"},
			{Op: "resolve", Seat: 0, Answers: []oracleAnswer{{Kind: "modes", Pick: []string{"CARDNAME is 1/6 with defender."}}}},
			{Op: "pass_to", Seat: 1, Step: "main1", Active: "p1"},
			{Op: "cast", Seat: 1, Card: "p1:Clone", Mana: "CCCU"},
			{Op: "resolve", Seat: 1, Answers: []oracleAnswer{
				{Kind: "choose", Pick: []string{"p0:Primal Plasma"}},
				{Kind: "modes", Pick: []string{"CARDNAME is 2/2 with flying."}},
			}},
		},
		Expect: []oracleExpect{
			{Card: "p1:Clone", Zone: "battlefield", PT: "2/2", Keywords: []string{"Flying"}, NoKeywords: []string{"Defender"}},
			{Card: "p0:Primal Plasma", Zone: "battlefield", PT: "1/6", Keywords: []string{"Defender"}},
		},
	}
	fails, tr, _ := runOracleScenario(testutil.CorpusRegistry(t), sc)
	if len(fails) > 0 {
		t.Fatalf("%s\n  transcript:\n    %s", strings.Join(fails, "\n"), strings.Join(tr, "\n    "))
	}
}

// CR 707.2 / 400.7: only an LKI copy (Identity Thief's Defined$ RememberedLKI)
// reads the copiable values a card last had on the battlefield. Body Double
// copies a Clone CARD in a graveyard, whose copiable values are the printed
// ones, even though that Clone last died as a Grizzly Bears copy.
func TestCopyOfGraveyardCloneReadsPrintedFaceNotLastCopy(t *testing.T) {
	t.Parallel()
	sc := oracleScenario{
		Name: "body-double-graveyard-clone-printed",
		CR:   []string{"707.2", "400.7"},
		Why:  "Body Double copying a graveyard Clone that last died as Grizzly Bears is the printed 0/0 Clone, not a Bears",
		Setup: map[string]oracleSeat{
			"p0": {Battlefield: []string{"Grizzly Bears"}, Hand: []string{"Clone", "Lightning Bolt", "Body Double"}},
		},
		Steps: []oracleStep{
			{Op: "cast", Seat: 0, Card: "p0:Clone", Mana: "CCCU"},
			{Op: "resolve", Seat: 0, Answers: []oracleAnswer{{Kind: "choose", Pick: []string{"p0:Grizzly Bears"}}}},
			{Op: "cast", Seat: 0, Card: "p0:Lightning Bolt", Mana: "R", Targets: []string{"p0:Clone"}},
			{Op: "resolve", Seat: 0},
			{Op: "cast", Seat: 0, Card: "p0:Body Double", Mana: "CCCCU"},
			{Op: "resolve", Seat: 0, Answers: []oracleAnswer{{Kind: "choose", Pick: []string{"p0:Clone"}}}},
		},
		Expect: []oracleExpect{
			{Card: "p0:Clone", Zone: "graveyard"},
			{Card: "p0:Body Double", Zone: "graveyard"},
		},
	}
	fails, tr, _ := runOracleScenario(testutil.CorpusRegistry(t), sc)
	if len(fails) > 0 {
		t.Fatalf("%s\n  transcript:\n    %s", strings.Join(fails, "\n"), strings.Join(tr, "\n    "))
	}
}
