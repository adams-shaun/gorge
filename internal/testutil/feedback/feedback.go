// Package feedback loads a feedback snapshot directory — the match.json /
// log.json (plus optional view.json / report.json) files the server's
// feedback capture (task fbrepro1) writes beside a bug report — back into
// the (events.Log, rules.Config) pair a deterministic replay needs, and
// drives that replay to a chosen intent.
//
// It exists so turning a report into a test case is one command (cmd/repro)
// and one call in the resulting test (EngineAt): a triage or implementer
// seat starts from the exact board the reporter saw, not from a guessed
// one. The contract it serves is replay's: (Config, Log) together, never
// the Log alone — the log cannot recover deck contents or the token table,
// so match.json's recorded card-name lists and token scripts are rebuilt
// into a Config through the same corpus the live match loaded its cards
// from.
//
// This package sits at the far end of the dependency order (it imports
// cards, state, events, rules and replay), one step before cmd/*: nothing
// in the engine may import it. It mirrors the snapshot's JSON shapes
// locally rather than importing host, so a package anywhere in the tree —
// including a generated repro test inside an early package — can depend on
// it without dragging the host tier in. The mirror is pinned by host's own
// marshal-shape test on one side and by cmd/repro's end-to-end tests on the
// other, which replay a snapshot produced by the live capture code; a field
// either side forgets fails there instead of silently replaying a
// different match.
package feedback

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/replay"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/state"
)

// matchJSON is match.json's shape, mirroring host.FeedbackMatch's tags
// field for field. Only the fields a replay's Config needs are read here,
// but every field host.FeedbackMatch marshals is declared, so an
// unrecognised key is at worst ignored, never a decode error — and the
// end-to-end tests replay a snapshot host actually produced, which is what
// catches drift.
type matchJSON struct {
	Table        string            `json:"table"`
	Match        int               `json:"match"`
	Seed         uint64            `json:"seed"`
	Names        []string          `json:"names"`
	PlayerNames  []string          `json:"player_names,omitempty"`
	Decks        []string          `json:"decks"`
	DeckCards    [][]string        `json:"deck_cards"`
	Spectator    string            `json:"spectator"`
	State        string            `json:"state"`
	Result       string            `json:"result,omitempty"`
	Winner       *uint8            `json:"winner"`
	Head         string            `json:"head,omitempty"`
	Events       int               `json:"events"`
	Turns        int32             `json:"turns"`
	Reason       string            `json:"reason,omitempty"`
	Mulligans    int               `json:"mulligans,omitempty"`
	Format       string            `json:"format,omitempty"`
	StartingLife int32             `json:"starting_life,omitempty"`
	Commanders   [][]int           `json:"commanders,omitempty"`
	Tokens       map[string]string `json:"tokens,omitempty"`
	TokensUnread []string          `json:"tokens_unread,omitempty"`
}

// logJSON is log.json's shape, mirroring host.FeedbackLog: the embedded
// events.Log (seed, events, intents) plus the prefix facts. The embedded
// Log decodes straight back into an events.Log — Seed, Events and Intents
// are the replay input; Head is what a replay must reproduce.
type logJSON struct {
	events.Log
	Head        string         `json:"head"`
	IntentCount int            `json:"intent_count"`
	Turn        int32          `json:"turn"`
	Step        string         `json:"step"`
	Priority    state.PlayerID `json:"priority"`
	Active      state.PlayerID `json:"active"`
}

// reportJSON is report.json's shape, mirroring cmd/gorged's report struct.
// Only the snapshot status is load-bearing here (it says whether the
// capture was complete); the rest is carried in Meta for context.
type reportJSON struct {
	Received string `json:"received"`
	Text     string `json:"text"`
	URL      string `json:"url,omitempty"`
	Snapshot string `json:"snapshot,omitempty"`
}

// Meta describes the snapshot as loaded: where it came from, the game
// facts log.json recorded at the capture point, and the capture's own
// status. It is provenance and triage context, not replay input — the
// replay input is the returned Log and Config.
type Meta struct {
	// Dir is the snapshot directory as the caller named it; ID is its base
	// name (the feedback report id the server generated).
	Dir, ID string
	// Report is report.json's snapshot status line ("captured: ...",
	// "partial: ...", "unavailable: ..."), empty when the report carries
	// none. A "partial" or "unavailable" snapshot may not replay.
	Report string
	// Text is the reporter's own words, when report.json is present.
	Text string
	// Table and Match name the table and match the snapshot captured.
	Table string
	Match int
	// Head is the chain hash log.json recorded at the capture prefix: what
	// a full replay must reproduce.
	Head string
	// IntentCount is len(Log.Intents) as recorded; Turn/Step/Priority/
	// Active are the game facts read off the same locked instant the log
	// was copied. Step is the Step's String() spelling.
	IntentCount int
	Turn        int32
	Step        string
	Priority    state.PlayerID
	Active      state.PlayerID
	// Seats is the number of seats the match dealt.
	Seats int
	// HasView is true when the snapshot carries a view.json (the report
	// named a seat and the projection succeeded).
	HasView bool
	// TokensUnread mirrors match.json's tokens_unread: token scripts that
	// could not be read back at capture time. A replay that reaches one of
	// these stems will diverge the same way a missing token table would.
	TokensUnread []string
}

