package main

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

// TestTSVReaderKeepsQuotedName guards the raw-tab reader. A card name that
// starts with a quote (Forge has `"Ach! Hans, Run!"`) is a literal name, the
// same way kgen's readCoverageRows reads the sidecar; encoding/csv would strip
// the quotes and every joined column for that card would silently miss.
func TestTSVReaderKeepsQuotedName(t *testing.T) {
	const name = `"Ach! Hans, Run!"`
	fsys := fstest.MapFS{
		"dirac/support/evidence-keys/a.tsv": {Data: []byte(
			"card\tlir16\tplays\tkeys\tunused\tbranches\tmissing_branches\n" +
				name + "\t0123456789abcdef\t3\t1/2\t\t0/0\tapi:Gap\n")},
	}

	sc, err := readSidecars(fsys)
	if err != nil {
		t.Fatal(err)
	}
	s, ok := sc[name]
	if !ok {
		t.Fatalf("readSidecars has no row keyed by the raw name %q: %v", name, sc)
	}
	if s.LIR != "0123456789abcdef" || s.MissingBranches != "api:Gap" {
		t.Fatalf("row for %q = %+v, want its sidecar fields intact", name, s)
	}

	// The same raw split backs the hunt's keys.tsv.
	hunt := filepath.Join(t.TempDir(), "keys.tsv")
	body := "ctx\tcard\tkey\tuses\tcompared\tagreed\tstatus\n" +
		"gorge\t" + name + "\tapi:Test\t2\t1\t1\tg\n"
	if err := os.WriteFile(hunt, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	h, err := readHunt(hunt)
	if err != nil {
		t.Fatal(err)
	}
	if got := h.g[name]; len(got) != 1 || got[0] != "api:Test" {
		t.Fatalf("readHunt g[%q] = %v, want [api:Test]", name, got)
	}
}
