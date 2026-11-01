package host

// Provenance ledger acceptance tests
// (docs/design/2026-10-10-human-divergence-design.md §5.2):
// the caretaker bit follows HumanSeat.caretakerFires, the auto bit follows
// SubmitIntentFrom's flag and the ?auto=1 query form, an undo truncates the
// ledger beside the log, and the ledger never reaches replay input (two
// identical matches, one with auto marks, replay to the same chain head).
//
// Every test asserts the PRECONDITION the mark depends on before reading the
// ledger: the caretaker really fired (its counter moved) or the flag really
// rode a submit (the mark is present before the property under test is
// checked), so a vacuous green run cannot hide a broken mark.

import (
	"testing"
	"time"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/server/protocol"
)

// driveLedgerMatch answers every seat-0 decision of the live match with
// action_test's legalIntent, flagging every autoEvery-th submit with the
// client-auto bit, until the match reaches MatchFinished. It fails on a
// crash and returns the submit count and the count of auto-flagged submits.
// The driver shape is driveHumanSeat's (human_match_test.go): each distinct
// pending Seq is submitted exactly once.
func driveLedgerMatch(t *testing.T, r *Registry, id TableID, autoEvery int) (submits, autos int) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	var last uint64
	var answered bool
	for i := 0; ; i++ {
		ms, err := r.Matches(id)
		if err != nil {
			r.Close()
			t.Fatalf("Matches: %v", err)
		}
		for _, mi := range ms {
			if mi.State == protocol.MatchCrashed {
				r.Close()
				t.Fatalf("match crashed: head %s after %d events", mi.Head, mi.Events)
			}
		}
		if len(ms) > 0 && ms[len(ms)-1].State == protocol.MatchFinished {
			return submits, autos
		}
		if d, err := r.Pending(id, 1, 0); err == nil && (!answered || d.Seq != last) {
			auto := autoEvery > 0 && i%autoEvery == 0
			if err := r.SubmitIntentFrom(id, 1, 0, legalIntent(d), auto); err != nil {
				r.Close()
				t.Fatalf("SubmitIntentFrom(seq %d): %v", d.Seq, err)
			}
			last, answered = d.Seq, true
			submits++
			if auto {
				autos++
			}
		}
		if time.Now().After(deadline) {
			r.Close()
			t.Fatal("human-driven ledger match did not finish within deadline")
		}
		time.Sleep(time.Millisecond)
	}
}

// ledgerProvenance is Provenance with the test's failure attached.
func ledgerProvenance(t *testing.T, r *Registry, id TableID) []uint8 {
	t.Helper()
	prov, err := r.Provenance(id, 1)
	if err != nil {
		t.Fatalf("Provenance: %v", err)
	}
	return prov
}

// ledgerIntents snapshots the finished-or-live match's logged intents. A
// finished non-perpetual match is the table's history tail (t.cur is nil
// once run() retires it), so the lookup tries cur first, then history.
func ledgerIntents(t *testing.T, r *Registry, id TableID) []decision.Intent {
	t.Helper()
	m := ledgerMatch(t, r, id)
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]decision.Intent(nil), m.e.L.Intents...)
}

// ledgerMatch returns the table's current match, or — once it has finished
// and run() retired it — the history tail, failing the test if there is
// neither.
func ledgerMatch(t *testing.T, r *Registry, id TableID) *match {
	t.Helper()
	r.mu.RLock()
	tb := r.tables[id]
	r.mu.RUnlock()
	if tb == nil {
		t.Fatalf("no table %s", id)
	}
	tb.mu.RLock()
	m := tb.cur
	hist := tb.history
	tb.mu.RUnlock()
	if m != nil {
		return m
	}
	if len(hist) > 0 {
		return hist[len(hist)-1]
	}
	t.Fatalf("table %s has neither a current match nor history", id)
	return nil
}

// ledgerHumanSeat waits for the match's seat-0 HumanSeat to be installed.
func ledgerHumanSeat(t *testing.T, r *Registry, id TableID) *HumanSeat {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		m := ledgerMatch(t, r, id)
		m.mu.RLock()
		slots := m.slots
		m.mu.RUnlock()
		if len(slots) > 0 {
			if hs, ok := slots[0].(*HumanSeat); ok {
				return hs
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("seat 0 was not installed as a HumanSeat within deadline")
		}
		time.Sleep(time.Millisecond)
	}
}

