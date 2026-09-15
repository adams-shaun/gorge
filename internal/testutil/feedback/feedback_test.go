package feedback

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
)

// TestCompileTokenDerivesTheCard pins the token recompilation Load does:
// a recorded script text must come back as a fully derived card — printed
// fields AND the derived P/T the JSON round trip cannot carry — through
// the same ParseBytes/Link/ApplyIntrinsics pipeline the corpus compiler
// runs. A zero-derived shell here would replay a 1/1 token as a 0/0 and
// diverge at its first combat.
func TestCompileTokenDerivesTheCard(t *testing.T) {
	src := "Name:Goblin Warrior Token\nManaCost:no cost\nColors:red,green\nTypes:Creature Goblin Warrior\nPT:1/1\nOracle:\n"
	c, err := compileToken("test_goblin", src)
	if err != nil {
		t.Fatalf("compileToken: %v", err)
	}
	f := c.Faces[0]
	if got := f.Power(); got != 1 || f.Toughness() != 1 {
		t.Errorf("derived P/T = %d/%d, want 1/1", got, f.Toughness())
	}
	if !strings.Contains(strings.Join(f.Types, " "), "Goblin") {
		t.Errorf("types = %v, want Goblin Warrior among them", f.Types)
	}
}

// TestResolveTokensSyncWinsOverCorpus is the test the previous round's
// review asked for. The old sync round-trip test could not detect a
// dropped sync-directory read because the capture, the sync directory and
// the live corpus all carried the SAME current token scripts, so removing
// the sync read changed nothing observable. Here the sync directory
// carries a script that DIFFERS observably from the live corpus (the same
// stem, a different P/T), and resolveTokens must return the synced version
// — proving Load prefers the exact historical text over the current corpus
// when the two disagree.
func TestResolveTokensSyncWinsOverCorpus(t *testing.T) {
	reg, err := openCorpus()
	if err != nil {
		t.Fatalf("openCorpus: %v", err)
	}
	// Pick a stem that exists in the live corpus so the fallback would have
	// produced a real (differing) token too; the synced script overrides it.
	const stem = "r_1_1_goblin"
	corpus, ok := reg.Tokens[stem]
	if !ok {
		t.Fatalf("corpus carries no %q token", stem)
	}
	corpusP, corpusT := corpus.Faces[0].Power(), corpus.Faces[0].Toughness()

	syncText := "Name:Goblin Token\nManaCost:no cost\nColors:red\nTypes:Creature Goblin\nPT:3/4\nOracle:\n"
	id := "zz-sync-wins-test"
	syncDir, err := TokenSyncDir(id)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(syncDir) })
	if err := os.MkdirAll(syncDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeSyncTokens(t, syncDir, map[string]string{stem: syncText}); err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(t.TempDir(), id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	resolved, err := resolveTokens(dir, matchJSON{}, reg)
	if err != nil {
		t.Fatalf("resolveTokens: %v", err)
	}
	got := resolved[stem]
	if got == nil {
		t.Fatalf("resolved tokens carry no %q", stem)
	}
	if p, tgh := got.Faces[0].Power(), got.Faces[0].Toughness(); p != 3 || tgh != 4 {
		t.Errorf("synced token resolved to %d/%d, want 3/4 (sync must beat corpus %d/%d)", p, tgh, corpusP, corpusT)
	}
}

// TestResolveTokensFallsBackToCorpusWhenSyncMissing pins the other side of
// the precedence: a stripped fixture with NO sync directory (a fresh
// checkout that never ran the fixture generator) resolves through the live
// corpus token map, so such a fixture replays anyway instead of failing.
func TestResolveTokensFallsBackToCorpusWhenSyncMissing(t *testing.T) {
	reg, err := openCorpus()
	if err != nil {
		t.Fatalf("openCorpus: %v", err)
	}
	const stem = "r_1_1_goblin"
	corpus, ok := reg.Tokens[stem]
	if !ok {
		t.Fatalf("corpus carries no %q token", stem)
	}

	// A fixture id whose sync directory does not exist anywhere.
	id := "zz-sync-absent-test"
	syncDir, _ := TokenSyncDir(id)
	if _, err := os.Stat(syncDir); err == nil {
		t.Fatalf("test sync dir %s unexpectedly exists", syncDir)
	}

	dir := filepath.Join(t.TempDir(), id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	resolved, err := resolveTokens(dir, matchJSON{}, reg)
	if err != nil {
		t.Fatalf("resolveTokens: %v", err)
	}
	got := resolved[stem]
	if got == nil {
		t.Fatalf("corpus fallback produced no %q token", stem)
	}
	if p, tgh := got.Faces[0].Power(), got.Faces[0].Toughness(); p != corpus.Faces[0].Power() || tgh != corpus.Faces[0].Toughness() {
		t.Errorf("corpus fallback resolved to %d/%d, want corpus %d/%d", p, tgh, corpus.Faces[0].Power(), corpus.Faces[0].Toughness())
	}
}

// writeSyncTokens writes a tokens.json map into a token sync directory in
// the exact shape readTokenScripts reads back (keyed stem → script text).
func writeSyncTokens(t *testing.T, dir string, texts map[string]string) error {
	t.Helper()
	raw, err := json.MarshalIndent(texts, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "tokens.json"), append(raw, '\n'), 0o644)
}

// TestConfigResolvesDecksThroughTheRegistry pins the deck half of Load's
// config rebuild: recorded card names resolve through the registry (the
// same normalised lookup a deck file gets), and a name the registry does
// not know is an error naming the seat and card, never a silent nil card.
func TestConfigResolvesDecksThroughTheRegistry(t *testing.T) {
	reg := cards.NewRegistry()
	c, diags := cards.ParseBytes("test_fixture_card.txt", []byte("Name:Test Pipewhale\nTypes:Creature Whale\nPT:2/2\nOracle:x\n"))
	if len(diags) != 0 {
		t.Fatalf("fixture card: %v", diags)
	}
	reg.Add(c)

	m := matchJSON{DeckCards: [][]string{{"Test Pipewhale"}}}
	cfg, err := config(m, reg, nil)
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	if len(cfg.Decks[0]) != 1 || cfg.Decks[0][0] != c {
		t.Errorf("deck resolved to %+v", cfg.Decks[0])
	}

	m.DeckCards = [][]string{{"Card The Corpus Never Knew"}}
	if _, err := config(m, reg, nil); err == nil {
		t.Fatal("config accepted an unknown card name")
	} else if !strings.Contains(err.Error(), "seat 0 card 0") {
		t.Errorf("error does not name the seat and card: %v", err)
	}
}
