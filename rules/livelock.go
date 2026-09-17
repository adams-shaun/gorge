package rules

import (
	"encoding/binary"
	"fmt"
	"hash"
	"hash/fnv"
	"strings"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Livelock detection: a real bug -- I-1's stack object that re-resolved
// forever, and the rakdos-muscle-scam-exe run that pegged a core for 15+
// minutes with no output -- presents to a harness as a game that never
// returns from Advance/Submit. An outside `timeout` cannot tell "stuck"
// from "slow" and reports nothing about WHERE the loop is, so the engine
// itself watches its own event stream and aborts (panics with a
// *LivelockError) the moment the stream looks non-terminating.
//
// The watcher is pure observation: it reads only the events the engine
// already logged, holds no state events depend on, and never emits an
// event, so a game that does not trip it is byte-identical to an
// un-watched one (the chain-head goldens in rules/heads_test.go pin that).
// A game that DOES trip it is a bug by definition -- the harnesses
// (cmd/mtgsim, cmd/botbench) recover the panic, report the diagnostic
// against that game's seed/deck, and move on; anywhere else the panic is
// the bug surfacing, exactly as the determinism invariant intends.
//
// Two independent triggers, because two loop shapes evade each other:
//
//   - Repeating cycle: the trailing window of event signatures repeats a
//     short period exactly. This catches I-1's shape (the same 13-event
//     resolve cycle over and over) and stays precise enough to report the
//     cycle itself. Signatures deliberately EXCLUDE Seq, Amount and Text:
//     a stuck loop whose payload drifts a little each iteration (a life
//     total ticking down, a counter creeping) must still be caught, while
//     zone/object/step identity still separates genuinely different
//     events.
//
//   - Runaway resolution: this many events with no DecisionAsk, StepChange
//     or TurnChange among them. This is the catch-all for loops whose
//     cycle grows (a new object minted every iteration, a period past
//     MaxPeriod) that the exact-period detector cannot lock onto. Every
//     legitimate game relents and asks a decision or moves to the next
//     step far inside this bound (measured: max quiet run over the
//     acceptance games is three orders of magnitude smaller).

const (
	// defaultCycleEvents is how many consecutive events a repeating cycle
	// must run before the watcher aborts. I-1's loop ran 700+ events in a
	// 13-event cycle before anyone noticed; 400 trips that inside one
	// Submit call, in well under a second, and is far past anything a
	// legitimate resolution emits.
	defaultCycleEvents = 400
	// defaultMaxPeriod is the longest cycle the exact-period detector
	// tracks. A stuck loop longer than this falls to the runaway trigger.
	defaultMaxPeriod = 128
	// defaultRunawayEvents is the no-progress backstop. It exists to be
	// never-false-positive, not tight: the runaway diagnostic still names
	// the span and the last event, which is where a debugger looks.
	defaultRunawayEvents = 50000
)

// progressKinds are the events that prove the game is still moving: a
// decision was asked (control returned to a seat), or the turn/step
// structure advanced. Every other event can repeat inside one resolution.
var progressKinds = map[events.Kind]bool{
	events.DecisionAsk: true,
	events.StepChange:  true,
	events.TurnChange:  true,
}

// LoopGuard overrides the livelock watcher's thresholds for one game. Zero
// fields fall back to the defaults above; a nil *LoopGuard in Config is
// the defaults, so every existing Config is unchanged. A RunawayEvents of
// 0 explicitly means DEFAULT here (the zero-value-falls-back rule), not
// "off" -- the backstop is what catches a loop the period detector cannot
// see, so it is never disabled.
type LoopGuard struct {
	CycleEvents   int
	MaxPeriod     int
	RunawayEvents int
}

func (g *LoopGuard) filled() LoopGuard {
	out := LoopGuard{CycleEvents: defaultCycleEvents, MaxPeriod: defaultMaxPeriod, RunawayEvents: defaultRunawayEvents}
	if g != nil {
		if g.CycleEvents > 0 {
			out.CycleEvents = g.CycleEvents
		}
		if g.MaxPeriod > 0 {
			out.MaxPeriod = g.MaxPeriod
		}
		if g.RunawayEvents > 0 {
			out.RunawayEvents = g.RunawayEvents
		}
	}
	if out.MaxPeriod < 1 {
		out.MaxPeriod = 1
	}
	return out
}

// LivelockError is the panic value the watcher aborts a stuck game with,
// and the diagnostic the harnesses print. It is deliberately a struct (not
// a string) so a harness can record the pieces (object, kind, repeat
// count, cycle length) structurally as well as render the text.
type LivelockError struct {
	// Reason is "repeating cycle" or "runaway resolution" -- which of the
	// two triggers fired.
	Reason string
	// Kind and Object identify the repeating event shape: for a cycle, the
	// first event of one period; for a runaway, the last event emitted.
	Kind   events.Kind
	Object state.ObjID
	// Repeats is how many full cycles the detector counted (0 for a
	// runaway); CycleLen is the period length in events (0 for a runaway).
	Repeats  int
	CycleLen int
	// FirstSeq and LastSeq bound the stuck span in the event log.
	FirstSeq uint64
	LastSeq  uint64
	// QuietEvents is the no-progress run length (0 for a period trip).
	QuietEvents int
	// Cycle renders one period of the repeating events (nil for a runaway),
	// earliest first.
	Cycle []string
}

func (e *LivelockError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "livelock detected (%s): object %d, kind %s, ", e.Reason, e.Object, e.Kind)
	switch e.Reason {
	case "repeating cycle":
		fmt.Fprintf(&b, "cycle of %d event(s) repeated %d time(s), events %d-%d", e.CycleLen, e.Repeats, e.FirstSeq, e.LastSeq)
	default:
		fmt.Fprintf(&b, "%d event(s) with no decision, step or turn change, events %d-%d", e.QuietEvents, e.FirstSeq, e.LastSeq)
	}
	if len(e.Cycle) > 0 {
		fmt.Fprintf(&b, "; cycle: [%s]", strings.Join(e.Cycle, "; "))
	}
	return b.String()
}