// TestLedgerAutoFlag: an answer submitted with the auto flag is marked
// provAuto in the ledger; the same answer without the flag is not. The
// caretaker never fires in this test (ThinkTimeout
// 0), so the caretaker bit must stay clear everywhere — a marked caretaker
// bit here would mean the bits are cross-contaminated.
func TestLedgerAutoFlag(t *testing.T) {
	t.Parallel()
	o := testOptions(t)
	r := undoTable(t, o, "t1")
	submits, autos := driveLedgerMatch(t, r, "t1", 2)
	if submits == 0 || autos == 0 || autos == submits {
		t.Fatalf("fixture degenerate: %d submits, %d auto-flagged (want both kinds present)", submits, autos)
	}
	if got := ledgerHumanSeat(t, r, "t1").caretakerCount(); got != 0 {
		t.Fatalf("caretaker fired %d times in a no-timeout match; the auto marks are not attributable to the flag", got)
	}
	intents := ledgerIntents(t, r, "t1")
	prov := ledgerProvenance(t, r, "t1")
	if len(prov) != len(intents) {
		t.Fatalf("ledger holds %d entries for %d logged intents", len(prov), len(intents))
	}
	autoMarked := 0
	for i, b := range prov {
		if b&provAuto != 0 {
			autoMarked++
			if intents[i].Player != 0 {
				t.Fatalf("intent %d (seat %d) carries the auto bit; only seat-0 submits were flagged", i, intents[i].Player)
			}
		}
		if b&provCaretaker != 0 {
			t.Fatalf("intent %d carries the caretaker bit; no caretaker ever fired", i)
		}
	}
	if autoMarked != autos {
		t.Fatalf("ledger marks %d auto intents, driver flagged %d submits", autoMarked, autos)
	}
}

// TestLedgerMarksCaretaker: with a ThinkTimeout armed, the first two seat-0
// decisions are never submitted — the caretaker turns them — and exactly
// those two intents carry the caretaker bit; every intent the human
// submitted afterwards is unmarked. No
// submit is ever auto-flagged, so the auto bit must stay clear.
func TestLedgerMarksCaretaker(t *testing.T) {
	t.Parallel()
	const skipN = 2
	o := testOptions(t)
	// 2s: far longer than one driver iteration even on a saturated box, so a
	// submitted decision can never lose the race to the timeout — a fire the
	// driver caused would break the exact count below. 200ms lost that race
	// at load ~60 (4 fires, want 2);
	// only the two withheld decisions wait it out, so the test pays ~4s.
	o.ThinkTimeout = 2 * time.Second
	r := undoTable(t, o, "t1")
	deadline := time.Now().Add(60 * time.Second)
	skipped := map[uint64]bool{}
	var last uint64
	var answered bool
	submits := 0
	for {
		ms, err := r.Matches("t1")
		if err != nil {
			r.Close()
			t.Fatalf("Matches: %v", err)
		}
		for _, mi := range ms {
			if mi.State == protocol.MatchCrashed {
				r.Close()
				t.Fatalf("match crashed: head %s after %d events", mi.Head, mi.Events)
			}
		}
		if len(ms) > 0 && ms[len(ms)-1].State == protocol.MatchFinished {
			break
		}
		if d, derr := r.Pending("t1", 1, 0); derr == nil && (!answered || d.Seq != last) {
			if len(skipped) < skipN {
				skipped[d.Seq] = true
			} else if !skipped[d.Seq] {
				if serr := r.SubmitIntent("t1", 1, 0, legalIntent(d)); serr != nil {
					r.Close()
					t.Fatalf("SubmitIntent(seq %d): %v", d.Seq, serr)
				}
				submits++
			}
			last, answered = d.Seq, true
		}
		if time.Now().After(deadline) {
			r.Close()
			t.Fatal("caretaker-led match did not finish within deadline")
		}
		time.Sleep(time.Millisecond)
	}
	hs := ledgerHumanSeat(t, r, "t1")
	if got := hs.caretakerCount(); got != skipN {
		t.Fatalf("caretaker fired %d times, want exactly the %d withheld decisions", got, skipN)
	}
	intents := ledgerIntents(t, r, "t1")
	prov := ledgerProvenance(t, r, "t1")
	if len(prov) != len(intents) {
		t.Fatalf("ledger holds %d entries for %d logged intents", len(prov), len(intents))
	}
	if submits == 0 {
		t.Fatal("the human never submitted; the unmarked half of the assertion is vacuous")
	}
	// The seat-0 intents in log order are: the withheld (caretaker) ones
	// first — they are the seat's first decisions — then the submitted ones.
	// Exactly the first skipN of them carry the caretaker bit, nothing else
	// anywhere in the ledger does.
	human := 0
	for i, in := range intents {
		if in.Player != 0 {
			if prov[i]&provCaretaker != 0 || prov[i]&provAuto != 0 {
				t.Fatalf("intent %d of seat %d is marked %02x; only withheld seat-0 answers may be marked", i, in.Player, prov[i])
			}
			continue
		}
		human++
		want := uint8(0)
		if human <= skipN {
			want = provCaretaker
		}
		if prov[i] != want {
			t.Fatalf("seat-0 intent #%d (index %d) marked %02x, want %02x", human-1, i, prov[i], want)
		}
	}
	if human <= skipN {
		t.Fatalf("only %d seat-0 intents; the %d withheld decisions never logged", human, skipN)
	}
}

