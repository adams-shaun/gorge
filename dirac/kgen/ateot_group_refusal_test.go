package kgen

import (
	"strings"
	"testing"
)

// TestTokenAtEOTGroupRefusal: gorge registers one delayed trigger naming every
// token of one resolution for an AtEOT$ rider (scheduleAtEOTGroup, CR
// 603.7a/b), while the kernel joins only the sacrifice and end-step exile
// riders (sacAtEndJoin, exileAtEndJoin), so a Destroy or ExileCombat mint that
// can make several tokens refuses. A one-token mint, the sacrifice riders and
// Feral Lightning's three exiled Elementals still lower.
func TestTokenAtEOTGroupRefusal(t *testing.T) {
	three := lValue{Kind: vConst, N: 3}
	one := lValue{Kind: vConst, N: 1}
	you := lRef{Kind: rYou}
	for _, tc := range []struct {
		name   string
		v      string
		n      lValue
		owner  lRef
		andYou bool
		refuse bool
	}{
		{"Destroy over three", "Destroy", three, you, false, true},
		{"ExileCombat over three", "ExileCombat", three, you, false, true},
		{"each player's destroyed token", "Destroy", one, lRef{Kind: rEachPlayer}, false, true},
		{"AndYou second mint", "Destroy", one, you, true, true},
		{"one destroyed token", "Destroy", one, you, false, false},
		{"Feral Lightning: three exiled", "Exile", three, you, false, false},
		{"sacrificed three", "Sacrifice", three, you, false, false},
	} {
		err := tokenAtEOTGroupRefusal(tc.v, tc.n, tc.owner, tc.andYou)
		switch {
		case tc.refuse && err == nil:
			t.Errorf("%s lowered; want a refusal", tc.name)
		case tc.refuse && !strings.Contains(err.Error(), "grouped trigger"):
			t.Errorf("%s refused %q, want the grouped-trigger refusal", tc.name, err)
		case !tc.refuse && err != nil:
			t.Errorf("%s refused %q; want it to lower", tc.name, err)
		}
	}
	if !atEOTGroupedAct(1) || atEOTGroupedAct(0) {
		t.Errorf("atEOTGroupedAct: exile (1) %v, sacrifice (0) %v; want true, false", atEOTGroupedAct(1), atEOTGroupedAct(0))
	}
}
