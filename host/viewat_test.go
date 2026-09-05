package host

import (
	"encoding/json"
	"errors"
	"math/rand/v2"
	"slices"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/replay"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/view"
)

func viewJSON(t *testing.T, v view.View) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestBoundsOfMatchesTheLoopsOwnBookkeeping(t *testing.T) {
	t.Parallel()
	_, m := finishedTable(t)
	got := boundsOf(m.e.L.Events)
	if len(got) != len(m.bounds) {
		t.Fatalf("boundsOf found %d boundaries, the loop recorded %d", len(got), len(m.bounds))
	}
	for i := range got {
		if got[i] != m.bounds[i] {
			t.Fatalf("boundary %d: derived %d, recorded %d", i, got[i], m.bounds[i])
		}
	}
	// One snapshot at genesis plus one per burst that began a turn; a burst
	// can contain two turn changes only in degenerate games, so at most one
	// snapshot per turn start.
	if len(m.snaps) < 3 || len(m.snaps) > len(m.turnStarts) {
		t.Fatalf("%d snapshots for %d turn starts", len(m.snaps), len(m.turnStarts))
	}

	// FL-43 (fix round 1): a partial final burst — the shape a crashed
	// match's log has, since Submit records its intent before
	// handle/checkStateBased/Advance can panic on it — must not be
	// mistaken for a complete boundary. boundsOf's last element must stay
	// at the previous real boundary, not grow to the truncated length.
	l, k := truncatedMidBurst(t, m)
	partial := boundsOf(l.Events)
	if len(partial) != k+1 {
		t.Fatalf("partial burst: boundsOf found %d boundaries, want %d (one per completed intent, boundary 0 = genesis)", len(partial), k+1)
	}
	if partial[len(partial)-1] != m.bounds[k] {
		t.Fatalf("partial burst: boundsOf's last boundary is %d, want the previous real one %d — the truncated tail must not count",
			partial[len(partial)-1], m.bounds[k])
	}
}

// truncatedMidBurst returns a copy of m's log cut strictly inside some
// burst — after at least one of its events, before the DecisionAsk or
// GameOver that would complete it — plus k, the number of intents that
// fully completed before the truncated (k+1-th) one. This is the shape a
// crashed match's own log has (spec D15, FL-43): Submit appends the
// intent to L.Intents before handle/checkStateBased/Advance run, so a
// panic partway through leaves a straggler tail with no ask and no
// GameOver, and L.Intents one entry longer than any complete boundary
// accounts for.
func truncatedMidBurst(t *testing.T, m *match) (*events.Log, int) {
	t.Helper()
	for k := 0; k+1 < len(m.bounds); k++ {
		if m.bounds[k+1] > m.bounds[k]+1 {
			cut := m.bounds[k] + 1
			return &events.Log{
				Seed:    m.e.L.Seed,
				Events:  append([]events.Event(nil), m.e.L.Events[:cut]...),
				Intents: append([]decision.Intent(nil), m.e.L.Intents[:k+1]...),
			}, k
		}
	}
	t.Skip("fixture has no multi-event burst to truncate")
	return nil, 0
}

// forwardOracle reconstructs "the game as of seq" with none of host's
// snapshot/genesis machinery: one engine driven forward through the
// recorded intents exactly as replay.ReplayTo would (Submit per intent,
// boundary j == j intents applied), cloned and event-Applied only for the
// intra-burst tail. Asking it for seqs in ascending order costs ONE
// replay for the whole sample instead of one per seq — the version of this
// test that replayed from genesis per seq was 23s of a 57s package
// (2026-09-05) and had to cut its sample to stay under the -race timeout
// (Ruling FL-49). bounds are the loop's own bookkeeping (m.bounds), not
// boundsOf, so a boundsOf bug cannot hide from it.
type forwardOracle struct {
	t      *testing.T
	l      *events.Log
	bounds []uint64
	e      *rules.Engine
	at     int // intents applied to e so far
}

func newForwardOracle(t *testing.T, m *match) *forwardOracle {
	t.Helper()
	e, err := replay.ReplayTo(m.e.L, m.cfg, 0)
	if err != nil {
		t.Fatalf("oracle genesis: %v", err)
	}
	return &forwardOracle{t: t, l: m.e.L, bounds: m.bounds, e: e}
}