// TestLedgerTruncatesOnUndo: the ledger describes exactly the intents the
// log holds. Two undos each truncate every entry at or past the rewind
// point — and the second one demonstrably drops a MARKED entry: the
// re-answered decision carries the auto bit before the undo and is gone
// from the ledger after it, while the marked intent inside the kept prefix
// survives both rewinds byte for byte.
func TestLedgerTruncatesOnUndo(t *testing.T) {
	t.Parallel()
	o := testOptions(t)
	rewound := make(chan int, 2)
	o.OnRewind = func(_ TableID, _ int, n int, _ uint64) error { rewound <- n; return nil }
	r := undoTable(t, o, "t1")

	// Intent 0-or-so: the human's first action, flagged auto — a marked
	// entry deep enough to survive the rewinds. The re-derived first-Min
	// answer must be the same for both seats' matches; flagging it changes
	// nothing about the intent itself.
	d := waitNextPending(t, r, "t1", 1, 0, 0)
	if err := r.SubmitIntentFrom("t1", 1, 0, legalIntent(d), true); err != nil {
		t.Fatalf("SubmitIntentFrom(seq %d, auto): %v", d.Seq, err)
	}
	waitPastSeq(t, r, "t1", d.Seq)
	d2 := waitNextPending(t, r, "t1", 1, 0, d.Seq)
	if err := r.SubmitIntent("t1", 1, 0, legalIntent(d2)); err != nil {
		t.Fatalf("SubmitIntent(seq %d): %v", d2.Seq, err)
	}
	waitPastSeq(t, r, "t1", d2.Seq)
	waitIntents(t, r, "t1", 3)

	intents := ledgerIntents(t, r, "t1")
	provBefore := ledgerProvenance(t, r, "t1")
	if len(provBefore) != len(intents) {
		t.Fatalf("ledger holds %d entries for %d logged intents", len(provBefore), len(intents))
	}
	if len(provBefore) < 3 {
		t.Fatalf("only %d logged intents; there is no tail to truncate", len(provBefore))
	}
	markedIdx := -1
	for i, in := range intents {
		if in.Player == 0 && markedIdx < 0 {
			markedIdx = i
		}
	}
	if markedIdx < 0 || provBefore[markedIdx]&provAuto == 0 {
		t.Fatalf("no auto-flagged intent in the kept prefix (marked %d, entry %02x): the flag never rode a submit", markedIdx, provBefore[markedIdx])
	}

	if err := r.Undo("t1", 1, 0); err != nil {
		t.Fatalf("first Undo: %v", err)
	}
	n := waitRewind(t, rewound)
	provAfter := ledgerProvenance(t, r, "t1")
	if len(provAfter) != n {
		t.Fatalf("after the first undo the ledger holds %d entries, want %d (the rewind point)", len(provAfter), n)
	}
	for i := range provAfter {
		if provAfter[i] != provBefore[i] {
			t.Fatalf("kept entry %d changed across the undo: %02x, want %02x", i, provAfter[i], provBefore[i])
		}
	}
	if provAfter[markedIdx]&provAuto == 0 {
		t.Fatalf("the marked intent inside the kept prefix (index %d) lost its auto bit", markedIdx)
	}

	// Second pass: the rewound decision is pending again; re-answer it WITH the
	// auto flag, let the bots move once, and undo again — this rewind
	// truncates a ledger entry that is currently MARKED.
	d3 := waitPendingSeq(t, r, "t1", d2.Seq)
	if err := r.SubmitIntentFrom("t1", 1, 0, legalIntent(d3), true); err != nil {
		t.Fatalf("SubmitIntentFrom(seq %d, auto): %v", d3.Seq, err)
	}
	waitPastSeq(t, r, "t1", d3.Seq)
	nIntents := waitIntents(t, r, "t1", n+2)
	provMid := ledgerProvenance(t, r, "t1")
	if len(provMid) != nIntents {
		t.Fatalf("ledger holds %d entries for %d logged intents", len(provMid), nIntents)
	}
	if provMid[n]&provAuto == 0 {
		t.Fatalf("the re-answered intent at index %d is unmarked (%02x): the auto flag never rode the resubmit", n, provMid[n])
	}
	intents2 := ledgerIntents(t, r, "t1")
	if intents2[n].Player != 0 {
		t.Fatalf("intent at index %d belongs to seat %d, want the re-answered human intent", n, intents2[n].Player)
	}

	if err := r.Undo("t1", 1, 0); err != nil {
		t.Fatalf("second Undo: %v", err)
	}
	if got := waitRewind(t, rewound); got != n {
		t.Fatalf("second rewind point %d, want %d", got, n)
	}
	provFinal := ledgerProvenance(t, r, "t1")
	if len(provFinal) != n {
		t.Fatalf("after the second undo the ledger holds %d entries, want %d — the marked entry at %d was not truncated", len(provFinal), n, n)
	}
	for i := range provFinal {
		if provFinal[i] != provBefore[i] {
			t.Fatalf("entry %d is %02x after both rewinds, want the original %02x", i, provFinal[i], provBefore[i])
		}
	}
}

