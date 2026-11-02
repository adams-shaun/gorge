package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A ChangeZone's Attacking$ can name its defender instead of leaving the choice
// to CR 508.4: Shark Shredder's Attacking$ TriggeredTarget is the player it dealt
// damage to. Forge and XMage both put the creature in combat.
func TestChangeZoneAttackingSelectorNamesTheDefender(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	res, err := RunOracleScenarioJSON(reg, []byte(`{"name":"enters-attacking-named-defender","cr":["508.4"],"why":"an entering attacker with a named defender",
"setup":{"p0":{"battlefield":["Shark Shredder, Killer Clone"]},"p1":{"graveyard":["Grizzly Bears"]}},
"steps":[{"op":"attack","seat":0,"attackers":["p0:Shark Shredder, Killer Clone"],"defender":"p1"},{"op":"pass_to","step":"combat-damage"},
{"op":"resolve","answers":[{"kind":"target","pick":["p1:Grizzly Bears"]}]}]}`))
	if err != nil || len(res.Fails) != 0 {
		t.Fatalf("err=%v fails=%v", err, res.Fails)
	}
	for _, p := range res.Snapshots[len(res.Snapshots)-1].Permanents {
		if p.Name == "Grizzly Bears" {
			if !p.Tapped || !p.Attacking || p.Controller != 0 {
				t.Fatalf("Grizzly Bears entered as %+v, want tapped and attacking under seat 0", p)
			}
			return
		}
	}
	t.Fatal("Grizzly Bears never reached the battlefield")
}
