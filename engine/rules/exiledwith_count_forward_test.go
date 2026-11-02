package rules

// exiledwith_count_forward_test.go pins the `ExiledWith$` count head on the
// FORWARD exiled-with link (CR 406.6 / 607.2a): Veteran
// Survivor's "As long as there are three or more cards exiled with Veteran
// Survivor" counts the cards its own Survival trigger exiled. That trigger
// names no ExiledWithSource, so its ChangeZone records only the source's
// forward Object.ExiledCards link, and the card it takes from an OPPONENT's
// graveyard lands in that opponent's exile. The count used to read only the
// reverse scalar in the controller's own exile and stayed at zero.

import "testing"

// veteranSurvivorTurn is one of p0's turns past the first: reach main1,
// attack with the Survivor (so it is tapped at the second main phase), then
// let its Survival trigger exile victim.
func veteranSurvivorTurn(victim string) string {
	return `{"op":"pass_to","seat":0,"step":"main1","active":"p0"},` +
		`{"op":"attack","seat":0,"attackers":["p0:Veteran Survivor"],"defender":"p1"},` +
		`{"op":"pass_to","seat":0,"step":"main2"},{"op":"resolve","targets":["p1:` + victim + `"]},`
}

func TestVeteranSurvivorCountsCardsItExiled(t *testing.T) {
	sc := `{"name":"veteran-survivor-exiledwith","cr":["406.6","607.2a","613.4c"],"why":"inline",` +
		`"setup":{"p0":{"battlefield":["Veteran Survivor"]},` +
		`"p1":{"graveyard":["Hill Giant","Sol Ring","Shock"]}},"steps":[` +
		`{"op":"attack","seat":0,"attackers":["p0:Veteran Survivor"],"defender":"p1"},` +
		`{"op":"pass_to","seat":0,"step":"main2"},{"op":"resolve","targets":["p1:Hill Giant"]},` +
		veteranSurvivorTurn("Sol Ring") +
		`{"op":"pass_to","seat":0,"decision":"priority","expect":[` +
		`{"card":"p0:Veteran Survivor","pt":"2/1","no_keywords":["Hexproof"]}]},` +
		veteranSurvivorTurn("Shock") +
		`{"op":"pass_to","seat":0,"decision":"priority","expect":[` +
		`{"card":"p0:Veteran Survivor","pt":"5/4","keywords":["Hexproof"]}]}]}`
	res := runInline(t, sc)
	final := res.Snapshots[len(res.Snapshots)-1]
	if len(final.Players) < 2 || len(final.Players[1].Exile) != 3 {
		t.Fatalf("precondition: p1 exile = %+v, want the three cards the Survivor exiled", final.Players)
	}
}
