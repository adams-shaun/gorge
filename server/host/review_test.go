package host

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/replay"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/protocol"
)

// reviewRegistry is a memory-mode registry for review tables; LoadDeck is
// never consulted by one.
func reviewRegistry(t *testing.T) *Registry {
	t.Helper()
	r, err := New(Options{LoadDeck: sampleLoader(t)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	return r
}

// TestReviewTableServesTheRecordedMatchLikeTheLiveOne loads the shared
// finished match's (Config, Log) into a review table and pins that every
// read the DVR makes -- the table listing, the match listing, views at
// sampled seqs and the event pages -- serves the same match the live
// registry served, with the same turn-start snapshot structure, an
// omniscient projection and no run loop.
func TestReviewTableServesTheRecordedMatchLikeTheLiveOne(t *testing.T) {
	t.Parallel()
	live, m := finishedTable(t)
	r := reviewRegistry(t)
	if err := r.AddReviewTable("rv", ReviewMatch{Config: m.rulesConfig(), Log: m.e.L.Clone(), Names: []string{"Ann"}, Policies: []string{"bot", "bot", "lethal-pressure"}, Label: "Review"}); err != nil {
		t.Fatal(err)
	}
	// Start is a no-op: no run loop, no new match.
	if err := r.StartAll(); err != nil {
		t.Fatal(err)
	}
	r.Wait("rv")

	tis := r.Tables()
	if len(tis) != 1 {
		t.Fatalf("tables %+v", tis)
	}
	ti := tis[0]
	if !ti.Review || ti.Spectator != "omniscient" || ti.State != protocol.TableIdle || ti.Match != 1 || ti.Seats != 4 || ti.Name != "Review" || ti.BotPolicy != "bot,lethal-pressure" {
		t.Fatalf("review table info %+v", ti)
	}
	ms, err := r.Matches("rv")
	if err != nil || len(ms) != 1 {
		t.Fatalf("matches %v, %v", ms, err)
	}
	liveMs, _ := live.Matches("t1")
	got, want := ms[0], liveMs[0]
	if got.State != protocol.MatchFinished || got.Head != want.Head || got.Events != want.Events || got.Turns != want.Turns || got.Result != want.Result {
		t.Fatalf("review match %+v, live %+v", got, want)
	}
	if (got.Winner == nil) != (want.Winner == nil) || (got.Winner != nil && *got.Winner != *want.Winner) {
		t.Fatalf("winner %v, live %v", got.Winner, want.Winner)
	}
	if got.Seats[0].Name != "Ann" || got.Seats[1].Name != m.gc.PlayerNames[1] || got.Seats[1].Deck != m.gc.Names[1] {
		t.Fatalf("seats %+v", got.Seats)
	}

	// The load walk took the snapshots a live match takes.
	_, rm, err := r.lookup("rv", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rm.snaps) != len(m.snaps) || len(rm.snaps) < 2 {
		t.Fatalf("%d review snapshots, live match took %d", len(rm.snaps), len(m.snaps))
	}
	for i := range rm.snaps {
		if rm.snaps[i].intent != m.snaps[i].intent || rm.snaps[i].seq != m.snaps[i].seq {
			t.Fatalf("snapshot %d at intent %d seq %d, live at intent %d seq %d", i, rm.snaps[i].intent, rm.snaps[i].seq, m.snaps[i].intent, m.snaps[i].seq)
		}
	}

	head := uint64(len(m.e.L.Events) - 1)
	for _, seq := range []uint64{0, m.bounds[0], head / 3, head / 2, head - 1, head} {
		rv, err := r.ViewAt("rv", 1, seq)
		if err != nil {
			t.Fatalf("review ViewAt(%d): %v", seq, err)
		}
		lv, err := live.ViewAt("t1", 1, seq)
		if err != nil {
			t.Fatalf("live ViewAt(%d): %v", seq, err)
		}
		if a, b := viewJSON(t, rv), viewJSON(t, lv); a != b {
			t.Fatalf("seq %d: review view differs from the live registry's\n%s\n%s", seq, a, b)
		}
		if rv.Visibility != view.Omniscient.String() {
			t.Fatalf("seq %d: visibility %q", seq, rv.Visibility)
		}
		for _, p := range rv.Players {
			if p.Hand == nil || len(p.Hand) != p.HandSize || p.Library != nil || p.LibraryTop != nil {
				t.Fatalf("seq %d seat %d: hand %v (size %d), library %v, top %v -- want every hand, no library", seq, p.ID, p.Hand, p.HandSize, p.Library, p.LibraryTop)
			}
		}
	}
	for _, since := range []uint64{0, head / 2, head} {
		re, err := r.Events("rv", 1, since)
		if err != nil {
			t.Fatal(err)
		}
		le, _ := live.Events("t1", 1, since)
		a, _ := json.Marshal(re)
		b, _ := json.Marshal(le)
		if string(a) != string(b) || uint64(len(re)) != head+1-since {
			t.Fatalf("events since %d: %d review bodies, %d live, differ=%t", since, len(re), len(le), string(a) != string(b))
		}
	}
	var bh ErrBeyondHead
	if _, err := r.Events("rv", 1, head+1); !errors.As(err, &bh) || bh.Head != head {
		t.Fatalf("events past head: %v", err)
	}
}

// TestReviewTableRefusesALogThatDoesNotReplay: the replay gate refuses an
// input whose log does not reproduce under its Config, with the replay
// error, and registers nothing.
func TestReviewTableRefusesALogThatDoesNotReplay(t *testing.T) {
	t.Parallel()
	_, m := finishedTable(t)
	r := reviewRegistry(t)

	short := m.e.L.Clone()
	short.Events = short.Events[:len(short.Events)-1] // the recording claims fewer events than the game made
	wrongSeed := m.e.L.Clone()
	wrongSeed.Seed++ // a different deal: the first shuffle diverges
	for _, tc := range []struct {
		name string
		in   ReviewMatch
		div  bool // the refusal carries a *replay.Divergence
	}{
		{"truncated", ReviewMatch{Config: m.rulesConfig(), Log: short}, true},
		{"wrong seed", ReviewMatch{Config: m.rulesConfig(), Log: wrongSeed}, false},
	} {
		err := r.AddReviewTable("bad", tc.in)
		if err == nil || !strings.Contains(err.Error(), "does not replay") {
			t.Fatalf("%s: AddReviewTable = %v, want the replay refusal", tc.name, err)
		}
		var div *replay.Divergence
		if tc.div && !errors.As(err, &div) {
			t.Fatalf("%s: %v does not carry the replay divergence", tc.name, err)
		}
	}
	if got := r.Tables(); len(got) != 0 {
		t.Fatalf("a refused input registered a table: %+v", got)
	}
}

// TestReviewTableInputValidation covers the refusals that need no replay.
func TestReviewTableInputValidation(t *testing.T) {
	t.Parallel()
	_, m := finishedTable(t)
	r := reviewRegistry(t)
	ok := ReviewMatch{Config: m.rulesConfig(), Log: m.e.L.Clone()}
	if err := r.AddReviewTable("", ok); err == nil {
		t.Fatal("empty id accepted")
	}
	if err := r.AddReviewTable("x", ReviewMatch{Config: m.rulesConfig()}); err == nil {
		t.Fatal("nil log accepted")
	}
	if err := r.AddReviewTable("x", ok); err != nil {
		t.Fatal(err)
	}
	if err := r.AddReviewTable("x", ok); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("duplicate id: %v", err)
	}
	persisted, err := New(Options{LoadDeck: sampleLoader(t), Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer persisted.Close()
	if err := persisted.AddReviewTable("x", ok); err == nil || !strings.Contains(err.Error(), "persistence") {
		t.Fatalf("persisted registry: %v", err)
	}
}

// TestLiveTableInfoCarriesNoReviewBit pins the live side: an ordinary table
// never reports review, and the wire omits the key entirely, so a live
// /api/tables body is byte-identical to one from before the field existed.
func TestLiveTableInfoCarriesNoReviewBit(t *testing.T) {
	t.Parallel()
	live, _ := finishedTable(t)
	tis := live.Tables()
	if len(tis) != 1 || tis[0].Review {
		t.Fatalf("live tables %+v", tis)
	}
	b, err := json.Marshal(tis)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), `"review"`) {
		t.Fatalf("live table JSON carries a review key: %s", b)
	}
}