// Load reads a feedback snapshot directory and rebuilds the (Log, Config)
// pair replay needs, resolving the recorded card names and token scripts
// through the repo's own corpus (the same card loader the live match used).
//
// match.json and log.json are required; report.json and view.json are
// optional (a report may name no seat, and older capture shapes may have
// written none). Missing files, unparseable JSON, a card name the corpus
// does not know and a token script that does not compile are errors — a
// repro that silently replayed a different match would be worse than one
// that refuses. The corpus is found through the repo root (.cards beside
// it); a checkout with no .cards/ fails here rather than skipping, because
// a caller of Load asked for a reproduction, not a test harness.
func Load(dir string) (*events.Log, rules.Config, Meta, error) {
	var cfg rules.Config
	matchRaw, err := os.ReadFile(filepath.Join(dir, "match.json"))
	if err != nil {
		return nil, cfg, Meta{}, fmt.Errorf("feedback: %s: %w", dir, err)
	}
	logRaw, err := os.ReadFile(filepath.Join(dir, "log.json"))
	if err != nil {
		return nil, cfg, Meta{}, fmt.Errorf("feedback: %s: %w", dir, err)
	}
	var m matchJSON
	if err := json.Unmarshal(matchRaw, &m); err != nil {
		return nil, cfg, Meta{}, fmt.Errorf("feedback: %s: match.json: %w", dir, err)
	}
	var lg logJSON
	if err := json.Unmarshal(logRaw, &lg); err != nil {
		return nil, cfg, Meta{}, fmt.Errorf("feedback: %s: log.json: %w", dir, err)
	}

	meta := Meta{
		Dir:          dir,
		ID:           filepath.Base(dir),
		Head:         lg.Head,
		IntentCount:  lg.IntentCount,
		Turn:         lg.Turn,
		Step:         lg.Step,
		Priority:     lg.Priority,
		Active:       lg.Active,
		Table:        m.Table,
		Match:        m.Match,
		Seats:        len(m.DeckCards),
		TokensUnread: m.TokensUnread,
	}
	if rep, err := os.ReadFile(filepath.Join(dir, "report.json")); err == nil {
		var r reportJSON
		if err := json.Unmarshal(rep, &r); err == nil {
			meta.Report = r.Snapshot
			meta.Text = r.Text
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "view.json")); err == nil {
		meta.HasView = true
	}

	reg, err := openCorpus()
	if err != nil {
		return nil, cfg, meta, err
	}
	cfg, err = config(m, reg)
	if err != nil {
		return nil, cfg, meta, err
	}
	return &lg.Log, cfg, meta, nil
}

// config rebuilds a rules.Config from match.json's content: decks from the
// recorded card-name lists (resolved through the registry exactly the way
// deck.File.Resolve resolves a deck file — Lookup normalises the name, so
// the recorded printed name finds its card), token scripts recompiled the
// way the corpus's own compile pipeline compiles them (ParseBytes, Link,
// ApplyIntrinsics), and the format/life/commander/mulligan settings. The
// seed is carried but replay overwrites it with the log's own seed
// (replay's Ruling P5), which is the one that produced the log.
func config(m matchJSON, reg *cards.Registry) (rules.Config, error) {
	if len(m.DeckCards) == 0 {
		return rules.Config{}, fmt.Errorf("feedback: match.json records no deck_cards")
	}
	decks := make([][]*cards.Card, len(m.DeckCards))
	for i, names := range m.DeckCards {
		decks[i] = make([]*cards.Card, len(names))
		for j, n := range names {
			c, ok := reg.Lookup(n)
			if !ok {
				return rules.Config{}, fmt.Errorf("feedback: seat %d card %d (%q) is not in the corpus", i, j, n)
			}
			decks[i][j] = c
		}
	}
	cfg := rules.Config{
		Seed:        m.Seed,
		Names:       m.Names,
		PlayerNames: m.PlayerNames,
		Decks:       decks,
		Mulligans:   m.Mulligans,
	}
	if m.Format == "commander" {
		cfg.Format = rules.FormatCommander
		cfg.StartingLife = m.StartingLife
		cfg.Commanders = m.Commanders
	}
	stems := make([]string, 0, len(m.Tokens))
	for s := range m.Tokens {
		stems = append(stems, s)
	}
	sort.Strings(stems)
	cfg.Tokens = make(map[string]*cards.Card, len(stems))
	for _, s := range stems {
		c, err := compileToken(s, m.Tokens[s])
		if err != nil {
			return rules.Config{}, err
		}
		cfg.Tokens[s] = c
	}
	return cfg, nil
}

