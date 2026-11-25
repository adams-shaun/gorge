package kgen

import (
	"strings"
	"testing"
)

// definedRefsSlice1Carriers lower through three Defined$ referents gorge
// resolves: Regenerate over Defined$ Valid <filter> (every battlefield
// permanent the filter admits gets a shield), the unless payer
// TriggeredSourceSAController on a BecomesTarget watcher (gorge's one resolver
// for it and TriggeredSourceController: the targeting spell's controller), and
// a hand move over DefinedPlayer$ Player (every player, controller first, each
// picking from their own hand).
var definedRefsSlice1Carriers = []string{
	"Wrap in Vigor", "Loxodon Hierarch", "Golgari Charm", "Wail of the Nim",
	"Frost Titan", "Reality Smasher", "Diffusion Sliver", "Amulet of Safekeeping", "Unsettled Mariner",
	"Mind Swords", "Sadistic Augermage",
}

// definedRefsSlice1Refused stay refused by a rider the each-player walk does
// not lower: Show and Tell's Aura entry election, Kynaios and Tiro's
// remembered moves.
var definedRefsSlice1Refused = map[string]string{
	"Show and Tell":               "AuraEntry",
	"Kynaios and Tiro of Meletis": "Remember",
}

func TestDefinedRefsSlice1(t *testing.T) {
	names := append([]string{}, definedRefsSlice1Carriers...)
	for n := range definedRefsSlice1Refused {
		names = append(names, n)
	}
	got := buildNamed(t, names)
	for _, n := range definedRefsSlice1Carriers {
		rs, ok := got[n]
		if !ok {
			t.Errorf("%s: not lowered (corpus missing?)", n)
			continue
		}
		if len(rs) != 0 {
			t.Errorf("%s: refused: %q", n, rs)
		}
	}
	for n, frag := range definedRefsSlice1Refused {
		if j := strings.Join(got[n], "; "); !strings.Contains(j, frag) {
			t.Errorf("%s: refusals %q, want one naming %q", n, j, frag)
		}
	}
}
