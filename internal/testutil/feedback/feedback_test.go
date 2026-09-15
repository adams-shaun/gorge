package feedback

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/replay"
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

// TestLoadSyncTokensWinOverCorpus exercises the production boundary, not
// resolveTokens in isolation: this stripped copy of a real fixture has a
// synced script that differs from the live corpus. Load must put
// that 3/4 card in Config.Tokens, which is exactly the Config replay receives.
func TestLoadSyncTokensWinOverCorpus(t *testing.T) {
	const stem = "r_1_1_goblin"
	const id = "zz-load-sync-wins-test"
	dir := copyStrippedFixture(t, id)

	syncDir, err := TokenSyncDir(id)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(syncDir) })
	if err := os.MkdirAll(syncDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeSyncTokens(t, syncDir, map[string]string{stem: "Name:Goblin Token\nManaCost:no cost\nColors:red\nTypes:Creature Goblin\nPT:3/4\nOracle:\n"}); err != nil {
		t.Fatal(err)
	}
	reg, err := openCorpus()
	if err != nil {
		t.Fatal(err)
	}
	corpus := reg.Tokens[stem]
	if corpus == nil {
		t.Fatalf("corpus carries no %q token", stem)
	}
	if corpus.Faces[0].Power() == 3 && corpus.Faces[0].Toughness() == 4 {
		t.Fatalf("live corpus %q is already 3/4; test fixture no longer distinguishes sync precedence", stem)
	}

	l, cfg, meta, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := cfg.Tokens[stem]
	if got == nil {
		t.Fatalf("Load Config.Tokens carries no %q", stem)
	}
	if p, toughness := got.Faces[0].Power(), got.Faces[0].Toughness(); p != 3 || toughness != 4 {
		t.Fatalf("Load Config.Tokens[%q] = %d/%d, want synced 3/4", stem, p, toughness)
	}
	// Replay consumes precisely the Config Load returned. This fixture does
	// not mint a token, so its recorded event chain remains unchanged while
	// still proving the production Config supplied to replay contains it.
	e, err := replay.Replay(l, cfg)
	if err != nil {
		t.Fatalf("replay with Load config: %v", err)
	}
	if got := e.L.Head(); got != meta.Head {
		t.Errorf("replay head %q, want recorded %q", got, meta.Head)
	}
}

// TestLoadFallsBackToCorpusWhenSyncMissing exercises the other production
// branch: a stripped fixture under an id with no sync directory must Load
// successfully with the current corpus token map.
func TestLoadFallsBackToCorpusWhenSyncMissing(t *testing.T) {
	const stem = "r_1_1_goblin"
	const id = "zz-load-sync-absent-test"
	dir := copyStrippedFixture(t, id)
	syncDir, err := TokenSyncDir(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(syncDir); err != nil {
		t.Fatal(err)
	}

	reg, err := openCorpus()
	if err != nil {
		t.Fatal(err)
	}
	corpus := reg.Tokens[stem]
	if corpus == nil {
		t.Fatalf("corpus carries no %q token", stem)
	}
	l, cfg, meta, err := Load(dir)
	if err != nil {
		t.Fatalf("Load without sync: %v", err)
	}
	got := cfg.Tokens[stem]
	if got == nil {
		t.Fatalf("Load corpus fallback carries no %q", stem)
	}
	if p, toughness := got.Faces[0].Power(), got.Faces[0].Toughness(); p != corpus.Faces[0].Power() || toughness != corpus.Faces[0].Toughness() {
		t.Fatalf("Load corpus fallback token = %d/%d, want corpus %d/%d", p, toughness, corpus.Faces[0].Power(), corpus.Faces[0].Toughness())
	}
	e, err := replay.Replay(l, cfg)
	if err != nil {
		t.Fatalf("replay with corpus fallback: %v", err)
	}
	if got := e.L.Head(); got != meta.Head {
		t.Errorf("replay head %q, want recorded %q", got, meta.Head)
	}
}

const fixtureIDForTest = "20260914T120000Z-fb01"

// copyStrippedFixture gives Load a real, committed capture shape under a
// caller-selected id. Its match.json contains no token text, so that id
// selects either the synced-token or missing-sync branch in Load.
func copyStrippedFixture(t *testing.T, id string) string {
	t.Helper()
	root, err := Root()
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "cmd", "repro", "testdata", "feedback", fixtureIDForTest)
	dir := filepath.Join(t.TempDir(), id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"match.json", "log.json"} {
		raw, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
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
