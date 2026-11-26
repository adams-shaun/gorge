package oracle

import "testing"

const svogthosRepro = "svogthos_green_only_from_candleguide_filter_search_unmodelled"

// CR 602.2b, 605.3a: Svogthos, the Restless Tomb's {3}{B}{G} animate is payable
// when the only green source is Crossroads Candleguide's "{2}: Add one mana of
// any color". gorge's potential-play prefix search (potentialPrefixScript, 160
// nodes) tries the colour ask's B answer before its G one, both being announced
// pips, and leaves the activation census_incomplete (found=false limited=true)
// on a Candleguide, three Islands, four Swamps and the land, so gorge offers
// none. The kernel's filter search now bounds a path by the same potentialScriptNodes
// budget instead of sixteen steps, so on the pinned game it spends the budget
// the way gorge does and also offers none: the recorded game replays clean.
// The card is therefore no longer refused (dirac/support/unrefused.tsv); this
// test pins the unrefusal's proof, the game replaying without a divergence.
func TestMBCSvogthosReproReplaysClean(t *testing.T) {
	res := replayRawRepro(t, svogthosRepro)
	if res.Div != nil {
		t.Fatalf("%s's recorded game diverges at %d: %s", svogthosRepro, res.Div.At, res.Div)
	}
	if res.StallUnmodelled {
		t.Fatalf("%s's recorded game ends at an unmodelled stall: %q", svogthosRepro, res.Stall)
	}
}