// viewAt returns the omniscient projection at seq. seq must not go
// backwards between calls.
func (o *forwardOracle) viewAt(seq uint64) view.View {
	o.t.Helper()
	j := 0
	for j+1 < len(o.bounds) && o.bounds[j+1] <= seq+1 {
		j++
	}
	if j < o.at {
		o.t.Fatalf("oracle asked to go backwards: boundary %d after %d", j, o.at)
	}
	for ; o.at < j; o.at++ {
		if err := o.e.Submit(o.l.Intents[o.at]); err != nil {
			o.t.Fatalf("oracle intent %d: %v", o.at, err)
		}
	}
	if got := uint64(len(o.e.L.Events)); got != o.bounds[j] {
		o.t.Fatalf("oracle: %d intents produced %d events, boundary %d is %d", j, got, j, o.bounds[j])
	}
	e := o.e
	if seq+1 > o.bounds[j] {
		e = o.e.Clone()
		for s := o.bounds[j]; s <= seq; s++ {
			events.Apply(e.G, o.l.Events[s])
		}
	}
	return view.ProjectFor(e.G, e, view.NoSeat, view.Omniscient, nil)
}

func TestViewAtFromSnapshotsEqualsViewAtFromGenesis(t *testing.T) {
	t.Parallel()
	_, m := finishedTable(t)
	head := uint64(len(m.e.L.Events) - 1)
	rng := rand.New(rand.NewPCG(1, 2))
	seqs := []uint64{0, m.bounds[0] - 1, m.bounds[0], head}
	// The oracle side is one replay however many seqs are asked; the
	// snapshot side is a snapshot clone + partial-turn replay PER seq, so
	// the sample is what bounds this test's cost (every boundary, ~1000
	// seqs, ran 99s). 24 random + every other turn start and its successor
	// + 12 boundary-aligned seqs.
	for i := 0; i < 24; i++ {
		seqs = append(seqs, uint64(rng.IntN(int(head)+1)))
	}
	for i := 0; i < len(m.turnStarts); i += 2 {
		seqs = append(seqs, m.turnStarts[i], m.turnStarts[i]+1)
	}
	step := len(m.bounds) / 12
	if step < 1 {
		step = 1
	}
	for j := 0; j < len(m.bounds); j += step {
		seqs = append(seqs, m.bounds[j]-1)
	}
	slices.Sort(seqs)
	seqs = slices.Compact(seqs)

	// Independent oracle at every sampled seq — boundary-aligned and
	// intra-burst alike. This is the check that catches a bug shared by
	// both of viewAt's own paths (an off-by-one in boundsOf or the j/from
	// selection), which comparing them only against each other cannot.
	oracle := newForwardOracle(t, m)
	for _, seq := range seqs {
		if seq > head {
			continue
		}
		fast, err := viewAt(m.cfg, m.e.L, m.snaps, seq, view.NoSeat, view.Omniscient, nil)
		if err != nil {
			t.Fatalf("seq %d (snapshots): %v", seq, err)
		}
		if a, b := viewJSON(t, fast), viewJSON(t, oracle.viewAt(seq)); a != b {
			t.Fatalf("seq %d: snapshot path differs from the forward oracle\n%s\n%s", seq, a, b)
		}
	}

	// viewAt's own snapshot-less branch (a full replay per call) still has
	// to agree; three seqs cover its from==j==0, mid-log, and head cases.
	for _, seq := range []uint64{0, seqs[len(seqs)/2], head} {
		fast, err := viewAt(m.cfg, m.e.L, m.snaps, seq, view.NoSeat, view.Omniscient, nil)
		if err != nil {
			t.Fatalf("seq %d (snapshots): %v", seq, err)
		}
		slow, err := viewAt(m.cfg, m.e.L, nil, seq, view.NoSeat, view.Omniscient, nil)
		if err != nil {
			t.Fatalf("seq %d (genesis): %v", seq, err)
		}
		if a, b := viewJSON(t, fast), viewJSON(t, slow); a != b {
			t.Fatalf("seq %d: snapshot path differs from full replay\n%s\n%s", seq, a, b)
		}
	}
}

