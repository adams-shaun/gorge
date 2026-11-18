package oracle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestCmd5ArmyOfTheDamnedPastOldObjectBound: the lockstep table entry
// cmd5_army_of_the_damned_past_224_objects counts a kernel capacity stall as a
// pass, so this replays it and requires the game to run past the script's end
// (where the object table used to fill at 224 ids) with no divergence and no
// kernel stall.
func TestCmd5ArmyOfTheDamnedPastOldObjectBound(t *testing.T) {
	t.Parallel()
	const name = "cmd5_army_of_the_damned_past_224_objects"
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
		t.Fatalf("the game ended at decision %d, not past the script's %d", res.Decisions, len(r.Actions))
	}
}

// TestReproFormat1MapsTargetBytes: a format-1 repro (stack abilities at
// 0xE0.., players at 0xF0/0xF1) loads with the current Refs, and a format-2
// one (the two-seat wire bytes: stack abilities at 0xEC.., players at
// 0xFC/0xFD) loads as the same Refs. The files are written as disk bytes, one
// byte a field, as a recording holds them.
func TestReproFormat1MapsTargetBytes(t *testing.T) {
	word := func(k CanonKind, src, tgt, mode uint8) uint64 {
		return uint64(k)<<24 | uint64(src)<<16 | uint64(tgt)<<8 | uint64(mode)
	}
	// the kinds are arbitrary: only source and target bytes are Refs
	old := []uint64{
		word(ActActivate, 0x05, 0xF0, 7),    // player 0
		word(ActAttack, 0x17, 0xF1, 0),      // player 1
		word(ActActivate, 0xE0, 0xFF, 3),    // stack ability 0 as the source
		word(ActActivate, 0x02, 0xE3, 0),    // stack ability 3 as the target
		word(ActActivate, 0xEF, 0xDF, 1),    // stack ability 15; the last old instance, 223
		word(ActActivate, 0xFF, 0xFF, 0xE5), // the mode byte is not a Ref
	}
	cur := []uint64{
		word(ActActivate, 0x05, 0xFC, 7),
		word(ActAttack, 0x17, 0xFD, 0),
		word(ActActivate, 0xEC, 0xFF, 3),
		word(ActActivate, 0x02, 0xEF, 0),
		word(ActActivate, 0xFB, 0xDF, 1),
		word(ActActivate, 0xFF, 0xFF, 0xE5),
	}
	want := []Canon{
		Pack(ActActivate, 0x05, RefPlayer, 7),
		Pack(ActAttack, 0x17, RefPlayer+1, 0),
		Pack(ActActivate, RefStack, RefNone, 3),
		Pack(ActActivate, 0x02, RefStack+3, 0),
		Pack(ActActivate, RefStack+15, 0xDF, 1),
		Pack(ActActivate, RefNone, RefNone, 0xE5),
	}
	if RefPlayer-RefStack != 16 {
		t.Fatalf("RefStack %#x RefPlayer %#x: %d stack codes, a format-1 recording names 16", RefStack, RefPlayer, RefPlayer-RefStack)
	}
	dir := t.TempDir()
	for _, c := range []struct {
		format string
		words  []uint64
	}{
		{"koracle-repro-1", old},
		{"koracle-repro-2", cur},
	} {
		b, err := json.Marshal(map[string]any{"format": c.format, "decks": [2]string{"a", "b"}, "actions": c.words})
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, c.format+".json")
		if err := os.WriteFile(path, b, 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := ReadRepro(path)
		if err != nil {
			t.Fatalf("%s: %v", c.format, err)
		}
		if got.Format != reproFormat {
			t.Errorf("%s loaded as %q, want %q", c.format, got.Format, reproFormat)
		}
		for i := range want {
			if got.Actions[i] != want[i] {
				t.Errorf("%s action %d: got %#x, want %#x", c.format, i, uint64(got.Actions[i]), uint64(want[i]))
			}
		}
		// and back: the current Refs write the two-seat bytes
		out, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		var w struct {
			Actions []uint64 `json:"actions"`
			Packing string   `json:"packing"`
		}
		if err := json.Unmarshal(out, &w); err != nil {
			t.Fatal(err)
		}
		if w.Packing != "" || !slices.Equal(w.Actions, cur) {
			t.Errorf("%s written back as %#x (packing %q), want %#x", c.format, w.Actions, w.Packing, cur)
		}
	}
}

// TestKernelGameFormat1MapsTargetBytes: ReadKernelGame maps a format-1
// game's actions the same way and drops its projection hashes, which encode
// the old bytes.
func TestKernelGameFormat1MapsTargetBytes(t *testing.T) {
	g := &KernelGame{Format: "koracle-game-1", Decks: [2]string{"a", "b"},
		Actions: []Canon{Pack(ActAttack, 0x17, 0xF1, 0), Pack(ActActivate, 0xE2, RefNone, 0)},
		Hashes:  []uint64{1, 2, 3}}
	path := filepath.Join(t.TempDir(), "g.json")
	b, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadKernelGame(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Format != gameFormat || got.Hashes != nil {
		t.Errorf("format %q hashes %v, want %q and none", got.Format, got.Hashes, gameFormat)
	}
	if got.Actions[0] != Pack(ActAttack, 0x17, RefPlayer+1, 0) || got.Actions[1] != Pack(ActActivate, RefStack+2, RefNone, 0) {
		t.Errorf("actions %#x, not mapped", got.Actions)
	}
}
