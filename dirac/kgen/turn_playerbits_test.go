package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestTurnFactsStaticLowering: the two per-player this-turn facts lower only
// from the exact corpus shapes. Edgar, King of Figaro's FlipCoinMod static
// becomes kop.StatFlipCoinMod; Elvish Refueler's Activations static becomes
// kop.StatActivationLimit and reads Count$ThisTurnActivated_
// Activated.Exhaust+YouCtrl. Any other shape refuses by name.
func TestTurnFactsStaticLowering(t *testing.T) {
	edgar := lowerCorpusCard(t, "Edgar, King of Figaro")
	for _, rf := range edgar.Refusals {
		t.Errorf("Edgar refused: %s", rf.Reason)
	}
	var flip bool
	for _, st := range edgar.Statics {
		flip = flip || st.Kind == "FlipCoinMod"
	}
	if !flip {
		t.Fatalf("Edgar's statics %+v do not carry FlipCoinMod", edgar.Statics)
	}

	refueler := lowerCorpusCard(t, "Elvish Refueler")
	for _, rf := range refueler.Refusals {
		t.Errorf("Elvish Refueler refused: %s", rf.Reason)
	}
	var act bool
	for _, st := range refueler.Statics {
		if st.Kind != "ActivationLimit" {
			continue
		}
		act = true
		if st.Cond.Kind != cAnd || st.Cond.Sub[0].A.Kind != vThisTurnActExhaust {
			t.Fatalf("Activations condition %+v does not read ThisTurnActExhaust", st.Cond)
		}
	}
	if !act {
		t.Fatalf("Elvish Refueler's statics %+v do not carry ActivationLimit", refueler.Statics)
	}

	// The CheckSVar body lowers to the exhaust-activation fact; another
	// ThisTurnActivated spec still refuses by name.
	lc := &lowerCtx{face: &cards.Face{SVars: map[string]string{}}, known: func(string) bool { return false }}
	v, err := lc.countValue("ThisTurnActivated_Activated.Exhaust+YouCtrl", 0)
	if err != nil || v.Kind != vThisTurnActExhaust {
		t.Fatalf("ThisTurnActivated exhaust = %+v err %v, want ThisTurnActExhaust", v, err)
	}
	if _, err := lc.countValue("ThisTurnActivated_Activated.Equip+YouCtrl", 0); err == nil || !strings.Contains(err.Error(), "ThisTurnActivated") {
		t.Fatalf("another ThisTurnActivated spec: want a named refusal, got %v", err)
	}

	// Another FlipCoinMod shape refuses by name.
	for _, tc := range []struct{ from, to string }{
		{"Result$ True", "Result$ False"},
		{"ValidPlayer$ You", "ValidPlayer$ Opponent"},
		{"SVarCompare$ EQ0", "SVarCompare$ GE1"},
	} {
		src := "Name:Turn Facts Probe\nTypes:Creature Human\nPT:1/1\n" +
			"S:Mode$ FlipCoinMod | ValidPlayer$ You | CheckSVar$ Count$YouFlipThisTurn | SVarCompare$ EQ0 | Result$ True\n"
		r := lowerScript(t, "Turn Facts Probe", strings.Replace(src, tc.from, tc.to, 1))
		if len(r) == 0 || !strings.Contains(strings.Join(r, "\n"), "FlipCoinMod") {
			t.Errorf("FlipCoinMod %s: refusals %v, want one naming FlipCoinMod", tc.to, r)
		}
	}
}
