package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
)

// readRepoFile reads a path relative to the repository root. Tests run in the
// package directory, which is two levels down from it.
func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func sampleCoverage() coverageData {
	return coverageData{
		Corpus:    "95f04e8a04c8925fa97cb226fc3341cabcc90a53",
		Cards:     100,
		Supported: 87,
		Tokens:    9,
		ByType: byCountThenKey([]cards.CoverageGroup{
			{Key: "Instant", Cards: 40, Supported: 39},
			{Key: "Creature", Cards: 60, Supported: 48},
		}),
		ByColour: byCountThenKey([]cards.CoverageGroup{{Key: "Blue", Cards: 100, Supported: 87}}),
		ByMana:   []cards.CoverageGroup{{Key: "2", Cards: 100, Supported: 87}},
		Missing:  []cards.MissingPrimitive{{Name: "kw:Crew", Cards: 12}},
	}
}

// TestRenderIsDeterministic pins the property the refresh job depends on: the
// same data renders byte-identically every time, with no clock or map order
// anywhere in the output.
func TestRenderIsDeterministic(t *testing.T) {
	d := sampleCoverage()
	doc, sum := renderCoverageDoc(d), renderSummary(d)
	for i := 0; i < 5; i++ {
		if got := renderCoverageDoc(d); got != doc {
			t.Fatal("renderCoverageDoc is not deterministic")
		}
		if got := renderSummary(d); got != sum {
			t.Fatal("renderSummary is not deterministic")
		}
	}
	for _, want := range []string{
		"- Cards in the corpus: **100**",
		"- Fully playable: **87 (87.0%)**",
		"- Corpus pin: `Card-Forge/forge@95f04e8a04c8925fa97cb226fc3341cabcc90a53`",
		"| Creature | 60 | 48 | 80.0% |",
		"| `kw:Crew` | 12 |",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("doc is missing %q", want)
		}
	}
	// Biggest bucket first on the unordered axes.
	if i, j := strings.Index(doc, "| Creature |"), strings.Index(doc, "| Instant |"); i > j {
		t.Error("by-type rows are not count-ordered")
	}
}

// TestByCountThenKeyDoesNotAliasInput guards the renderer against reordering
// the caller's slice in place, which would make the by-mana table (which must
// stay key-ordered) depend on whether a by-count table was rendered first.
func TestByCountThenKeyDoesNotAliasInput(t *testing.T) {
	in := []cards.CoverageGroup{{Key: "a", Cards: 1}, {Key: "b", Cards: 9}}
	out := byCountThenKey(in)
	if in[0].Key != "a" || in[1].Key != "b" {
		t.Errorf("input was reordered: %+v", in)
	}
	if out[0].Key != "b" {
		t.Errorf("output is not count-ordered: %+v", out)
	}
}

func TestSpliceSummary(t *testing.T) {
	readme := "# gorge\n\n" + readmeBeginMarker + "\nstale\n" + readmeEndMarker + "\n\n## Design\n"
	section := readmeBeginMarker + "\nfresh\n" + readmeEndMarker
	got, err := spliceSummary(readme, section)
	if err != nil {
		t.Fatal(err)
	}
	want := "# gorge\n\n" + section + "\n\n## Design\n"
	if got != want {
		t.Fatalf("spliceSummary =\n%q\nwant\n%q", got, want)
	}
	// Idempotent: splicing the same section again changes nothing.
	again, err := spliceSummary(got, section)
	if err != nil {
		t.Fatal(err)
	}
	if again != got {
		t.Error("spliceSummary is not idempotent")
	}
	for _, bad := range []string{
		"# gorge\n",
		"# gorge\n" + readmeBeginMarker + "\n",
		"# gorge\n" + readmeEndMarker + "\n" + readmeBeginMarker + "\n",
	} {
		if _, err := spliceSummary(bad, section); err == nil {
			t.Errorf("spliceSummary(%q) succeeded, want an error", bad)
		}
	}
}

// TestRepoReadmeCarriesMarkers fails if someone removes the markers from the
// committed README: the refresh job would then error out on every run instead
// of quietly appending its block somewhere wrong.
func TestRepoReadmeCarriesMarkers(t *testing.T) {
	readme := readRepoFile(t, "README.md")
	if !strings.Contains(readme, readmeBeginMarker) || !strings.Contains(readme, readmeEndMarker) {
		t.Fatalf("README.md is missing the %s / %s markers", readmeBeginMarker, readmeEndMarker)
	}
	if _, err := spliceSummary(readme, renderSummary(sampleCoverage())); err != nil {
		t.Fatal(err)
	}
}