// waitPastSeq waits until the game moves past the decision just answered —
// answerOnce's post-submit wait, without answerOnce's own submit.
func waitPastSeq(t *testing.T, r *Registry, id TableID, seq uint64) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		if d, err := r.Pending(id, 1, 0); err == nil && d.Seq != seq {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("game did not advance past seq %d", seq)
		}
		time.Sleep(time.Millisecond)
	}
}

// TestLedgerNotInLog: the provenance bits are host metadata, never replay
// input. Two tables with the identical config, one driven with auto marks
// on every other submit and one without any, reach the identical chain head
// The precondition — the ledgers really
// differ — is asserted first, so equal heads cannot mean equal marks.
func TestLedgerNotInLog(t *testing.T) {
	t.Parallel()
	r1 := undoTable(t, testOptions(t), "t1")
	r2 := undoTable(t, testOptions(t), "t2")
	submits1, autos1 := driveLedgerMatch(t, r1, "t1", 2)
	submits2, _ := driveLedgerMatch(t, r2, "t2", 0)
	if submits1 != submits2 {
		t.Fatalf("the identical fixtures answered different decision counts: %d vs %d", submits1, submits2)
	}
	if autos1 == 0 {
		t.Fatal("no submit was auto-flagged; the mark precondition is vacuous")
	}
	prov1 := ledgerProvenance(t, r1, "t1")
	prov2 := ledgerProvenance(t, r2, "t2")
	if len(prov1) != len(prov2) {
		t.Fatalf("ledger lengths diverged: %d vs %d", len(prov1), len(prov2))
	}
	marked1 := 0
	for i, b := range prov1 {
		if b&provAuto != 0 {
			marked1++
		}
		// Table 2 ran the identical match with no flags at all: clearing
		// table 1's auto bit must recover table 2's entry exactly. A
		// caretaker-bit difference here would mean the flag contaminated
		// the wrong bit.
		if b&^provAuto != prov2[i] {
			t.Fatalf("entry %d: %02x vs %02x — the fixtures differ beyond the auto bit", i, b, prov2[i])
		}
	}
	if marked1 != autos1 {
		t.Fatalf("table 1 marks %d intents, driver flagged %d", marked1, autos1)
	}
	for i, b := range prov2 {
		if b != 0 {
			t.Fatalf("table 2 (never auto) entry %d is marked %02x", i, b)
		}
	}
	m1 := ledgerMatch(t, r1, "t1")
	m2 := ledgerMatch(t, r2, "t2")
	m1.mu.RLock()
	h1 := m1.e.L.Head()
	m1.mu.RUnlock()
	m2.mu.RLock()
	h2 := m2.e.L.Head()
	m2.mu.RUnlock()
	if h1 != h2 {
		t.Fatalf("chain heads diverged with the auto flag: %s vs %s — the flag leaked into the log", h1, h2)
	}
}
