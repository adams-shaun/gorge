package main

// The L9c policynet wiring's pins: the -checkpoint flag's front-door
// validation (missing / unnecessary / drifted checkpoints all fail BEFORE
// any game starts, with the checkpoint loader's own named error), and the
// bench support contract itself — a real `-a policynet -checkpoint ck.bin
// -b bot -pairs <pair> -games 1` run plays through the matrix path and
// exits 0.

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/seat"

	"github.com/adams-shaun/gorge/internal/policynet"
)

// writeZeroCheckpoint writes a genuinely zero-weight checkpoint (every
// option scores OutB = 0, so the policynet seat passes priority and declares
// nothing: the game always terminates against the default bot) to a temp
// file through the real writer.
func writeZeroCheckpoint(t *testing.T) string {
	t.Helper()
	m := policynet.NewModel(policynet.TableRows, 8, 4, rand.New(rand.NewPCG(2, 3)))
	for _, blk := range [][]float32{m.Table, m.StateW, m.StateB, m.HidW, m.HidB, m.OutW} {
		for i := range blk {
			blk[i] = 0
		}
	}
	m.OutB = 0
	path := filepath.Join(t.TempDir(), "ck.bin")
	if err := m.SaveCheckpoint(path); err != nil {
		t.Fatalf("SaveCheckpoint: %v", err)
	}
	return path
}

// TestPolicynetCheckpointFlagValidation pins the front-door validation:
//   - a policynet side without -checkpoint is refused;
//   - a checkpoint with no policynet side is refused;
//   - a checkpoint that fails the loader (drifted encoder hash) is refused
//     with the loader's named error.
func TestPolicynetCheckpointFlagValidation(t *testing.T) {
	// policynet side, no checkpoint.
	code := mainExit("policynet", "bot", 1, 0, 2, 0, "mono-red-goblins:mono-blue-tempo", "constructed", "text", 0,
		200, 20000, ".cards", "", false, false, "", 0, 0, "", "", "", "", "")
	if code == 0 {
		t.Error("policynet without -checkpoint must exit non-zero")
	}

	// checkpoint, but no policynet side.
	code = mainExit("bot", "bot", 1, 0, 2, 0, "mono-red-goblins:mono-blue-tempo", "constructed", "text", 0,
		200, 20000, ".cards", "", false, false, "", 0, 0, "", "", "", "", writeZeroCheckpoint(t))
	if code == 0 {
		t.Error("-checkpoint without a policynet side must exit non-zero")
	}

	// A drifted checkpoint: the loader's encoder-hash error reaches the
	// front door. Byte 8 is the hash field's first byte (checkpoint.go's
	// header layout), so a flip is guaranteed to mismatch EncoderHash().
	ck := writeZeroCheckpoint(t)
	raw, err := os.ReadFile(ck)
	if err != nil {
		t.Fatalf("read checkpoint: %v", err)
	}
	raw[8] ^= 0x01
	bad := filepath.Join(t.TempDir(), "bad.bin")
	if err := os.WriteFile(bad, raw, 0o644); err != nil {
		t.Fatalf("write drifted checkpoint: %v", err)
	}
	code = mainExit("policynet", "bot", 1, 0, 2, 0, "mono-red-goblins:mono-blue-tempo", "constructed", "text", 0,
		200, 20000, ".cards", "", false, false, "", 0, 0, "", "", "", "", bad)
	if code == 0 {
		t.Error("a drifted checkpoint must exit non-zero")
	}
}

// TestPolicynetKindsFlagValidation pins -policynet-kinds' front door: an
// unknown or repeated kind is refused, the flag is refused without a
// policynet side, and the validated default is attackers only (the default
// seat, byte for byte).
func TestPolicynetKindsFlagValidation(t *testing.T) {
	defer func(arg string, given bool, kinds []decision.Kind) {
		policynetKindsArg, policynetKindsGiven, policynetKinds = arg, given, kinds
	}(policynetKindsArg, policynetKindsGiven, policynetKinds)

	if !slices.Equal(policynetKinds, []decision.Kind{decision.KAttackers}) || policynetKindsArg != "attackers" {
		t.Fatalf("default -policynet-kinds = %q / %v, want attackers only", policynetKindsArg, policynetKinds)
	}
	ck := writeZeroCheckpoint(t)
	for _, bad := range []string{"blocker", "attackers,attackers", "attackers,", ""} {
		policynetKindsArg, policynetKindsGiven = bad, true
		code := mainExit("policynet", "bot", 1, 0, 2, 0, "mono-red-goblins:mono-blue-tempo", "constructed", "text", 0,
			200, 20000, ".cards", "", false, false, "", 0, 0, "", "", "", "", ck)
		if code == 0 {
			t.Errorf("-policynet-kinds %q must exit non-zero", bad)
		}
	}
	policynetKindsArg, policynetKindsGiven = "attackers,priority", true
	code := mainExit("bot", "bot", 1, 0, 2, 0, "mono-red-goblins:mono-blue-tempo", "constructed", "text", 0,
		200, 20000, ".cards", "", false, false, "", 0, 0, "", "", "", "", "")
	if code == 0 {
		t.Error("-policynet-kinds without a policynet side must exit non-zero")
	}
	kinds, err := seat.ParsePolicyNetKinds("attackers, priority")
	if err != nil || !slices.Equal(kinds, []decision.Kind{decision.KAttackers, decision.KPriority}) {
		t.Errorf("ParsePolicyNetKinds(attackers, priority) = %v, %v", kinds, err)
	}
}

// TestPolicynetPlaysPairMatrix is the brief's bench-support contract, end to
// end: `-a policynet -checkpoint ck.bin -b bot -pairs <pair> -games 1 -seed S`
// plays through the same runMatrixTraced path every other policy uses and
// exits 0. With the zero checkpoint the policynet seat passes every priority
// and declares no attack, so the game terminates (the default bot kills an
// idle opponent) and the run is clean.
func TestPolicynetPlaysPairMatrix(t *testing.T) {
	dir := corpusDirOrSkip(t)
	code := mainExit("policynet", "bot", 1, 42, 2, 0, "mono-red-goblins:mono-blue-tempo", "constructed", "text", 0,
		200, 20000, dir, "", false, false, "", 0, 0, "", "", "", "", writeZeroCheckpoint(t))
	if code != 0 {
		t.Fatalf("policynet pair run exited %d (mainExit prints its report to os.Stdout; the exit code is the contract)", code)
	}
}