// livelockWatcher is the per-engine observer state. Plain values and
// capped slices only, so Clone can carry or reset it cheaply; it is
// written by observe during an emission burst and read by nothing else.
type livelockWatcher struct {
	guard LoopGuard
	// sigs is the trailing signature window, newest last, capped at
	// 2*MaxPeriod (the two halves the period detector compares).
	sigs []uint64
	// recent is the trailing event window, newest last, capped at
	// MaxPeriod, so an abort can render one real period.
	recent []events.Event
	// runPeriod/runEvents track the active periodic run: runEvents counts
	// the consecutive events (>= 2*runPeriod at detection) matching the
	// runPeriod period.
	runPeriod int
	runEvents int
	// quiet/quietSince track the no-progress run for the runaway backstop.
	quiet      int
	quietSince uint64
}

func newLivelockWatcher(g *LoopGuard) livelockWatcher {
	return livelockWatcher{guard: g.filled()}
}

// newLivelockWatcherFromGuard is Clone's constructor: a fresh watcher over
// thresholds the source engine was already running (Clone's doc above).
func newLivelockWatcherFromGuard(g LoopGuard) livelockWatcher {
	return livelockWatcher{guard: g}
}

// observe feeds one just-logged event to the watcher. It panics with a
// *LivelockError when either trigger fires; every other return leaves the
// game byte-identical to an un-watched one.
func (w *livelockWatcher) observe(ev events.Event) {
	sig := eventSignature(ev)
	w.sigs = append(w.sigs, sig)
	if len(w.sigs) > 2*w.guard.MaxPeriod {
		w.sigs = w.sigs[1:]
	}
	w.recent = append(w.recent, ev)
	if len(w.recent) > w.guard.MaxPeriod {
		w.recent = w.recent[len(w.recent)-w.guard.MaxPeriod:]
	}

	// Runaway backstop: count the events since the last progress event.
	if progressKinds[ev.Kind] {
		w.quiet = 0
	} else {
		if w.quiet == 0 {
			w.quietSince = ev.Seq
		}
		w.quiet++
	}
	if w.quiet > w.guard.RunawayEvents {
		panic(&LivelockError{
			Reason:      "runaway resolution",
			Kind:        ev.Kind,
			Object:      ev.Obj,
			FirstSeq:    w.quietSince,
			LastSeq:     ev.Seq,
			QuietEvents: w.quiet,
		})
	}

	// Exact-period detector. While a run is active, each event that
	// continues the period extends it; any event that breaks the pattern
	// ends the run and the trailing window is re-scanned for a fresh one.
	if w.runPeriod > 0 {
		n := len(w.sigs)
		if n > w.runPeriod && w.sigs[n-1] == w.sigs[n-1-w.runPeriod] {
			w.runEvents++
			if w.runEvents >= w.guard.CycleEvents {
				w.abort()
			}
			return
		}
		w.runPeriod, w.runEvents = 0, 0
	}
	w.detect()
}

