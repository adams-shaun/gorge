package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// stolenBearSacrificed is p0 stealing p1's Grizzly Bears with Act of Treason and
// sacrificing it to Village Rites: the Bears leaves the battlefield under p0's
// control while p1 owns it.
const stolenBearSacrificed = `{"op":"cast","seat":0,"card":"p0:Act of Treason","mana":"RRR","targets":["p1:Grizzly Bears"]},{"op":"resolve"},
{"op":"cast","seat":0,"card":"p0:Village Rites","mana":"B","answers":[{"kind":"any","pick":["p1:Grizzly Bears"]}]},{"op":"resolve"}`

// TestRevoltReadsTheControllerAsThePermanentLeft: revolt asks whether "a
// permanent you controlled left the battlefield", so the thief who sacrifices a
// stolen permanent has revolt and its owner does not.
func TestRevoltReadsTheControllerAsThePermanentLeft(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sc, err := decodeOracleScenario([]byte(`{"name":"revolt-thief-sacs-stolen","cr":["Oracle"],"why":"revolt reads the controller",
"setup":{"p0":{"hand":["Act of Treason","Village Rites","Foot Mystic"],"mana":"RRRBBBBB"},"p1":{"battlefield":["Grizzly Bears"]}},
"steps":[` + stolenBearSacrificed + `,{"op":"cast","seat":0,"card":"p0:Foot Mystic","mana":"BBBB"},{"op":"resolve"}],
"expect":[{"count":{"seat":0,"zone":"battlefield","name":"Ninja","token":true},"eq":1}]}`))
	if err != nil {
		t.Fatal(err)
	}
	fails, transcript, run := runOracleScenario(reg, sc)
	if len(fails) != 0 {
		t.Fatalf("fails: %v\n%s", fails, strings.Join(transcript, "\n"))
	}
	if run.e.RevoltHolds(1) {
		t.Fatal("the stolen permanent's owner has revolt; p0 controlled it as it left")
	}
}

// TestDiedUnderYourControlReadsTheControllerAsItDied: the per-turn entry list's
// YouCtrl reads the permanent as it last existed (CR 603.10a), not the card's
// owner in the graveyard. Body Count draws for the stolen Bears.
func TestDiedUnderYourControlReadsTheControllerAsItDied(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	res, err := RunOracleAuditScenarioJSON(reg, []byte(`{"name":"body-count-stolen","cr":["603.10a"],"why":"a creature that died under your control",
"setup":{"p0":{"hand":["Act of Treason","Village Rites","Body Count"],"library_top":["Island","Island","Island","Island"],"mana":"RRRBBBBB"},"p1":{"battlefield":["Grizzly Bears"]}},
"steps":[`+stolenBearSacrificed+`,{"op":"cast","seat":0,"card":"p0:Body Count","mana":"BBBB"},{"op":"resolve"}],
"expect":[{"hand_size":{"p0":3}}]}`), true)
	if err != nil || len(res.Fails) != 0 {
		t.Fatalf("err=%v fails=%v\n%s", err, res.Fails, strings.Join(res.Transcript, "\n"))
	}
}
