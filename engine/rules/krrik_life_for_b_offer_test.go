package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 601.2h / 602.2b: under K'rrik, Son of Yawgmoth ("For each {B} in a cost,
// you may pay 2 life rather than pay that mana") the player chooses how each
// {B} is paid. The grant used to settle only in the automatic resolver, which
// pays life only when mana is short, so a controller with an untapped Swamp
// was never offered the life payment. XMage PhyrexianManaTest
// #testKrrikActivatedAbility is the shape: pay Frozen Shade's {B} with life.
func TestKrrikOffersLifeForBWhileASwampCouldPay(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	run := func(answers []oracleAnswer) ([]string, *oracleRun) {
		t.Helper()
		sc := oracleScenario{
			Setup: map[string]oracleSeat{"p0": {Battlefield: []string{"K'rrik, Son of Yawgmoth", "Frozen Shade", "Swamp"}}},
			Steps: []oracleStep{
				{Op: "activate", Seat: 0, Card: "p0:Frozen Shade", Ability: "CARDNAME gets +1/+1 until end of turn.", Answers: answers},
				{Op: "resolve", Seat: 0},
			},
		}
		fails, _, r := runOracleScenario(reg, sc)
		return fails, r
	}
	tapped := func(r *oracleRun) bool {
		id, err := r.resolve("p0:Swamp")
		if err != nil {
			t.Fatal(err)
		}
		return r.e.G.Obj(id).Tapped
	}

	t.Run("pay 2 life leaves the Swamp untapped", func(t *testing.T) {
		fails, r := run([]oracleAnswer{{Kind: "choose", Pick: []string{"Pay 2 life"}}})
		if len(fails) != 0 {
			t.Fatalf("scenario failed: %v", fails)
		}
		if got := r.e.G.Players[0].Life; got != 18 {
			t.Fatalf("p0 life %d, want 18", got)
		}
		if tapped(r) {
			t.Fatal("Swamp tapped although the {B} was paid with life")
		}
		if bad := r.check(oracleExpect{Card: "p0:Frozen Shade", PT: "1/2"}); len(bad) > 0 {
			t.Fatal(bad)
		}
	})

	t.Run("pay B taps the Swamp and keeps the life", func(t *testing.T) {
		fails, r := run([]oracleAnswer{{Kind: "choose", Pick: []string{"Pay B"}}})
		if len(fails) != 0 {
			t.Fatalf("scenario failed: %v", fails)
		}
		if got := r.e.G.Players[0].Life; got != 20 {
			t.Fatalf("p0 life %d, want 20", got)
		}
		if !tapped(r) {
			t.Fatal("Swamp untapped although the {B} was paid with mana")
		}
	})
}