func TestViewAtHeadEqualsTheLiveProjection(t *testing.T) {
	t.Parallel()
	r, m := finishedTable(t)
	head := uint64(len(m.e.L.Events) - 1)
	got, err := r.ViewAt("t1", 1, head)
	if err != nil {
		t.Fatal(err)
	}
	want := view.ProjectFor(m.e.G, m.e, view.NoSeat, view.Omniscient, nil)
	if viewJSON(t, got) != viewJSON(t, want) {
		t.Fatal("ViewAt(head) differs from projecting the live engine")
	}
	if got.Over != true {
		t.Fatal("finished match's head view is not Over")
	}
}

func TestViewAtTracksIntraBurstStateChanges(t *testing.T) {
	t.Parallel()
	_, m := finishedTable(t)
	// Find a LifeChange or Damage-to-player event and check the life total
	// moves at exactly that seq — inside a burst, not at its boundary.
	for _, ev := range m.e.L.Events {
		if ev.Kind != events.LifeChange || ev.Amount == 0 {
			continue
		}
		before, err := viewAt(m.cfg, m.e.L, m.snaps, ev.Seq-1, view.NoSeat, view.Omniscient, nil)
		if err != nil {
			t.Fatal(err)
		}
		after, err := viewAt(m.cfg, m.e.L, m.snaps, ev.Seq, view.NoSeat, view.Omniscient, nil)
		if err != nil {
			t.Fatal(err)
		}
		if after.Players[ev.Player].Life-before.Players[ev.Player].Life != ev.Amount {
			t.Fatalf("seq %d: life moved %d, event says %d", ev.Seq,
				after.Players[ev.Player].Life-before.Players[ev.Player].Life, ev.Amount)
		}
		return
	}
	t.Skip("fixture produced no life change")
}

func TestViewAtAndEventsErrors(t *testing.T) {
	t.Parallel()
	r, m := finishedTable(t)
	head := uint64(len(m.e.L.Events) - 1)
	_, err := r.ViewAt("t1", 1, head+1)
	var beyond ErrBeyondHead
	if !errors.As(err, &beyond) || beyond.Head != head {
		t.Fatalf("beyond head: %v", err)
	}
	if _, err := r.ViewAt("t9", 1, 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown table: %v", err)
	}
	if _, err := r.ViewAt("t1", 2, 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown match: %v", err)
	}
	if _, err := r.Events("t1", 1, head+1); !errors.As(err, &beyond) {
		t.Fatalf("events beyond head: %v", err)
	}
}

// TestViewAtSurvivesATruncatedFinalBurst is FL-43's regression test: a
// log shaped like a crashed match's (a straggler tail from the last,
// unfinished burst — see truncatedMidBurst) must not make viewAt(head)
// resubmit the unfinished intent, whether or not it panics for real.
// Fixed, this resolves to a clean view built from the previous boundary
// plus events.Apply of the tail — via a real snapshot AND via a from-
// genesis replay.ReplayTo, which must agree with each other too.
func TestViewAtSurvivesATruncatedFinalBurst(t *testing.T) {
	t.Parallel()
	_, m := finishedTable(t)
	l, k := truncatedMidBurst(t, m)
	head := uint64(len(l.Events) - 1)

	fromSnaps, err := viewAt(m.cfg, l, m.snaps, head, view.NoSeat, view.Omniscient, nil)
	if err != nil {
		t.Fatalf("viewAt(head) with snapshots panicked or errored: %v", err)
	}
	fromGenesis, err := viewAt(m.cfg, l, nil, head, view.NoSeat, view.Omniscient, nil)
	if err != nil {
		t.Fatalf("viewAt(head) from genesis panicked or errored: %v", err)
	}
	if a, b := viewJSON(t, fromSnaps), viewJSON(t, fromGenesis); a != b {
		t.Fatalf("truncated burst: snapshot path differs from genesis replay\n%s\n%s", a, b)
	}
	if fromSnaps.Over {
		t.Fatal("a truncated mid-burst log is never a finished game")
	}

	// The unfinished intent (l.Intents[k]) must never have been resubmitted:
	// confirm by reconstructing the same state the hard way, from the
	// previous complete boundary plus a direct events.Apply of the one
	// straggler event — no Submit involved anywhere past boundary k.
	base, err := replay.ReplayTo(l, m.cfg, k)
	if err != nil {
		t.Fatalf("replay to the last complete boundary: %v", err)
	}
	if uint64(len(base.L.Events)) != m.bounds[k] {
		t.Fatalf("replay to boundary %d produced %d events, want %d", k, len(base.L.Events), m.bounds[k])
	}
	for s := m.bounds[k]; s <= head; s++ {
		events.Apply(base.G, l.Events[s])
	}
	want := view.ProjectFor(base.G, base, view.NoSeat, view.Omniscient, nil)
	if a, b := viewJSON(t, fromSnaps), viewJSON(t, want); a != b {
		t.Fatalf("truncated burst: viewAt differs from the manual apply-only reconstruction\n%s\n%s", a, b)
	}
}

