package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Lightning, Army of One's Stagger is a replacement effect created with the
// player its first-strike damage hit remembered: damage the other attacker
// deals to that player in the regular damage step is doubled (CR 614.1,
// 120.3).
func TestStaggerDoublesLaterCombatDamageToTheRememberedPlayer(t *testing.T) {
	steps := `{"op":"attack","seat":0,"attackers":["p0:Lightning, Army of One","p0:Hellspur Brute"],"defender":"p1"}`
	for i := 0; i < 6; i++ {
		steps += `,{"op":"pass","seat":0},{"op":"pass","seat":1}`
	}
	scenario := `{"name":"stagger","cr":["614.1"],"why":"damage to the remembered player is doubled",
 "setup":{"p0":{"battlefield":["Lightning, Army of One","Hellspur Brute"]},"p1":{"battlefield":["Mountain"],"life":17}},"steps":[` + steps + `]}`
	res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(scenario))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario did not play cleanly: %v", res.Fails)
	}
	last := res.Snapshots[len(res.Snapshots)-1]
	if got := last.Players[1].Life; got != 4 {
		t.Fatalf("defender at %d life; want 4 (3 first-strike damage, then 5 doubled to 10)", got)
	}
}
