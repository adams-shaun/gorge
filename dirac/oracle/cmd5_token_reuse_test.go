package oracle

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestCmd5TokenIdsReusedNoStall: the TestStdSetReprosLockstep entry
// cmd5_token_ids_reused_after_cease counts a kernel capacity stall as a pass,
// so this replays it and requires the game to go past the script's last
// action (where the kernel's object table filled) without a divergence or a
// kernel stall: the 25th token takes a ceased token's id on both sides.
func TestCmd5TokenIdsReusedNoStall(t *testing.T) {
	t.Parallel()
	const name = "cmd5_token_ids_reused_after_cease"
	r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	res := replayStdReproResult(t, synthReg(t), "custom", name, true) // recorded on the full surface
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) {
		t.Fatalf("the kernel stalled (%s) after %d decisions; the script has %d", res.Stall, res.Decisions, len(r.Actions))
	}
	if res.Decisions <= len(r.Actions) {
		t.Fatalf("the game ended at decision %d, not past the script's %d (the object table filled at its end)", res.Decisions, len(r.Actions))
	}
}
