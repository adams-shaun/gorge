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
		// The file lands in testdata/repro, where TestReproGlobMatchesTable wants a pool and a
		// surface of every recording and the stall flag to say what the replay ends in.
		if r.Pool != "one-off" || r.Surface != "full" || r.Stall {
			t.Fatalf("repro pool %q surface %q stall %v, want one-off, full, false", r.Pool, r.Surface, r.Stall)
		}
		if _, err := ReadRepro(filepath.Join(dir, cardset.SupportSlug(card)+".repro.json")); err != nil {
			t.Fatal(err)
		}
		rows, err := os.ReadFile(filepath.Join(dir, "stale.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		if want := fmt.Sprintf("%s\t%s\t%d\t%s\n", card, cardset.SupportSlug(card), s.played, pool); string(rows) != want {
			t.Fatalf("stale.tsv = %q, want %q", rows, want)
		}
	})

	t.Run("a declared kernel stall marks the recorded game", func(t *testing.T) {
		c := cleanGame{res: &Result{Stall: KernelStallPrefix + "capacity"}, m: &Matchup{}, rc: RunConfig{}}
		if r := c.proofRepro(card); !r.Stall || r.Pool != "one-off" {
			t.Fatalf("proofRepro: stall %v pool %q, want true, one-off", r.Stall, r.Pool)
		}
	})

	t.Run("stale with no proven game warns that nothing was recorded", func(t *testing.T) {
		dir := t.TempDir()
		failed, warn := staleVerdict(s, staleMsg, warnBase, false, dir)
		if failed != "" {
			t.Fatalf("failed %q", failed)
		}
		if !strings.Contains(warn, "cannot unrefuse it") || strings.Contains(warn, "regen-main.sh unrefuses it") {
			t.Fatalf("warn = %q, want the no-proof warning", warn)
		}
		if es, err := os.ReadDir(dir); err != nil || len(es) != 0 {
			t.Fatalf("recorded %v (%v) with no proven game", es, err)
		}
	})
}

// TestProvenCleanReplaysTheRecordingOnItsOwnCards: provenClean keeps a game only when its
// recording (as the bot would write it) replays clean on its own cards' table, the replay
// TestUnrefusalNeedsProof runs. The game is a recorded unrefusal proof replayed from its
// script, so its result, matchup and seed are a real clean game's.
func TestProvenCleanReplaysTheRecordingOnItsOwnCards(t *testing.T) {
	const proof = "ancient_cornucopia_card_flags_after_its_cast"
	r, err := ReadRepro(filepath.Join("testdata", "repro", proof+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	res, m, err := replayOnOwnCards(t, r)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Clean() || len(res.Actions) == 0 {
		t.Fatalf("precondition: the recorded proof replays clean with actions, got clean %v, %d actions", res.Clean(), len(res.Actions))
	}
	c := cleanGame{res: &res, m: m, seed: r.Seed, rc: r.RunConfig()}
	got := provenClean(t, "Ancient Cornucopia", []cleanGame{c})
	if got == nil {
		t.Fatal("provenClean dropped a game whose recording replays clean")
	}
	if got.seed != r.Seed {
		t.Fatalf("provenClean returned seed %d, want %d", got.seed, r.Seed)
	}
}
