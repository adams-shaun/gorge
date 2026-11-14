package oracle

import (
	"path/filepath"
	"testing"
)

// A land whose second mana ability costs life beside its plain {C} one
// (Ramunap Ruins: "{T}, Pay 1 life: Add {R}") paying a mid-resolution generic
// cost, a ward {2} payment in a manual window. gorge lists the land's
// abilities in script order and the option of a single-colour ability has no
// mana symbol, so the window's answer takes the first listed one, {C}, and
// pays no life (payexec.WindowAnswer; CR 605.3b, 118.3). The kernel's window
// (mana_window.go windowColOwed) answered the life ability's colour and left
// the payer one life lower. The recording was made by the kfuzz evidence
// sweep over dr-mbc-abba (pair 40, gorge leading) and replays clean.
func TestRamunapPayLifeManaLockstep(t *testing.T) {
	reg := synthReg(t)
	r, err := ReadRepro(filepath.Join("testdata", "repro", "ramunap_paylife_mana_pays_a_ward_with_the_colourless_ability.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Surface != "autopay" {
		t.Fatalf("recorded on %q, want autopay: the class is a surface question", r.Surface)
	}
	stallCheck(t, r, replayReproValue(t, reg, globSet(t, r.Pool, r), r), false)
}
