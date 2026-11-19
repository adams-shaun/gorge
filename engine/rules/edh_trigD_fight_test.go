package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// edh_trigD_fight_test.go plays the printed text of the EDH fight-family
// ticket's two cards through real games: Foe-Razer Regent (Mode$ Fight) and
// Neyith of the Dire Hunt (Mode$ FightOnce + Mode$ AttackerBlockedOnce).

func runTrigDScenario(t *testing.T, raw string) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	sc, err := decodeOracleScenario([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	fails, transcript, _ := runOracleScenario(reg, sc)
	if len(fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", fails, strings.Join(transcript, "\n"))
	}
}

// TestEDHTrigDFoeRazerFight: Foe-Razer Regent's own ETB makes it fight an
// opposing Grizzly Bears (the harness answers the "may" yes and picks the
// Bears). The Bears die (4 damage) and Razer's own hit fires its "whenever a
// creature you control fights" line; the delayed trigger puts two +1/+1
// counters on Razer at the next end step (6/7). The Bears' hit back is dealt
// by a creature Razer's controller does not control, so it adds no second
// instance (exactly two counters).
func TestEDHTrigDFoeRazerFight(t *testing.T) {
	t.Parallel()
	runTrigDScenario(t, `{"name":"foerazer-fight","cr":["701.14","603.7"],"why":"Mode$ Fight",
 "setup":{"p0":{"battlefield":["Foe-Razer Regent"]},"p1":{"battlefield":["Grizzly Bears"]}},
 "steps":[{"op":"pass_to","seat":0,"step":"end","active":"p0"},{"op":"resolve","seat":0}],
 "expect":[{"count":{"seat":1,"zone":"graveyard","name":"Grizzly Bears"},"eq":1},
  {"card":"p0:Foe-Razer Regent","pt":"6/7","counters":{"P1P1":2}}]}`)
}

// TestEDHTrigDFoeRazerCombatIsNotAFight: after the ETB fight above, Razer
// attacks and kills a blocking Giant Spider. That ordinary combat damage is
// not a fight, so the end step still adds exactly the fight's two counters.
func TestEDHTrigDFoeRazerCombatIsNotAFight(t *testing.T) {
	t.Parallel()
	runTrigDScenario(t, `{"name":"foerazer-combat","cr":["701.14d"],"why":"combat damage is not a fight",
 "setup":{"p0":{"battlefield":["Foe-Razer Regent"]},"p1":{"battlefield":["Grizzly Bears","Giant Spider"]}},
 "steps":[{"op":"attack","seat":0,"attackers":["p0:Foe-Razer Regent"],"defender":"p1"},
  {"op":"block","seat":1,"blocks":[["p1:Giant Spider","p0:Foe-Razer Regent"]]},
  {"op":"pass_to","seat":0,"step":"end","active":"p0"},{"op":"resolve","seat":0}],
 "expect":[{"count":{"seat":1,"zone":"graveyard","name":"Grizzly Bears"},"eq":1},
  {"count":{"seat":1,"zone":"graveyard","name":"Giant Spider"},"eq":1},
  {"card":"p0:Foe-Razer Regent","pt":"6/7","counters":{"P1P1":2}}]}`)
}

// TestEDHTrigDNeyithFightOnce: Prey Upon makes seat 0's Grizzly Bears fight
// an opposing one. Both creatures dealing damage is one fight, so Neyith's
// "whenever one or more creatures you control fight" draws exactly one card.
func TestEDHTrigDNeyithFightOnce(t *testing.T) {
	t.Parallel()
	runTrigDScenario(t, `{"name":"neyith-fight","cr":["701.14"],"why":"Mode$ FightOnce",
 "setup":{"p0":{"hand":["Prey Upon"],"battlefield":["Neyith of the Dire Hunt","Grizzly Bears","Forest"]},"p1":{"battlefield":["Grizzly Bears"]}},
 "steps":[{"op":"cast","seat":0,"card":"p0:Prey Upon","mana":"G","targets":["p0:Grizzly Bears","p1:Grizzly Bears"]},
  {"op":"resolve","seat":0},{"op":"resolve","seat":0}],
 "expect":[{"hand_size":{"p0":1}}]}`)
}

// TestEDHTrigDNeyithCombatIsNotAFight: an attack the opponent does not block
// deals combat damage, which is neither a fight nor a block: Neyith draws
// nothing.
func TestEDHTrigDNeyithCombatIsNotAFight(t *testing.T) {
	t.Parallel()
	runTrigDScenario(t, `{"name":"neyith-unblocked","cr":["701.14d"],"why":"combat damage is not a fight",
 "setup":{"p0":{"battlefield":["Neyith of the Dire Hunt","Grizzly Bears"]},"p1":{}},
 "steps":[{"op":"attack","seat":0,"attackers":["p0:Grizzly Bears"],"defender":"p1"},
  {"op":"pass_to","seat":0,"step":"end","active":"p0"}],
 "expect":[{"hand_size":{"p0":0}},{"life":{"p1":18}}]}`)
}

// TestEDHTrigDAttackerBlockedOnce: two of seat 0's attackers blocked in one
// declaration are ONE "become blocked" event, so Neyith draws once, not twice.
func TestEDHTrigDAttackerBlockedOnce(t *testing.T) {
	t.Parallel()
	runTrigDScenario(t, `{"name":"neyith-blocked","cr":["509.3c"],"why":"Mode$ AttackerBlockedOnce",
 "setup":{"p0":{"battlefield":["Neyith of the Dire Hunt","Grizzly Bears","Silvercoat Lion"]},"p1":{"battlefield":["Hill Giant","Giant Spider"]}},
 "steps":[{"op":"attack","seat":0,"attackers":["p0:Grizzly Bears","p0:Silvercoat Lion"],"defender":"p1"},
  {"op":"block","seat":1,"blocks":[["p1:Hill Giant","p0:Grizzly Bears"],["p1:Giant Spider","p0:Silvercoat Lion"]]},
  {"op":"resolve","seat":0}],
 "expect":[{"hand_size":{"p0":1}}]}`)
}
