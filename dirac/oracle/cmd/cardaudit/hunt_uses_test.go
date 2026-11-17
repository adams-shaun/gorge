package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHuntOwedUses: owed_uses sums the hunt's per-key uses for the missing
// keys; a missing key the hunt does not name owes one use.
func TestHuntOwedUses(t *testing.T) {
	file := filepath.Join(t.TempDir(), "keys.tsv")
	body := "ctx\tcard\tkey\tuses\tcompared\tagreed\tstatus\n" +
		"4seat\tAlpha\tapi:a\t3\t1\t1\tboth\n" +
		"4seat\tAlpha\tapi:b\t2\t1\t1\tboth\n" +
		"4seat\tAlpha\tapi:c\t0\t0\t0\topen\n"
	if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	h, err := readHunt(file)
	if err != nil {
		t.Fatal(err)
	}
	if !h.fourSeat["Alpha"] {
		t.Error("Alpha is not marked 4-seat from its hunt ctx")
	}
	uses, ok := h.owedUses("Alpha", "api:a,api:b,api:d")
	if !ok {
		t.Fatal("owedUses found no hunt row for Alpha")
	}
	// 3 + 2 for the two named keys, plus one for api:d, which the hunt omits.
	if uses != 6 {
		t.Errorf("owedUses = %d, want 6", uses)
	}

	var b bytes.Buffer
	if err := writeNeed(&b, []row{{Card: "Alpha", DClaim: "supported", Missing: "api:a,api:b", OwedUses: uses}}); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d need lines, want header + Alpha:\n%s", len(lines), b.String())
	}
	if got := strings.Split(lines[1], "\t")[2]; got != "6" {
		t.Errorf("owed_uses column = %q, want 6", got)
	}
}
