package oracle

import (
	"path/filepath"
	"testing"
)

// CR 708.4: a face-down spell is colourless. Sunscape Familiar's "blue spells
// you cast cost {1} less" does not reach Belltoll Dragon cast face down (gorge
// priced it from the card in hand, so the seat floated one mana more than the
// kernel), and Ramos, Dragon Engine's "a +1/+1 counter for each of that spell's
// colors" puts none (the kernel read the card's blue). Recorded by the kfuzz
// evidence sweep over dr-mbc-aabb, gorge leading, on the autopay surface.
func TestFaceDownSpellIsColourlessLockstep(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro(filepath.Join("testdata", "repro", "face_down_spell_is_colourless_for_cost_and_cast_trigger.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Surface != "autopay" {
		t.Fatalf("recorded on %q, want autopay", r.Surface)
	}
	stallCheck(t, r, replayReproValue(t, synthReg(t), globSet(t, r.Pool, r), r), false)
}
