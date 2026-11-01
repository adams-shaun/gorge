package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 611.2c: the set an effect affects is locked in as it resolves, so one of
// them dying does not end it for the other.
func TestEffectGoesOnForTheOtherTarget(t *testing.T) {
	t.Parallel()
	sc, err := decodeOracleScenario([]byte(`{"name":"secret-tunnel-survivor-stays-unblockable","cr":["611.2c"],"why":"one target dying leaves the other unblockable","setup":{"p0":{"battlefield":["Secret Tunnel","Grizzly Bears","Runeclaw Bear"],"hand":[],"mana":"CCCCC"},"p1":{"battlefield":["Hill Giant"],"hand":["Terror"],"mana":"BB"}},"steps":[{"op":"activate","seat":0,"card":"p0:Secret Tunnel","ability_index":1,"mana":"CCCC","targets":["p0:Grizzly Bears","p0:Runeclaw Bear"]},{"op":"resolve"},{"op":"pass","seat":0},{"op":"cast","seat":1,"card":"p1:Terror","mana":"BB","targets":["p0:Runeclaw Bear"]},{"op":"resolve"},{"op":"pass_to","seat":0,"decision":"attackers"},{"op":"attack","seat":0,"attackers":["p0:Grizzly Bears"],"defender":"p1"}],"expect":[{"card":"p0:Runeclaw Bear","zone":"graveyard"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	sc.xmageFixture = true
	fails, transcript, run := runOracleScenario(testutil.CorpusRegistry(t), sc)
	if len(fails) != 0 {
		t.Fatalf("%v\n%s", fails, transcript)
	}
	e := run.e
	var bears, giant state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if e.G.Obj(id).Face().Name == "Grizzly Bears" {
			bears = id
		}
	}
	for _, id := range e.G.Zone(state.ZBattlefield, 1) {
		if e.G.Obj(id).Face().Name == "Hill Giant" {
			giant = id
		}
	}
	if e.canBlock(giant, bears) {
		t.Fatal("the surviving target can be blocked")
	}
}
