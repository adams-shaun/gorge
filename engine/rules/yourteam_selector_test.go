package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestPirAddsOneMoreCounter pins the YourTeamCtrl card predicate on a real
// replacement: Pir, Imaginative Rascal replaces an AddCounter event on a
// permanent "your team controls" with one more counter. gorge models no
// teams, so that is a permanent the controller controls. Before the alias the
// predicate was unknown, the replacement never applied, and Battlegrowth put
// one +1/+1 counter on the Lion instead of two.
func TestPirAddsOneMoreCounter(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	const sc = `{"name":"pir","cr":["614.1a"],"why":"Pir, Imaginative Rascal (XMage DoublingSeasonTest#test_LoyaltyCounters_Pir shape)","setup":{"p0":{"hand":["Battlegrowth"],"battlefield":["Pir, Imaginative Rascal","Silvercoat Lion","Forest"]}},"steps":[{"op":"cast","seat":0,"card":"p0:Battlegrowth","mana":"G","targets":["p0:Silvercoat Lion"]},{"op":"resolve","seat":0}],"expect":[{"card":"p0:Silvercoat Lion","counters":{"P1P1":2}},{"card":"p0:Silvercoat Lion","pt":"4/4"}]}`
	res, err := RunOracleAuditScenarioJSON(reg, []byte(sc), true)
	if err != nil || len(res.Fails) != 0 {
		t.Fatalf("scenario: err=%v fails=%v\n%s", err, res.Fails, strings.Join(res.Transcript, "\n"))
	}
}

// TestSylviaGrantsDoubleStrikeToOwnDragon pins Forge's CardProperty
// YourTeamCtrl in a static: Sylvia Brightspear's "Dragons your team controls
// have double strike" grants the keyword to the Dragon her controller
// controls, and not to the opponent's Dragon. Before the alias the filter
// matched nothing and no Dragon gained double strike.
func TestSylviaGrantsDoubleStrikeToOwnDragon(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	const sc = `{"name":"sylvia","cr":["613.1"],"why":"Sylvia Brightspear: Dragons your team controls have double strike","setup":{"p0":{"battlefield":["Sylvia Brightspear","Shivan Dragon"]},"p1":{"battlefield":["Shivan Dragon"]}},"steps":[]}`
	res, err := RunOracleScenarioJSON(reg, []byte(sc))
	if err != nil || len(res.Fails) != 0 {
		t.Fatalf("scenario: err=%v fails=%v\n%s", err, res.Fails, strings.Join(res.Transcript, "\n"))
	}
	snap := res.Snapshots[len(res.Snapshots)-1]
	has := func(ref string) (bool, bool) {
		p, ok := snapPerm(snap, ref)
		if !ok {
			return false, false
		}
		for _, k := range p.Keywords {
			if k == "Double Strike" {
				return true, true
			}
		}
		return false, true
	}
	if own, ok := has("p0:Shivan Dragon"); !ok || !own {
		t.Fatalf("precondition: p0 Shivan Dragon on battlefield=%v, double strike=%v; want both true (keywords: %v)", ok, own, snap.Permanents)
	}
	if opp, ok := has("p1:Shivan Dragon"); !ok || opp {
		t.Fatalf("precondition: p1 Shivan Dragon on battlefield=%v, double strike=%v; want present without double strike", ok, opp)
	}
}
