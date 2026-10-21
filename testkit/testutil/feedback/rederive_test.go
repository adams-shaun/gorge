package feedback

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules"
)

// rederiveEnv names the fixture directories (comma-separated, relative to
// the repo root) TestRederiveFeedbackFixtures re-derives. Unset, it skips.
const rederiveEnv = "FEEDBACK_REDERIVE"

// TestRederiveFeedbackFixtures re-derives a real player capture's log.json
// after an engine change that moves WHEN a decision is posed but not WHICH
// decisions are posed or how they are answered (precedent 8a3613cea,
// 6960ea1e4). The recorded intents are replayed in order through this tree's
// engine (the genesis replay.walk uses); each keeps its recorded Player,
// Choices and payment fields and takes the pending decision's Seq, which is
// all a cast-order reorder changes. It refuses (fails) when an intent's
// player or decision kind does not match the pending decision, so it can
// never silently re-answer a different question. log.json is rewritten in
// host.FeedbackLog's shape and match.json's "events" count follows it.
func TestRederiveFeedbackFixtures(t *testing.T) {
	spec := os.Getenv(rederiveEnv)
	if spec == "" {
		t.Skip("set " + rederiveEnv + "=<dir>[,<dir>...] (repo-relative) to re-derive feedback fixtures")
	}
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range strings.Split(spec, ",") {
		dir := filepath.Join(root, strings.TrimSpace(rel))
		if err := rederive(dir); err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
	}
}

func rederive(dir string) error {
	l, cfg, meta, err := Load(dir)
	if err != nil {
		return err
	}
	cfg.Seed = l.Seed
	asked := false
	for _, ev := range l.Events {
		if ev.Kind == events.DecisionAsk && ev.Text == "starting_player" {
			asked = true
			break
		}
	}
	var e *rules.Engine
	if asked {
		e = rules.NewStartingPlayerChoice(cfg)
		if e.AskStartingPlayer() == nil {
			return fmt.Errorf("log recorded a starting_player ask but none is available")
		}
	} else {
		e = rules.New(cfg)
	}
	e.Advance()
	askKind := map[int]string{} // recorded decision seq -> recorded kind
	for _, ev := range l.Events {
		if ev.Kind == events.DecisionAsk {
			askKind[int(ev.Seq)] = ev.Text
		}
	}
	moved := 0
	for i, in := range l.Intents {
		d := e.Pending()
		if d == nil || e.G.Over {
			return fmt.Errorf("intent %d: no decision pending", i)
		}
		if d.Player != in.Player || string(d.Kind) != askKind[int(in.Seq)] {
			return fmt.Errorf("intent %d (seq %d, %s, p%d): pending is %s p%d at seq %d -- not a pure reorder",
				i, in.Seq, askKind[int(in.Seq)], in.Player, d.Kind, d.Player, d.Seq)
		}
		if in.Seq != d.Seq {
			moved++
		}
		in.Seq = d.Seq
		if err := e.Submit(in); err != nil {
			return fmt.Errorf("intent %d rejected: %w", i, err)
		}
	}
	if e.L.Head() == meta.Head && moved == 0 {
		fmt.Printf("%s: unchanged (head %s)\n", filepath.Base(dir), meta.Head)
		return nil
	}
	out := struct {
		events.Log
		Head        string `json:"head"`
		IntentCount int    `json:"intent_count"`
		Turn        int32  `json:"turn"`
		Step        string `json:"step"`
		Priority    int    `json:"priority"`
		Active      int    `json:"active"`
	}{Log: *e.L, Head: e.L.Head(), IntentCount: len(e.L.Intents), Turn: e.G.Turn,
		Step: e.G.Step.String(), Priority: int(e.G.Priority), Active: int(e.G.Active)}
	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "log.json"), append(raw, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Printf("%s: %d -> %d events, %d intents (%d re-sequenced), head %s -> %s\n",
		filepath.Base(dir), len(l.Events), len(e.L.Events), len(e.L.Intents), moved, meta.Head, e.L.Head())
	if len(e.L.Events) == len(l.Events) {
		return nil // match.json's "events" count is unchanged: leave the file byte for byte
	}
	mp := filepath.Join(dir, "match.json")
	mraw, err := os.ReadFile(mp)
	if err != nil {
		return err
	}
	// Only the "events" count moves; the rest of match.json is kept byte for byte.
	mout := eventsCountRE.ReplaceAll(mraw, []byte(fmt.Sprintf(`"events": %d,`, len(e.L.Events))))
	return os.WriteFile(mp, mout, 0o644)
}

var eventsCountRE = regexp.MustCompile(`"events": [0-9]+,`)
