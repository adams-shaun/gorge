package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestBuildRowCarriesHuntZeroUses: buildRow propagates the hunt row flag, so a
// hunt that records uses=0 for every owed key produces owed_uses=0 in the plan
// rather than the one-use-per-key fallback used when the hunt has no row.
func TestBuildRowCarriesHuntZeroUses(t *testing.T) {
	file := filepath.Join(t.TempDir(), "keys.tsv")
	body := "ctx\tcard\tkey\tuses\tcompared\tagreed\tstatus\n" +
		"4seat\tAlpha\tapi:a\t0\t0\t0\topen\n" +
		"4seat\tAlpha\tapi:b\t0\t0\t0\topen\n"
	if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	h, err := readHunt(file)
	if err != nil {
		t.Fatal(err)
	}

	in := inputs{
		reg:       cards.NewRegistry(),
		supported: map[string]bool{},
		lirNow:    map[string]string{"Alpha": "0123456789abcdef"},
		ev:        &evidence{lists: map[string][]string{"Alpha": {"a"}}},
		sc:        map[string]sidecarRow{"Alpha": {Card: "Alpha", Unused: "api:a,api:b"}},
		hunt:      h,
	}
	r := buildRow("Alpha", in)
	if r.DClaim != "supported" {
		t.Fatalf("precondition: Alpha d_claim = %q, want supported", r.DClaim)
	}
	if r.Missing != "api:a,api:b" {
		t.Fatalf("precondition: Alpha missing = %q, want api:a,api:b", r.Missing)
	}

	var b bytes.Buffer
	if err := writeNeed(&b, []row{r}); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d need lines, want header + Alpha:\n%s", len(lines), b.String())
	}
	if got := strings.Split(lines[1], "\t")[2]; got != "0" {
		t.Errorf("owed_uses = %q, want the hunt's 0, not the key count", got)
	}
}
