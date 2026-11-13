package oracle

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestCmd5MyrBattlesphereReprosLockstep: a hunt game (cmd5 pool) where Myr
// Battlesphere's attack trigger offers "tap X untapped Myr" with one untapped
// Myr left. gorge's one KChoose (Max = candidates) is complete at Max picks and
// asks no Done, so the kernel must close the cost when its last candidate is
// tapped; it used to ask one Done too many, diverging at decision 732.
func TestCmd5MyrBattlesphereReprosLockstep(t *testing.T) {
	t.Parallel()
	const name = "cmd5_myr_battlesphere_taps_the_last_candidate"
	r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Divergence == nil || r.Divergence.At != 732 || r.Divergence.Field != "Cards[69].Power" {
		t.Fatalf("the recording is not the tap-election divergence at 732: %+v", r.Divergence)
	}
	has := false
	for _, deck := range r.Cards {
		for _, c := range deck {
			has = has || c == "Myr Battlesphere"
		}
	}
	if !has {
		t.Fatal("the repro's decks do not hold Myr Battlesphere")
	}
	res := replayStdReproResult(t, synthReg(t), "custom", name, true)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) {
		t.Fatalf("the kernel stalled: %q", res.Stall)
	}
	// the divergence used to be at decision 732: the game must run well past it
	if res.Decisions <= 732 {
		t.Fatalf("the game ended at decision %d, before the tap election at 732", res.Decisions)
	}
}
