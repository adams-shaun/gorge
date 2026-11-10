package oracle

import (
	"encoding/json"
	"os"
	"path/filepath"
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
// one loads as written.
func TestReproFormat1MapsTargetBytes(t *testing.T) {
	// the kinds are arbitrary: only source and target bytes are Refs
	old := []Canon{
		Pack(ActActivate, 0x05, 0xF0, 7),    // player 0
		Pack(ActAttack, 0x17, 0xF1, 0),      // player 1
		Pack(ActActivate, 0xE0, RefNone, 3), // stack ability 0 as the source
		Pack(ActActivate, 0x02, 0xE3, 0),    // stack ability 3 as the target
		Pack(ActActivate, 0xEF, 0xDF, 1),    // stack ability 15; the last old instance, 223
		Pack(ActActivate, RefNone, RefNone, 0xE5),
	}
	want := []Canon{
		Pack(ActActivate, 0x05, 0xFC, 7),
		Pack(ActAttack, 0x17, 0xFD, 0),
		Pack(ActActivate, 0xEC, RefNone, 3),
		Pack(ActActivate, 0x02, 0xEF, 0),
		Pack(ActActivate, 0xFB, 0xDF, 1),
		Pack(ActActivate, RefNone, RefNone, 0xE5), // the mode byte is not a Ref
	}
	if RefStack != 0xEC || RefPlayer != 0xFC {
		t.Fatalf("RefStack %#x RefPlayer %#x, want 0xEC and 0xFC", RefStack, RefPlayer)
	}
	for i := range old[:len(old)-1] { // the last case is the unchanged mode byte
		if old[i] == want[i] {
			t.Fatalf("action %d is unchanged by the map; the case proves nothing", i)
		}
	}
	dir := t.TempDir()
	for _, c := range []struct {
		format string
		acts   []Canon
		want   []Canon
	}{
		{"koracle-repro-1", old, want},
		{"koracle-repro-2", want, want},
	} {
		r := &Repro{Format: c.format, Decks: [2]string{"a", "b"}, Actions: c.acts}
		path := filepath.Join(dir, c.format+".json")
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
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
		for i := range c.want {
			if got.Actions[i] != c.want[i] {
				t.Errorf("%s action %d: got %#x, want %#x", c.format, i, uint32(got.Actions[i]), uint32(c.want[i]))
			}
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
	if got.Actions[0] != Pack(ActAttack, 0x17, 0xFD, 0) || got.Actions[1] != Pack(ActActivate, 0xEE, RefNone, 0) {
		t.Errorf("actions %#x, not mapped", got.Actions)
	}
}