// compileToken recompiles one recorded token script through the corpus's
// own parse/link/intrinsics pipeline — the exact pipeline cards' compile
// runs — so the rebuilt card is the one the match minted from, not a
// zero-derived shell (a compiled card's P/T and colour identity are
// derived, which is why match.json ships script text and not cards).
func compileToken(stem, src string) (*cards.Card, error) {
	c, diags := cards.ParseBytes("tokenscripts/"+stem+".txt", []byte(src))
	if len(diags) != 0 {
		return nil, fmt.Errorf("feedback: token %q: %v", stem, diags)
	}
	if d := c.Link(); len(d) != 0 {
		return nil, fmt.Errorf("feedback: token %q: link: %v", stem, d)
	}
	for _, f := range c.Faces {
		f.ApplyIntrinsics()
	}
	return c, nil
}

// EngineAt loads the snapshot at dir and replays it to the first n
// recorded intents — an engine positioned exactly where the reporter's
// game stood after n answers. n < 0 (the default the generated test
// skeletons pass) replays every recorded intent, i.e. to the capture
// point. Load errors and replay divergences are t.Fatalf: a repro test is
// about what happens AT the reported state, and getting there is fixture
// work, not an error to continue past.
//
// A negative n is NORMALIZED HERE, not left for replay.ReplayTo: ReplayTo
// clamps its own argument to [0, len(l.Intents)], so a bare -1 handed
// through would mean genesis — zero intents, the board before the match
// began — which is not what a caller passing the default asked for. EngineAt
// turns any negative into len(l.Intents) first, so the documented "every
// recorded intent" is what happens.
//
// The replay is replay.ReplayTo — the same code path cmd/repro drives — so
// an engine this returns and an engine a caller builds from Load's own
// return values are the same replay by construction; the head-equality
// gate is pinned in cmd/repro's tests.
func EngineAt(t testing.TB, dir string, n int) *rules.Engine {
	t.Helper()
	l, cfg, _, err := Load(dir)
	if err != nil {
		t.Fatalf("feedback: %v", err)
	}
	if n < 0 {
		n = len(l.Intents) // ReplayTo clamps negatives to 0 (genesis); the default here is ALL intents
	}
	e, err := replay.ReplayTo(l, cfg, n)
	if err != nil {
		t.Fatalf("feedback: %s: replay to intent %d: %v", dir, n, err)
	}
	return e
}

// openCorpus finds and opens the repo's .cards corpus. The repo root is
// resolved the way cards/boundary_test.go and testutil.CorpusRegistry do —
// `git rev-parse --show-toplevel` with every inherited GIT_* variable
// stripped (cards.GitEnv), because a hook or test harness exports GIT_DIR
// as a relative path that does not resolve from this package's own
// directory. A checkout with no .cards/ is an error, not a skip: Load's
// callers asked for a reproduction.
func openCorpus() (*cards.Registry, error) {
	root, err := repoRoot()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(root, ".cards")
	if _, err := os.Stat(dir); err != nil {
		return nil, fmt.Errorf("feedback: no .cards/ corpus at %s — run `make fetch-cards compile-cards`", dir)
	}
	reg, err := cards.OpenCorpus(dir)
	if err != nil {
		return nil, fmt.Errorf("feedback: corpus at %s: %w", dir, err)
	}
	return reg, nil
}

// repoRoot resolves the worktree root the same way testutil.CorpusRegistry
// does. Exported through Root so cmd/repro can resolve -emit-test package
// paths against the same root the corpus was found at.
func repoRoot() (string, error) {
	out, err := gitRevParse()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// Root is the resolved repo root — where .cards lives and where -emit-test
// resolves a relative package path against.
func Root() (string, error) { return repoRoot() }

// gitRevParse runs `git rev-parse --show-toplevel` from this package's own
// directory, with every inherited GIT_* variable stripped — a hook or test
// harness exports GIT_DIR as a relative path (usually ".git") that only
// resolves from the repo root, so an unstripped environment makes this
// read the wrong place or fail outright.
func gitRevParse() (string, error) {
	cmd := exec.Command("git", "-C", ".", "rev-parse", "--show-toplevel")
	cmd.Env = cards.GitEnv()
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("feedback: could not resolve git repo root: %v", err)
	}
	return string(out), nil
}