// detect scans for the shortest period p whose trailing 2p signatures are
// two identical halves, and if one is found, opens a run on it.
func (w *livelockWatcher) detect() {
	n := len(w.sigs)
	maxP := w.guard.MaxPeriod
	if lim := n / 2; lim < maxP {
		maxP = lim
	}
	for p := 1; p <= maxP; p++ {
		ok := true
		for j := n - 1; j >= n-2*p+1; j-- {
			if w.sigs[j] != w.sigs[j-p] {
				ok = false
				break
			}
		}
		if ok {
			w.runPeriod, w.runEvents = p, 2*p
			if w.runEvents >= w.guard.CycleEvents {
				w.abort()
			}
			return
		}
	}
}

// abort panics with the diagnostic for the active run. The rendered cycle
// is the last runPeriod real events -- exactly one period, however many
// times it has repeated.
func (w *livelockWatcher) abort() {
	p := w.runPeriod
	first := w.recent[len(w.recent)-p]
	cycle := make([]string, 0, p)
	for _, ev := range w.recent[len(w.recent)-p:] {
		cycle = append(cycle, describeEvent(ev))
	}
	panic(&LivelockError{
		Reason:   "repeating cycle",
		Kind:     first.Kind,
		Object:   first.Obj,
		Repeats:  w.runEvents / p,
		CycleLen: p,
		FirstSeq: first.Seq,
		LastSeq:  w.recent[len(w.recent)-1].Seq,
		Cycle:    cycle,
	})
}

// eventSignature is the watcher's per-event fingerprint: everything that
// identifies the event's shape (kind, seat, object, zone/step movement,
// counter name, secret bit, id and pair payloads), and deliberately NOT
// Seq (every event has a fresh one), Amount and Text (a stuck loop whose
// payload drifts must still be caught). Length-prefixing keeps adjacent
// fields unambiguous.
func eventSignature(ev events.Event) uint64 {
	h := fnv.New64a()
	var b [5]byte
	b[0] = byte(ev.Kind)
	b[1] = byte(ev.Player)
	b[2] = byte(ev.From)
	b[3] = byte(ev.To)
	b[4] = byte(ev.Step)
	h.Write(b[:])
	if ev.Secret {
		h.Write([]byte{1})
	} else {
		h.Write([]byte{0})
	}
	var u4 [4]byte
	binary.LittleEndian.PutUint32(u4[:], uint32(ev.Obj))
	h.Write(u4[:])
	sigStr(h, ev.Counter)
	binary.LittleEndian.PutUint32(u4[:], uint32(len(ev.IDs)))
	h.Write(u4[:])
	for _, id := range ev.IDs {
		binary.LittleEndian.PutUint32(u4[:], uint32(id))
		h.Write(u4[:])
	}
	binary.LittleEndian.PutUint32(u4[:], uint32(len(ev.Pairs)))
	h.Write(u4[:])
	for _, pr := range ev.Pairs {
		binary.LittleEndian.PutUint32(u4[:], uint32(pr[0]))
		h.Write(u4[:])
		binary.LittleEndian.PutUint32(u4[:], uint32(pr[1]))
		h.Write(u4[:])
	}
	return h.Sum64()
}

func sigStr(h hash.Hash64, s string) {
	var u4 [4]byte
	binary.LittleEndian.PutUint32(u4[:], uint32(len(s)))
	h.Write(u4[:])
	h.Write([]byte(s))
}

// describeEvent renders one event for the diagnostic: compact, stable, and
// enough to name the object and what was being done to it.
func describeEvent(ev events.Event) string {
	s := fmt.Sprintf("seq %d %s", ev.Seq, ev.Kind)
	if ev.Obj != 0 {
		s += fmt.Sprintf(" obj=%d", ev.Obj)
	}
	if ev.Player != 0 || ev.Obj == 0 {
		s += fmt.Sprintf(" player=%d", ev.Player)
	}
	if ev.Text != "" {
		t := ev.Text
		if len(t) > 80 {
			t = t[:80] + "..."
		}
		s += fmt.Sprintf(" %q", t)
	}
	return s
}
