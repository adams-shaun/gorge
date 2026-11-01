package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

func playRuleScenario(t *testing.T, sc string) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	res, err := RunOracleAuditScenarioJSON(reg, []byte(sc), true)
	if err != nil || len(res.Fails) != 0 {
		t.Fatalf("scenario: err=%v fails=%v\n%s", err, res.Fails, res.Transcript)
	}
	if res.ReplayDiff != "" {
		t.Fatalf("replay diff: %s", res.ReplayDiff)
	}
}

// CR 608.2h: "that creature's power" reads the permanent the trigger names
// while it is on the battlefield, so an anthem counts.
func TestEnteredCreaturePowerReadsThePermanent(t *testing.T) {
	t.Parallel()
	playRuleScenario(t, `{"name":"terror-anthem-live-power","cr":["608.2h"],"why":"Terror reads the entered creature's live power (2+1=3)","setup":{"p0":{"battlefield":["Terror of the Peaks","Glorious Anthem"],"hand":["Grizzly Bears"]},"p1":{}},"steps":[{"op":"cast","seat":0,"card":"p0:Grizzly Bears","mana":"GG"},{"op":"resolve","answers":[{"kind":"target","pick":["p1"]}]},{"op":"resolve"}],"expect":[{"life":{"p1":17}}]}`)
	playRuleScenario(t, `{"name":"railway-brawler-anthem","cr":["608.2h"],"why":"Railway Brawler puts X counters, X = entered creature's live power = 3","setup":{"p0":{"battlefield":["Railway Brawler","Glorious Anthem"],"hand":["Grizzly Bears"]},"p1":{}},"steps":[{"op":"cast","seat":0,"card":"p0:Grizzly Bears","mana":"GG"},{"op":"resolve"},{"op":"resolve"}],"expect":[{"card":"p0:Grizzly Bears","counters":{"P1P1":3}}]}`)
}

// CR 608.2h: once the entered creature has left, the same read uses what it
// was as it last existed on the battlefield, not the card it was before it
// entered.
func TestEnteredCreatureThatLeftReadsItsLastPower(t *testing.T) {
	t.Parallel()
	playRuleScenario(t, `{"name":"goliath-entered-creature-bounced","cr":["608.2h"],"why":"Bears bounced in response: X is its last power, 3","setup":{"p0":{"battlefield":["Hamletback Goliath","Glorious Anthem"],"hand":["Grizzly Bears"]},"p1":{"hand":["Unsummon"],"mana":"U"}},"steps":[{"op":"cast","seat":0,"card":"p0:Grizzly Bears","mana":"GG"},{"op":"pass","seat":0},{"op":"pass","seat":1,"expect":[{"stack_size":1}]},{"op":"pass","seat":0},{"op":"cast","seat":1,"card":"p1:Unsummon","mana":"U","targets":["p0:Grizzly Bears"]},{"op":"resolve","answers":[{"kind":"any","pick":["yes"]}]}],"expect":[{"card":"p0:Grizzly Bears","zone":"hand"},{"card":"p0:Hamletback Goliath","counters":{"P1P1":3}}]}`)
}

// CR 202.3e: a cast trigger reads the spell on the stack, where X counts
// towards its mana value.
func TestCastTriggerReadsXInSpellManaValue(t *testing.T) {
	t.Parallel()
	playRuleScenario(t, `{"name":"kaervek-x-spell","cr":["202.3e","608.2h"],"why":"Blaze with X=2 has mana value 3 on the stack","setup":{"p0":{"battlefield":["Kaervek the Merciless"]},"p1":{"hand":["Blaze"],"mana":"RRR"}},"steps":[{"op":"pass_to","seat":1,"step":"main1","active":"p1","answers":[{"kind":"any","pick":[]}]},{"op":"cast","seat":1,"card":"p1:Blaze","mana":"RRR","targets":["p0"],"answers":[{"kind":"choose","pick":["2"]},{"kind":"target","pick":["p1"]}]},{"op":"resolve"}],"expect":[{"life":{"p1":17}}]}`)
}
