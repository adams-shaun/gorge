package host

// REPRO_REGEN_FIXTURE=1 regenerates committedCaptureRel from a REAL live
// match — parkedOvershootMatch driven to the measured overshoot burst,
// snapshotted with the production SnapshotForFeedback path — exactly the
// mechanism cmd/repro/fixture_gen_test.go uses for its own committed
// fixture. Never hand-edit the committed JSON; run this and commit what it
// writes.
//
// fb-20260915T094418Z / event-802 staleness (2026-09-17): the previously
// committed capture was itself regenerated with parkedOvershootMatch before
// that helper wired host.Options.Tokens (see overshoot_tail_test.go) — its
// live match ran token-starved, so a token-minting ability it hit recorded
// the engine's "unimplemented API" Note stand-in. feedback.Load's
// resolveTokens falls back to the full corpus token map when a capture
// carries no token text, so replaying that same capture today actually
// mints the token: "recorded move_zone [Note's text], replayed choose"
// diverging at the first decision downstream of the mismatch (event 802).
// Fixed at the source (Tokens now threaded through), then this regenerates
// the fixture so the committed copy matches a token-complete live capture.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil/feedback"
	"github.com/adams-shaun/gorge/replay"
	"github.com/adams-shaun/gorge/state"
)

func TestGenerateOvershootCapture(t *testing.T) {
	if os.Getenv("REPRO_REGEN_FIXTURE") == "" {
		t.Skip("set REPRO_REGEN_FIXTURE=1 to regenerate " + committedCaptureRel)
	}
	root, err := feedback.Root()
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	r, tb, m, _ := parkedOvershootMatch(t, "")
	_ = tb
	assertOvershootShape(t, m)

	seat0 := state.PlayerID(0)
	snap, err := r.SnapshotForFeedback("t1", &seat0)
	if err != nil {
		t.Fatalf("SnapshotForFeedback: %v", err)
	}

	dir := t.TempDir()
	matchRaw, err := json.MarshalIndent(snap.Match, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	logRaw, err := json.MarshalIndent(snap.Log, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	viewRaw, err := json.MarshalIndent(snap.View, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []struct {
		name string
		body []byte
	}{
		{"match.json", matchRaw}, {"log.json", logRaw}, {"view.json", viewRaw},
	} {
		if err := os.WriteFile(filepath.Join(dir, f.name), append(f.body, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Verify before writing: the live capture (match.json still carries its
	// token script text) must replay to its own recorded head.
	l, cfg, meta, err := feedback.Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	e, err := replay.Replay(l, cfg)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if got := e.L.Head(); got != meta.Head {
		t.Fatalf("replayed head %q, recorded %q", got, meta.Head)
	}

	id := filepath.Base(committedCaptureRel)
	dst := filepath.Join(root, "cmd", "repro", "testdata", "feedback", id)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	strippedMatch, err := writeCommittedOvershootMatch(matchRaw, id)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"match.json": strippedMatch,
		"log.json":   append(logRaw, '\n'),
		"view.json":  append(viewRaw, '\n'),
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dst, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// report.json: keep the existing committed one's provenance fields —
	// copy it over unchanged if present, since it documents the ORIGINAL
	// report, not the regeneration.
	if raw, err := os.ReadFile(filepath.Join(dst, "report.json")); err == nil {
		if err := os.WriteFile(filepath.Join(dst, "report.json"), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Round-trip check: the committed copy has its token text stripped, so
	// re-loading it must resolve those tokens through the gitignored sync
	// directory and still replay to the same head.
	l2, cfg2, meta2, err := feedback.Load(dst)
	if err != nil {
		t.Fatalf("load committed fixture: %v", err)
	}
	e2, err := replay.Replay(l2, cfg2)
	if err != nil {
		t.Fatalf("replay committed fixture: %v", err)
	}
	if got := e2.L.Head(); got != meta2.Head {
		t.Fatalf("committed fixture replayed head %q, recorded %q", got, meta2.Head)
	}
	if meta2.Head != meta.Head {
		t.Fatalf("stripped fixture head %q differs from capture head %q", meta2.Head, meta.Head)
	}
	t.Logf("fixture written to %s (%d events, %d intents, head %s)", dst, len(l2.Events), meta2.IntentCount, meta2.Head)
}

// writeCommittedOvershootMatch mirrors cmd/repro/emit.go's
// writeCommittedMatch/syncTokens exactly (unexported there, so duplicated
// here rather than importing package main): strips match.json's `tokens`/
// `tokens_unread` (Forge scripts are GPL-3.0; gorge is Apache-2.0) and
// syncs the real script text into the gitignored per-fixture-id directory
// feedback.TokenSyncDir names, so the committed fixture still replays
// locally.
func writeCommittedOvershootMatch(raw []byte, id string) ([]byte, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	if tokRaw, ok := doc["tokens"]; ok {
		var tokens map[string]string
		if err := json.Unmarshal(tokRaw, &tokens); err != nil {
			return nil, err
		}
		if len(tokens) > 0 {
			syncDir, err := feedback.TokenSyncDir(id)
			if err != nil {
				return nil, err
			}
			if err := os.MkdirAll(syncDir, 0o755); err != nil {
				return nil, err
			}
			tokRawOut, err := json.MarshalIndent(tokens, "", "  ")
			if err != nil {
				return nil, err
			}
			if err := os.WriteFile(filepath.Join(syncDir, "tokens.json"), append(tokRawOut, '\n'), 0o644); err != nil {
				return nil, err
			}
		}
		delete(doc, "tokens")
	}
	delete(doc, "tokens_unread")
	// `name_universe_names` is the whole corpus's sorted card-name list
	// (~24k entries): dropped here exactly as cmd/repro/emit.go's
	// writeCommittedMatch drops it (the mirror this function duplicates) —
	// the `name_universe` MODE bit stays, so the fixture still replays with
	// a universe, and feedback.config() re-derives the label list from the
	// live corpus. Without this the re-recorded fixture grows by ~500 KB
	// of corpus-derived JSON (measured: 20260923 re-record). The mirror
	// had drifted from emit.go when the stripper gained this deletion.
	delete(doc, "name_universe_names")
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}
