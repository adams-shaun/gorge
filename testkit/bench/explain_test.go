package bench_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/azredeal"
	"github.com/adams-shaun/gorge/bots/explain"
	"github.com/adams-shaun/gorge/bots/heuristic"
	ss "github.com/adams-shaun/gorge/bots/searchseat"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/matchrecord"
	"github.com/adams-shaun/gorge/testkit/bench"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// notesOf decodes a record's Explain, failing unless it holds exactly one
// entry per intent.
func notesOf(t *testing.T, rec *matchrecord.Record) []*explain.Note {
	t.Helper()
	if len(rec.Explain) != len(rec.Log.Intents) {
		t.Fatalf("%d explain entries for %d intents", len(rec.Explain), len(rec.Log.Intents))
	}
	out := make([]*explain.Note, len(rec.Explain))
	for i, raw := range rec.Explain {
		if string(raw) == "null" {
			continue
		}
		var n explain.Note
		if err := json.Unmarshal(raw, &n); err != nil {
			t.Fatalf("explain[%d]: %v", i, err)
		}
		out[i] = &n
	}
	return out
}

// checkNotes asserts the overlay invariants over a whole recorded game: every
// intent has a note with a source, written for the seat that answered it; a
// note that knows the bot's answer and says the bot answered (heuristic,
// forced) carries exactly the logged intent; a searched policy note names
// candidate 0 as the bot's answer and its chosen row is the logged intent.
// It returns the per-source counts.
func checkNotes(t *testing.T, rec *matchrecord.Record, notes []*explain.Note) map[explain.Source]int {
	t.Helper()
	by := map[explain.Source]int{}
	for i, n := range notes {
		in := rec.Log.Intents[i]
		if n == nil {
			t.Fatalf("intent %d (seat %d): null note", i, in.Player)
		}
		if n.Source == "" {
			t.Fatalf("intent %d: note without a source: %+v", i, n)
		}
		if n.Seat != int(in.Player) {
			t.Fatalf("intent %d: note for seat %d, intent of seat %d", i, n.Seat, in.Player)
		}
		by[n.Source]++
		if (n.Source == explain.Heuristic || n.Source == explain.Forced) && n.Bot != nil && !slices.Equal(n.Bot, in.Choices) {
			t.Fatalf("intent %d: source %s, bot answered %v, intent played %v", i, n.Source, n.Bot, in.Choices)
		}
		if n.Source != explain.Policy || n.Search == nil {
			continue
		}
		var bot, chosen *explain.Row
		for j := range n.Search.Rows {
			r := &n.Search.Rows[j]
			if r.Bot {
				bot = r
			}
			if r.Chosen {
				chosen = r
			}
		}
		if bot != nil && !slices.Equal(n.Bot, bot.Choices) {
			t.Fatalf("intent %d: note bot %v, candidate 0 %v", i, n.Bot, bot.Choices)
		}
		if chosen == nil || !slices.Equal(chosen.Choices, in.Choices) {
			t.Fatalf("intent %d: chosen row %+v, intent played %v", i, chosen, in.Choices)
		}
		for _, r := range n.Search.Rows {
			if r.Q < -1 || r.Q > 1 {
				t.Fatalf("intent %d: row Q %v outside [-1,1]", i, r.Q)
			}
		}
		if v := n.Search.RootValue; v < -1 || v > 1 {
			t.Fatalf("intent %d: root value %v outside [-1,1]", i, v)
		}
	}
	return by
}

