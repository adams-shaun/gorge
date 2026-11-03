package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Rules of Acquisition's second mode: a Pump names the opponent, the chained
// GainControl names the permanent and hands it to the parent's player
// (NewController$ ParentTarget), and the chained draw is gated on the
// permanent having changed hands.
func TestRulesOfAcquisitionNinetyEighthGivesAndDraws(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sc := oracleScenario{
		Name: "rules-of-acquisition-98th",
		Setup: map[string]oracleSeat{
			"p0": {Hand: []string{"Rules of Acquisition"}, Battlefield: []string{"Grizzly Bears"}, Library: []string{"Island", "Island", "Island", "Island"}},
			"p1": {Battlefield: []string{"Serra Angel"}},
		},
		Steps: []oracleStep{
			{Op: "cast", Seat: 0, Card: "p0:Rules of Acquisition", Mana: "UUUUU",
				Targets: []string{"p1", "p0:Grizzly Bears"},
				Answers: []oracleAnswer{{Kind: "modes", Pick: []string{"Follow the 98th Rule"}}}},
			{Op: "resolve"},
		},
	}
	fails, _, run := runOracleScenario(reg, sc)
	if len(fails) > 0 {
		t.Fatalf("scenario failed:\n  %s", strings.Join(fails, "\n  "))
	}
	e := run.e
	bears := e.G.Obj(findBattlefield(t, e, 0, "Grizzly Bears", 0))
	hand := 0
	for i := range e.G.Objs {
		if o := &e.G.Objs[i]; o.Zone == state.ZHand && o.Owner == 0 {
			hand++
		}
	}
	if bears.Controller != 1 {
		t.Fatalf("Grizzly Bears controller = seat %d, want seat 1", bears.Controller)
	}
	if hand != 3 {
		t.Fatalf("seat 0 holds %d cards after resolving, want the 3 drawn", hand)
	}
}
