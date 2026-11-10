package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Great Train Heist's third Spree mode is `DB$ Effect | ValidTgts$ Opponent |
// Triggers$ TrigDamage | RememberObjects$ TargetedPlayer`, with TrigDamage
// `Mode$ DamageDone | ValidSource$ Creature.YouCtrl | ValidTarget$
// Player.IsRemembered | CombatDamage$ True`. The registration must carry the
// Effect's remembered PLAYER (the event arm used to encode objects only) and
// the DamageDone matcher must read that capture for Player.IsRemembered, so
// combat damage to the targeted opponent makes the tapped Treasure.
const oracleGreatTrainHeistThirdModeScenario = `{"name":"great-train-heist-third-mode-treasure","cr":["603.7","611.2"],
 "setup":{"p0":{"hand":["Great Train Heist"],"battlefield":["Grizzly Bears"],"mana":"RR"},"p1":{}},
 "steps":[
  {"op":"cast","seat":0,"card":"p0:Great Train Heist","mana":"RR","targets":["p1"],
   "answers":[{"kind":"modes","pick":["Choose target opponent. Whenever a creature you control deals combat damage to that player this turn, create a tapped Treasure token."]}]},
  {"op":"resolve"},
  {"op":"attack","seat":0,"attackers":["p0:Grizzly Bears"],"defender":"p1"},
  {"op":"pass_to","seat":0,"step":"combat-damage"},
  {"op":"resolve"},
  {"op":"pass_to","seat":0,"step":"end"}
 ]}`

func TestGreatTrainHeistThirdModeCreatesTreasure(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	res, err := RunOracleScenarioJSON(reg, []byte(oracleGreatTrainHeistThirdModeScenario))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", res.Fails, strings.Join(res.Transcript, "\n"))
	}
	if len(res.Snapshots) == 0 {
		t.Fatal("no snapshots")
	}
	last := res.Snapshots[len(res.Snapshots)-1]
	// precondition: the combat damage really happened (Grizzly Bears is 2/2)
	if len(last.Players) < 2 || last.Players[1].Life != 18 {
		t.Fatalf("p1 life = %+v, want 18 (the Bears must have connected)", last.Players)
	}
	n := 0
	for _, p := range last.Permanents {
		if p.Name != "Treasure Token" || p.Controller != 0 {
			continue
		}
		n++
		if !p.Tapped || !p.Token {
			t.Errorf("Treasure %+v: want a tapped token", p)
		}
	}
	if n != 1 {
		t.Fatalf("Treasure tokens = %d, want 1\n%s", n, strings.Join(res.Transcript, "\n"))
	}
}