// azSeat is the hosted az-redeal adapter at a test budget.
func azSeat(t *testing.T, seed uint64, sims int) seat.Seat {
	t.Helper()
	cfg := azredeal.Hosted()
	cfg.Search.Sims = sims
	s, err := azredeal.New(bots.Options{Seed: seed}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// TestRecordExplainsEveryIntentOfAnAZGame is T3's done-means: an
// az-redeal-vs-heuristic bench game recorded with Hooks.Record carries a
// non-null note with a source on every intent of both seats, the notes
// agree with the log (checkNotes), the record still verifies, and recording
// changes no answer (the same game unrecorded has the same head). It logs
// the record's gzipped size with and without the overlay.
func TestRecordExplainsEveryIntentOfAnAZGame(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	const seed = 424242
	cfg := recordConfig(t, reg, seed, "mono-red-prowess", "mono-blue-tempo")
	pols := []string{azredeal.Policy, "heuristic"}
	seats := func() []seat.Seat { return []seat.Seat{azSeat(t, seed^1, 8), heuristic.NewBot(seed ^ 2)} }
	dir := t.TempDir()
	var paths []string
	hooks := bench.Hooks{EnvSeats: true, Record: fileSink(t, dir, pols, &paths)}
	o, e, err := bench.PlayGame(cfg, seats(), 60, 6000, hooks)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 {
		t.Fatalf("%d records", len(paths))
	}
	_, plain, err := bench.PlayGame(cfg, seats(), 60, 6000, bench.Hooks{EnvSeats: true})
	if err != nil {
		t.Fatal(err)
	}
	if plain.L.Head() != e.L.Head() {
		t.Fatalf("recording changed the game: head %s recorded, %s unrecorded", e.L.Head(), plain.L.Head())
	}
	rec, rcfg, err := matchrecord.Load(paths[0], reg)
	if err != nil {
		t.Fatal(err)
	}
	if err := matchrecord.Verify(rec, rcfg); err != nil {
		t.Fatal(err)
	}
	by := checkNotes(t, rec, notesOf(t, rec))
	if by[explain.Policy] == 0 {
		t.Fatalf("no policy note: the az seat's searches never reached the record (%v)", by)
	}
	seatsNoted := map[int]bool{}
	for _, n := range notesOf(t, rec) {
		seatsNoted[n.Seat] = true
		if n.Seat == 0 && n.Policy != azredeal.Policy || n.Seat == 1 && n.Policy != "heuristic" {
			t.Fatalf("seat %d note names policy %q", n.Seat, n.Policy)
		}
	}
	if !seatsNoted[0] || !seatsNoted[1] {
		t.Fatalf("notes cover seats %v, want both", seatsNoted)
	}
	withFI, err := os.Stat(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	rec.Explain = nil
	bare := filepath.Join(dir, "bare"+matchrecord.Ext)
	if err := matchrecord.Write(bare, rec); err != nil {
		t.Fatal(err)
	}
	bareFI, err := os.Stat(bare)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("seed %d: %d turns, %d intents, stall %q; sources %v; record %d B gzipped (%d B without explain)",
		seed, e.G.Turn, len(e.L.Intents), o.StallOn, by, withFI.Size(), bareFI.Size())
	if withFI.Size() > 64<<10 {
		t.Errorf("record is %d B gzipped, over the 64 KB budget", withFI.Size())
	}
}

// TestRecordExplainsASearchSeatGame records the teacher seat (searchseat)
// against the bot for the first hundred intents (the budget
// TestSearchSeatGameCompletesWithCoverage plays): the intent-capped stall
// still records, and its scored decisions carry searchseat tables.
func TestRecordExplainsASearchSeatGame(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	const seed = uint64(30000000)
	cfg := recordConfig(t, reg, seed, "mono-red-prowess", "mono-blue-tempo")
	var paths []string
	hooks := bench.Hooks{Record: fileSink(t, t.TempDir(), []string{"search", "heuristic"}, &paths)}
	if _, _, err := bench.PlayGame(cfg, []seat.Seat{ss.NewSearchBot(seed^1, ss.Defaults()), heuristic.NewBot(seed ^ 2)}, 200, 100, hooks); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 {
		t.Fatalf("%d records", len(paths))
	}
	rec, _, err := matchrecord.Load(paths[0], reg)
	if err != nil {
		t.Fatal(err)
	}
	notes := notesOf(t, rec)
	by := checkNotes(t, rec, notes)
	searched := 0
	for _, n := range notes {
		if n.Search == nil {
			continue
		}
		searched++
		if n.Search.Engine != "searchseat" || len(n.Search.Rows) < 2 {
			t.Fatalf("searchseat note %+v", n.Search)
		}
		for _, r := range n.Search.Rows {
			if r.Choices == nil {
				t.Fatalf("a teacher row did not translate to choices: %+v", n.Search)
			}
		}
	}
	if searched == 0 {
		t.Fatalf("no scored decision in the record (%v)", by)
	}
	t.Logf("sources %v, %d scored decisions", by, searched)
}

// refuseOnceSeat answers from a wrapped bot except its first answer, which
// the engine refuses. With retry it is a bots.RefusalAnswerer (the ladder's
// first rung is its own re-answer); without, the ladder falls to the shared
// bots.Fallbacks.
type refuseOnceSeat struct {
	inner   seat.Seat
	refused bool
}

func (s *refuseOnceSeat) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	in, err := s.inner.Decide(ctx, v, d)
	if err == nil && !s.refused {
		s.refused = true
		return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{len(d.Options)}}, nil
	}
	return in, err
}

type refuseOnceRetry struct{ refuseOnceSeat }

func (s *refuseOnceRetry) AnswerRefused(v view.View, d decision.Decision, _ decision.Intent) decision.Intent {
	in, _ := s.inner.Decide(context.Background(), v, d)
	return in
}

// TestRecordExplainsTheRefusalLadder: the decision whose first answer the
// engine refused is noted with the ladder rung that answered it --
// refusal_retry for the seat's own AnswerRefused, refusal_fallback with rung
// 1 for the first shared fallback -- and the note is filed at the accepted
// intent's index.
func TestRecordExplainsTheRefusalLadder(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	const seed = 424242
	for _, tc := range []struct {
		name string
		s    seat.Seat
		want explain.Source
		rung int
	}{
		{"retry", &refuseOnceRetry{refuseOnceSeat{inner: heuristic.NewBot(seed ^ 1)}}, explain.RefusalRetry, 0},
		{"fallback", &refuseOnceSeat{inner: heuristic.NewBot(seed ^ 1)}, explain.RefusalFallback, 1},
	} {
		cfg := recordConfig(t, reg, seed, "mono-black-aggro", "mono-red-prowess")
		var paths []string
		hooks := bench.Hooks{Record: fileSink(t, t.TempDir(), []string{"naughty", "heuristic"}, &paths)}
		if _, _, err := bench.PlayGame(cfg, []seat.Seat{tc.s, heuristic.NewBot(seed ^ 2)}, 60, 4000, hooks); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		rec, _, err := matchrecord.Load(paths[0], reg)
		if err != nil {
			t.Fatal(err)
		}
		notes := notesOf(t, rec)
		checkNotes(t, rec, notes)
		first := -1
		for i, in := range rec.Log.Intents {
			if in.Player == 0 {
				first = i
				break
			}
		}
		if first < 0 {
			t.Fatalf("%s: seat 0 never answered", tc.name)
		}
		n := notes[first]
		if n.Source != tc.want || n.Rung != tc.rung || n.Reason == "" {
			t.Fatalf("%s: seat 0's refused decision noted %+v, want source %s rung %d with the refusal as reason", tc.name, n, tc.want, tc.rung)
		}
		ladder := 0
		for _, n := range notes {
			if n.Source == explain.RefusalRetry || n.Source == explain.RefusalFallback {
				ladder++
			}
		}
		if ladder != 1 {
			t.Fatalf("%s: %d ladder notes, want 1", tc.name, ladder)
		}
	}
}
