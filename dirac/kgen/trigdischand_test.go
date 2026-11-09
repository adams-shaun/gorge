package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cardLIRChunks splits a dumpLIR output into per-card chunks keyed by card
// name: a "CARD <name> token=..." line starts one chunk, everything up to the
// next CARD line belongs to it (the header line itself is dropped, so chunks
// of two cards with the same ability body compare equal).
func cardLIRChunks(lir string) map[string]string {
	chunks := map[string]string{}
	name := ""
	var sb strings.Builder
	flush := func() {
		if name != "" {
			chunks[name] = sb.String()
		}
	}
	for _, line := range strings.Split(lir, "\n") {
		if rest, ok := strings.CutPrefix(line, "CARD "); ok {
			flush()
			name = rest
			if i := strings.Index(rest, " token="); i > 0 {
				name = rest[:i]
			}
			sb.Reset()
			continue
		}
		if name != "" {
			sb.WriteString(line)
			sb.WriteByte('\n')
		}
	}
	flush()
	return chunks
}

// TestTrigOwnDiscardHand: a printed trigger's own "you may discard your hand.
// If you do, draw ..." cost (Cost$ Discard<N/Hand> on the trigger's AB$) is
// the trigger's own cost, so it reads the unless-cost selection methods
// (gorge's triggered-cost window pays a Hand discard with the whole hand
// whatever the count; CR 118.3, 603.4). Narset, Jeskai Waymaster (TDM) and
// Byway Barterer (BLB) write Discard<1/Hand>, Vaultguard Trooper (EOE) writes
// Discard<0/Hand>, which reads as Discard<1/Hand>. An activated
// discard-your-hand (Reverberating Summons, TDM) is the activated-cost form and
// lowers to the DiscardHand cost table byte (TestDiscardHandActCostLowers).
func TestTrigOwnDiscardHand(t *testing.T) {
	carriers := []string{"Narset, Jeskai Waymaster", "Vaultguard Trooper", "Byway Barterer"}
	lir, ref := lowerCarriers(t, append(carriers, "Reverberating Summons"))
	chunks := cardLIRChunks(lir)
	want := "     0 IfPaid obj=You A=1 CS=DiscardHand skip=1 // covers the Sub chain"
	for _, n := range carriers {
		if rs := ref[n]; len(rs) > 0 {
			t.Errorf("%s: refused: %v", n, rs)
		}
		block, ok := chunks[n]
		if !ok {
			t.Errorf("%s: no lowered IR", n)
			continue
		}
		if !strings.Contains(block, want) {
			t.Errorf("%s: trigger program does not start with a DiscardHand IfPaid:\n%s", n, block)
		}
	}
	if rs := ref["Reverberating Summons"]; len(rs) > 0 {
		t.Errorf("Reverberating Summons: the activated discard-your-hand cost refused: %v", rs)
	}

	// Discard<0/Hand> lowers identically to Discard<1/Hand>: a probe with
	// Vaultguard Trooper's trigger shape and the explicit 1 must produce the
	// same program chunk.
	dir := t.TempDir()
	sub := filepath.Join(dir, "cardsfolder", "k")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "Name:Ktrigdischand Probe\nManaCost:1\nTypes:Artifact\n" +
		"T:Mode$ Phase | Phase$ End of Turn | TriggerZones$ Battlefield | ValidPlayer$ You | IsPresent$ Creature.YouCtrl+tapped | PresentCompare$ GE2 | Execute$ ProbeDraw | TriggerDescription$ Probe end-step rummage.\n" +
		"SVar:ProbeDraw:AB$ Draw | Cost$ Discard<1/Hand> | NumCards$ 2\n" +
		"Oracle:Probe end-step rummage.\n"
	if err := os.WriteFile(filepath.Join(sub, "ktrigdischand_probe.txt"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	list := filepath.Join(dir, "cards.txt")
	if err := os.WriteFile(list, []byte("Ktrigdischand Probe\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, dir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	if len(lw.cards) != 1 {
		t.Fatalf("lowered %d probe cards, want 1", len(lw.cards))
	}
	var sb strings.Builder
	dumpLIR(&sb, lw.cards[0])
	probe := cardLIRChunks(sb.String())["Ktrigdischand Probe"]
	if probe == "" {
		t.Fatalf("probe chunk missing:\n%s", sb.String())
	}
	vg, ok := chunks["Vaultguard Trooper"]
	if !ok {
		t.Fatalf("Vaultguard Trooper chunk missing:\n%s", lir)
	}
	if strings.TrimSpace(probe) != strings.TrimSpace(vg) {
		t.Errorf("Discard<0/Hand> (Vaultguard) and Discard<1/Hand> (probe) differ:\nprobe:\n%s\nvaultguard:\n%s", probe, vg)
	}
}
