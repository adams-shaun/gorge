package rules

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 707.9b/707.9d: Dominion Infiltration's token is a copy "except it has
// changeling", and that exception is part of its copiable values. Croaking
// Counterpart then copies the token "except it's a 1/1 green Frog": a copy
// effect that provides a specific set of creature types does not copy the
// characteristic-defining ability that defines them, so the inherited
// Changeling ends and the copy is a Frog and nothing else. (Lignify makes the
// token a non-Frog Treefolk so Croaking Counterpart may target it.)
func TestCloneChangelingExceptionDroppedByLaterSetCreatureTypes(t *testing.T) {
	t.Parallel()
	const sc = `{"name":"changeling-settypes-order","cr":["707.9b","707.9d"],"why":"inline","setup":{"p0":{"battlefield":["Mountain","Mountain","Mountain","Mountain","Forest","Forest","Forest","Island","Island"],` +
		`"hand":["Dominion Infiltration","Lignify","Croaking Counterpart"]},"p1":{"battlefield":["Grizzly Bears"]}},` +
		`"steps":[{"op":"cast","seat":0,"card":"p0:Dominion Infiltration","mana":"RRRR","answers":[{"kind":"choose","pick":["X = 1"]},{"kind":"target","pick":["p1:Grizzly Bears"]}]},{"op":"resolve"},` +
		`{"op":"cast","seat":0,"card":"p0:Lignify","mana":"GG","targets":["p0:token:Grizzly Bears"]},{"op":"resolve"},` +
		`{"op":"cast","seat":0,"card":"p0:Croaking Counterpart","mana":"GUU","targets":["p0:token:Grizzly Bears"]},{"op":"resolve"}]}`
	res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(sc))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", res.Fails, strings.Join(res.Transcript, "\n"))
	}
	var frog *OracleSnapPerm
	for i, p := range res.Snapshots[len(res.Snapshots)-1].Permanents {
		if p.Token && p.PT == "1/1" {
			frog = &res.Snapshots[len(res.Snapshots)-1].Permanents[i]
		}
	}
	if frog == nil {
		t.Fatalf("precondition: no 1/1 Frog token copy on the battlefield: %+v", res.Snapshots[len(res.Snapshots)-1].Permanents)
	}
	if !slices.Contains(frog.Types, "Frog") {
		t.Fatalf("precondition: the Croaking Counterpart copy is not a Frog: %v", frog.Types)
	}
	if frog.AllCreatureTypes || slices.Contains(frog.Keywords, "Changeling") || len(frog.Types) != 2 {
		t.Errorf("Frog copy of a changeling token = types %v all creature types %v keywords %v; want [Creature Frog], false, no Changeling",
			frog.Types, frog.AllCreatureTypes, frog.Keywords)
	}
}