// TestViewAtAndEventsDuringALiveMatch exercises ViewAt/Events from a
// second goroutine while the match loop is still writing to the same
// match — the concurrency -race is meant to catch. finishedTable (every
// other test here) only reads after Wait, so on its own -race never sees
// a reader overlap a writer.
func TestViewAtAndEventsDuringALiveMatch(t *testing.T) {
	t.Parallel()
	r, _ := New(testOptions(t))
	t.Cleanup(func() { r.Close() })
	if err := r.AddTable(fourSeatTable("t1", false)); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		var seq uint64
		for {
			select {
			case <-stop:
				return
			default:
			}
			switch _, err := r.ViewAt("t1", 1, seq); {
			case err == nil:
				seq++
			case errors.As(err, new(ErrBeyondHead)), errors.Is(err, ErrNotFound):
				// Not ready yet, or we ran past a head that keeps moving.
			default:
				t.Errorf("ViewAt(%d): %v", seq, err)
				return
			}
			if _, err := r.Events("t1", 1, 0); err != nil &&
				!errors.As(err, new(ErrBeyondHead)) && !errors.Is(err, ErrNotFound) {
				t.Errorf("Events: %v", err)
				return
			}
		}
	}()
	r.Wait("t1")
	close(stop)
	wg.Wait()
}

// TestViewAtAndEventsRedactForAPublicTable is the redaction negative for
// the Public (no-seat) spectator visibility, mirroring
// TestEventsSinceReturnsRedactedDescribedTail's Omniscient/shuffle check:
// a Public ViewAt never carries a hand, and a Public Events never carries
// the Obj of a hidden-to-hidden move (view.RedactEvents rule 2).
func TestViewAtAndEventsRedactForAPublicTable(t *testing.T) {
	t.Parallel()
	r, _ := New(testOptions(t))
	t.Cleanup(func() { r.Close() })
	cfg := fourSeatTable("t1", false)
	cfg.Spectator = view.Public
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")
	r.mu.RLock()
	m := r.tables["t1"].history[0]
	r.mu.RUnlock()
	head := uint64(len(m.e.L.Events) - 1)

	v, err := r.ViewAt("t1", 1, head)
	if err != nil {
		t.Fatal(err)
	}
	if v.Visibility != view.Public.String() {
		t.Fatalf("visibility %q, want %q", v.Visibility, view.Public.String())
	}
	for _, p := range v.Players {
		if p.Hand != nil {
			t.Fatalf("public ViewAt exposes seat %d's hand: %v", p.ID, p.Hand)
		}
	}

	bodies, err := r.Events("t1", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	strippedAny := false
	for _, b := range bodies {
		if b.Event.Kind == "draw" && b.Event.Obj == 0 {
			strippedAny = true
		}
	}
	if !strippedAny {
		t.Fatal("public Events never stripped a draw's Obj — fixture has no hidden-to-hidden move to check")
	}
}

func TestEventsSinceReturnsRedactedDescribedTail(t *testing.T) {
	t.Parallel()
	r, m := finishedTable(t)
	head := uint64(len(m.e.L.Events) - 1)
	all, err := r.Events("t1", 1, 0)
	if err != nil || len(all) != int(head)+1 || all[0].Event.Seq != 0 || all[len(all)-1].Event.Seq != head {
		t.Fatalf("Events(0): %d bodies, %v", len(all), err)
	}
	tail, _ := r.Events("t1", 1, head-4)
	if len(tail) != 5 || tail[0].Event.Seq != head-4 {
		t.Fatalf("Events(head-4): %d bodies from %d", len(tail), tail[0].Event.Seq)
	}
	for _, b := range all {
		if b.Event.Kind == "shuffle" && len(b.Event.IDs) != 0 {
			t.Fatal("Events leaks library order")
		}
		if b.Line == "" && b.Event.Kind != "clock_tick" {
			t.Fatalf("seq %d (%s) has no line", b.Event.Seq, b.Event.Kind)
		}
	}
	_ = protocol.TEvent
}
