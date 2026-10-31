package host

// HD-3 acceptance tests (docs/design/2026-10-10-human-divergence-design.md
// §5.3-§5.5, ticket HD-3): SnapshotMatch names match k; a shadow built the
// way the host's dispatch builds a seat's input reproduces itself exactly
// (parity, zero divergences); a seat that deviates at chosen intents is
// found at exactly those intents; caretaker and auto intents are never
// scanned.

import (
	"slices"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/sbtactical"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/divergence"
	"github.com/adams-shaun/gorge/server/protocol"
)

// TestShadowParityZeroDivergences is the key test: a seat that answers every
// decision with a fresh bots.New seeded ShadowSeed(seed, 0, n) through the
// host's own dispatch is, to the scan, indistinguishable from its shadow:
// ShadowScan reports 0 divergences over every decision the seat answered.
// Run for the Board path ("bot") and an EnvSeat policy (sb-tactical) through
// the exported contract (RunShadowParityContract).
func TestShadowParityZeroDivergences(t *testing.T) {
	t.Parallel()
	for _, policy := range []string{bots.Default, sbtactical.Policy} {
		t.Run(policy, func(t *testing.T) {
			t.Parallel()
			o := testOptions(t)
			o.BotDeps = sampleBotDeps(t)
			RunShadowParityContract(t, o, policy, 20261010, 0)
		})
	}
}

// TestShadowScanFindsInjected: a seat that answers like the shadow except at
// two chosen intents is found at exactly those intents, with the material
// reason the predicate gives for the swapped answer.
func TestShadowScanFindsInjected(t *testing.T) {
	t.Parallel()
	type inj struct {
		n    int
		why  string
		alt  []int
		kind decision.Kind
	}
	var injected []inj
	eligible := 0
	o := testOptions(t)
	o.BotDeps = sampleBotDeps(t)
	r, ss := shadowSeatTable(t, o, bots.Default, 20261011, 0, func(s *shadowTestSeat) {
		s.deviate = func(d *decision.Decision, n int, b decision.Intent) (decision.Intent, bool) {
			if len(injected) >= 2 {
				return decision.Intent{}, false
			}
			for _, o := range d.Options {
				alt := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{o.Index}}
				if d.Validate(alt) != nil {
					continue
				}
				if mat, why := divergence.Material(d, alt, b); mat {
					eligible++
					// Skip some eligible decisions so the two injections are
					// spread through the game rather than adjacent.
					if eligible < 4 || (len(injected) == 1 && eligible < 12) {
						return decision.Intent{}, false
					}
					injected = append(injected, inj{n: n, why: why, alt: alt.Choices, kind: d.Kind})
					return alt, true
				}
				break
			}
			return decision.Intent{}, false
		}
	})
	if len(injected) != 2 {
		t.Fatalf("fixture injected %d deviations, want 2 (eligible %d)", len(injected), eligible)
	}
	divs, scanned, err := r.shadowScan("t1", 1, 0, bots.Default)
	if err != nil {
		t.Fatalf("ShadowScan: %v", err)
	}
	if scanned < 20 {
		t.Fatalf("scan visited only %d decisions", scanned)
	}
	if len(divs) != len(injected) {
		t.Fatalf("scan found %d divergences, injected %d (%+v)", len(divs), len(injected), injected)
	}
	for i, in := range injected {
		dv := divs[i]
		if dv.Intent != in.n || dv.Material != in.why || dv.Kind != in.kind {
			t.Fatalf("divergence %d = intent %d %s %s, want intent %d %s %s", i, dv.Intent, dv.Material, dv.Kind, in.n, in.why, in.kind)
		}
		if !slices.Equal(dv.Human.Choices, in.alt) {
			t.Fatalf("divergence %d human choices %v, want %v", i, dv.Human.Choices, in.alt)
		}
		if slices.Equal(dv.Human.Choices, dv.Shadow.Choices) {
			t.Fatalf("divergence %d human and shadow answers are the same: %v", i, dv.Human.Choices)
		}
		if dv.Policy != bots.Default || dv.Table != "t1" || dv.Match != 1 || dv.Seat != 0 ||
			dv.ShadowSeed != divergence.ShadowSeed(ss.matchSeed, 0, in.n) || dv.ID == "" || len(dv.Options) == 0 {
			t.Fatalf("divergence %d record fields wrong: %+v", i, dv)
		}
	}
}

