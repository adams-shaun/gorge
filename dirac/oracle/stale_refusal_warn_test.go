package oracle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestStaleRefusalWarnsOutsideStrict covers staleVerdict, the decision a stale refusal ends
// on: a pair that still implicates the card is a pass with no warning; a stale refusal
// outside DIRAC_PIN_STRICT=1 passes with the bot's warning (and, under
// DIRAC_UNREFUSE_STALE=<dir>, writes the clean game and the stale.tsv line
// scripts/unrefuse-stale.sh acts on); under the pin it fails, as the register's assertion
// always has.
func TestStaleRefusalWarnsOutsideStrict(t *testing.T) {
	const card, pool = "Frostboil Snare", "std-trk"
	s := refusedSearch{card: card, pool: pool, played: 5, found: -1}
	staleMsg := fmt.Sprintf("stale: %d pairs of %s with %s dealt in and none implicates it. %s", s.played, pool, card, staleRemedy)
	warnBase := fmt.Sprintf("WARNING: %s no longer diverges (%d pairs of %s)", card, s.played, pool)
	// Precondition: the slug the stale record names is the card's, so the script and the
	// proof row agree on the file.
	if slug := cardset.SupportSlug(card); slug != "frostboil-snare" {
		t.Fatalf("SupportSlug(%q) = %q", card, slug)
	}

	t.Run("a diverging pair is a pass with no warning", func(t *testing.T) {
		div := s
		div.found = 7
		failed, warn := staleVerdict(div, staleMsg, warnBase, true, "")
		if failed != "" || warn != "" {
			t.Fatalf("staleVerdict(found 7) = %q, %q, want \"\", \"\"", failed, warn)
		}
	})

	t.Run("stale with the pin off warns and passes", func(t *testing.T) {
		failed, warn := staleVerdict(s, staleMsg, warnBase, false, "")
		if failed != "" {
			t.Fatalf("staleVerdict(passes) failed %q", failed)
		}
		if want := warnBase + ": scripts/regen-main.sh unrefuses it"; warn != want {
			t.Fatalf("warn = %q, want %q", warn, want)
		}
	})

	t.Run("stale fails under the pin", func(t *testing.T) {
		failed, warn := staleVerdict(s, staleMsg, warnBase, true, "")
		if failed != staleMsg || warn != "" {
			t.Fatalf("staleVerdict(strict) = %q, %q, want the stale message, \"\"", failed, warn)
		}
	})

	t.Run("stale with DIRAC_UNREFUSE_STALE writes the bot's record", func(t *testing.T) {
		dir := t.TempDir()
		// A fabricated clean game: writeStaleUnrefuse records it through NewRepro, whose
		// fields are what the test asserts, not the game itself.
		sc := s
		sc.clean = &cleanGame{
			res:  &Result{},
			m:    &Matchup{Names: [2]string{"deck-a", "deck-b"}},
			seed: 9,
			rc:   RunConfig{MaxTurns: 100},
		}
		failed, warn := staleVerdict(sc, staleMsg, warnBase, false, dir)
		if failed != "" {
			t.Fatalf("staleVerdict(staleDir) failed %q", failed)
		}
		if !strings.Contains(warn, "scripts/regen-main.sh unrefuses it") {
			t.Fatalf("warn = %q", warn)
		}
		b, err := os.ReadFile(filepath.Join(dir, cardset.SupportSlug(card)+".repro.json"))
		if err != nil {
			t.Fatal(err)
		}
		var r Repro
		if err := json.Unmarshal(b, &r); err != nil {
			t.Fatalf("%s: %v", b, err)
		}
		if r.Format != reproFormat {
			t.Fatalf("repro format %q, want %q", r.Format, reproFormat)
		}
		rows, err := os.ReadFile(filepath.Join(dir, "stale.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		if want := fmt.Sprintf("%s\t%s\t%d\t%s\n", card, cardset.SupportSlug(card), s.played, pool); string(rows) != want {
			t.Fatalf("stale.tsv = %q, want %q", rows, want)
		}
	})
}
