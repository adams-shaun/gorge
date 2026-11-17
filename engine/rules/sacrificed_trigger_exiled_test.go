package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 603.10a/603.6c/400.7e: It That Betrays's sacrifice trigger finds the
// card in the first public zone it went to. With Rest in Peace that is exile,
// though the script says Origin$ Graveyard.
func TestSacrificedTriggerFindsTheCardInExile(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	run := func(t *testing.T, rip bool) {
		t.Helper()
		bf := []string{"Flooded Strand"}
		if rip {
			bf = append(bf, "Rest in Peace")
		}
		sc := oracleScenario{
			Setup: map[string]oracleSeat{
				"p0": {Battlefield: bf, Library: []string{"Island"}},
				"p1": {Battlefield: []string{"It That Betrays"}},
			},
			Steps: []oracleStep{
				{Op: "activate", Seat: 0, Card: "p0:Flooded Strand"},
				{Op: "resolve"},
			},
			Expect: []oracleExpect{
				{Count: &oracleCount{Seat: 1, Zone: "battlefield", Name: "Flooded Strand"}, Eq: intp(1)},
				{Life: map[string]int32{"p0": 19}},
			},
		}
		fails, _, _ := runOracleScenario(reg, sc)
		if len(fails) != 0 {
			t.Fatalf("scenario failed: %v", fails)
		}
	}
	t.Run("control without Rest in Peace", func(t *testing.T) { run(t, false) })
	t.Run("exiled by Rest in Peace", func(t *testing.T) { run(t, true) })
}