// TestShadowScanSkipsLedgerMarked: the human-seated table's caretaker and
// auto-flagged intents are absent from the scan, and are PRESENT when the
// same match is scanned with an empty ledger — so the absence is the
// ledger's doing, not a fixture that never diverged there.
func TestShadowScanSkipsLedgerMarked(t *testing.T) {
	t.Parallel()
	o := testOptions(t)
	r := undoTable(t, o, "t1")
	submits, autos := driveLedgerMatch(t, r, "t1", 2)
	if submits == 0 || autos == 0 {
		t.Fatalf("fixture degenerate: %d submits, %d auto", submits, autos)
	}
	prov := ledgerProvenance(t, r, "t1")
	marked := map[int]bool{}
	for i, b := range prov {
		if b&(provCaretaker|provAuto) != 0 {
			marked[i] = true
		}
	}
	if len(marked) == 0 {
		t.Fatal("no ledger entry is marked")
	}
	divs, scanned, err := r.shadowScan("t1", 1, 0, bots.Default)
	if err != nil {
		t.Fatalf("ShadowScan: %v", err)
	}
	if len(divs) == 0 || scanned == 0 {
		t.Fatalf("unmarked scan found %d divergences over %d decisions; nothing to compare", len(divs), scanned)
	}
	for _, dv := range divs {
		if marked[dv.Intent] {
			t.Fatalf("divergence at intent %d, which the ledger marks %02x", dv.Intent, prov[dv.Intent])
		}
	}
	// The same match with the marks cleared must diverge at a marked intent.
	m := ledgerMatch(t, r, "t1")
	m.mu.Lock()
	for i := range m.ledger {
		m.ledger[i] = 0
	}
	m.mu.Unlock()
	all, scannedAll, err := r.shadowScan("t1", 1, 0, bots.Default)
	if err != nil {
		t.Fatalf("ShadowScan (cleared): %v", err)
	}
	if scannedAll <= scanned {
		t.Fatalf("clearing the ledger did not widen the scan: %d -> %d decisions", scanned, scannedAll)
	}
	hit := false
	for _, dv := range all {
		if marked[dv.Intent] {
			hit = true
		}
	}
	if !hit || len(all) <= len(divs) {
		t.Fatalf("cleared-ledger scan has %d divergences (%d before), none at a marked intent: the marks were not what excluded them", len(all), len(divs))
	}
}

// TestSnapshotMatchNamesMatchK: on a perpetual table, once match 2 has
// started, SnapshotMatch(id, 1) is match 1's — its head is the finished
// sidecar's — while SnapshotForFeedback follows the current match.
func TestSnapshotMatchNamesMatchK(t *testing.T) {
	t.Parallel()
	o := testOptions(t)
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	cfg := TableConfig{ID: "t1", Name: "perp", Seats: 2, Decks: []string{"a", "b"},
		Seed: 77, Pace: 0, Spectator: view.Omniscient, Perpetual: true}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	var first protocol.MatchInfo
	deadline := time.Now().Add(60 * time.Second)
	for {
		ms, err := r.Matches("t1")
		if err != nil {
			t.Fatal(err)
		}
		if len(ms) >= 3 && ms[0].State == protocol.MatchFinished {
			first = ms[0]
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("table never reached a third match: %+v", ms)
		}
		time.Sleep(time.Millisecond)
	}
	if first.Match != 1 || first.Head == "" {
		t.Fatalf("fixture: first match info %+v", first)
	}
	snap, err := r.SnapshotMatch("t1", 1, nil)
	if err != nil {
		t.Fatalf("SnapshotMatch: %v", err)
	}
	if snap.Match.Match != 1 || snap.Log.Head != first.Head || snap.Match.Head != first.Head {
		t.Fatalf("snapshot is match %d head %s / %s, want match 1 head %s", snap.Match.Match, snap.Log.Head, snap.Match.Head, first.Head)
	}
	if snap.View != nil {
		t.Fatal("a nil seat must skip the view")
	}
	cur, err := r.SnapshotForFeedback("t1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if cur.Match.Match == 1 {
		t.Fatalf("SnapshotForFeedback still names match 1; the fixture cannot tell the entry points apart")
	}
	if _, err := r.SnapshotMatch("t1", 99, nil); err == nil {
		t.Fatal("SnapshotMatch of an unknown match must fail")
	}
}

// TestShadowParityCappedOnASeatDecision: a capped match parks the decision
// at the cap, and the shadow seat answers it, before play's MaxIntents guard
// ends the match, so that answer never reaches the log. The contract's
// "scan visited every decision the seat answered" precondition counts only
// the answers the log records, so a cap that lands on the seat's own
// decision does not fail the row.
func TestShadowParityCappedOnASeatDecision(t *testing.T) {
	t.Parallel()
	o := testOptions(t)
	o.BotDeps = sampleBotDeps(t)
	const seed = 20261010
	for c := 60; c < 80; c++ {
		r, ss := shadowSeatTable(t, o, bots.Default, seed, c, nil)
		logged, err := r.loggedIntents("t1", 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(ss.seen) == 0 || ss.seen[len(ss.seen)-1] < logged {
			continue
		}
		if got := ss.answeredBelow(logged); got != len(ss.seen)-1 {
			t.Fatalf("cap %d: %d of %d answers below the %d logged intents, want all but the parked one", c, got, len(ss.seen), logged)
		}
		RunShadowParityContract(t, o, bots.Default, seed, c)
		return
	}
	t.Fatal("no cap in [60,80) parked a decision of the shadow seat")
}
