package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// dmgBatchScen is a table whose "Test Watcher" has a bare DamageDealtOnce
// line, n watchers on seat 0 and a bear as the damage source.
func dmgBatchScen(t *testing.T, n int) (*scen, Obj) {
	t.Helper()
	tbl := *vanilla.Table
	tbl.Cards = append([]kop.CardDef(nil), vanilla.Table.Cards...)
	tbl.Exprs = make([]kop.Expr, 1) // force the generated path
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Watcher", Types: kop.TCreature, Power: 1, Toughness: 1,
		Abilities: []kop.Ability{{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigDamageDealtOnce}}}})
	watcher := CardID(len(tbl.Cards) - 1)
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	src := sc.perm(vanilla.GrizzlyBears, 0)
	for i := 0; i < n; i++ {
		sc.perm(watcher, 0)
	}
	sc.s.g.stalled = 0
	sc.s.npend = 0
	return sc, src
}

// TestDmgBatchOutsideSweepIgnoresScratch: the DamageAll sweep batch's open
// flag is not read out of the shared scratch. Another arm's bytes in scr
// (combat damage frozen on a replacement-order ask, a cast's build) must
// neither arm a batch record nor index past the records: each damage event
// outside a sweep queues its DamageDealtOnce line on its own.
func TestDmgBatchOutsideSweepIgnoresScratch(t *testing.T) {
	sc, src := dmgBatchScen(t, 1)
	for i := range sc.s.g.scr {
		sc.s.g.scr[i] = ^uint64(0) // every byte of every other arm set: any flag read out of scr is true, any count is 255
	}
	if sc.s.g.dmgBOn || sc.s.g.dmgBn != 0 {
		t.Fatalf("precondition: batch open %v with %d lines before any sweep", sc.s.g.dmgBOn, sc.s.g.dmgBn)
	}
	for i := 1; i <= 12; i++ { // more events than the batch has records
		sc.s.fireDamageEvGen(sc.k, sc.slot(src), 1, false, false, nil, 0, nil)
		if sc.s.Stalled() {
			t.Fatalf("event %d: stalled %q", i, StallName(sc.s.StallReason()))
		}
		if int(sc.s.npend) != i {
			t.Fatalf("event %d: %d pending, want one trigger per event", i, sc.s.npend)
		}
	}
	if sc.s.g.dmgBn != 0 {
		t.Fatalf("outside a sweep %d batch records were armed", sc.s.g.dmgBn)
	}
}

// TestDmgBatchSweepBatches: inside an open sweep the same events are one
// batch: one trigger carrying the total, and the ninth distinct line stalls
// StallDmgBatch rather than writing past the records.
func TestDmgBatchSweepBatches(t *testing.T) {
	sc, src := dmgBatchScen(t, 1)
	sc.s.dmgBatchOpen()
	for i := 0; i < 3; i++ {
		sc.s.fireDamageEvGen(sc.k, sc.slot(src), 2, false, false, nil, 0, nil)
	}
	sc.s.dmgBatchClose()
	if sc.s.npend != 1 || sc.s.pend[0].trigAmt != 6 {
		t.Fatalf("a sweep of three 2-damage events queued %d (amount %d), want one for 6", sc.s.npend, sc.s.pend[0].trigAmt)
	}
	if sc.s.g.dmgBOn || sc.s.g.dmgBn != 0 {
		t.Fatalf("closed batch left open %v / %d", sc.s.g.dmgBOn, sc.s.g.dmgBn)
	}

	sc, src = dmgBatchScen(t, maxDmgBatch+1)
	sc.s.dmgBatchOpen()
	sc.s.fireDamageEvGen(sc.k, sc.slot(src), 1, false, false, nil, 0, nil)
	if got := sc.s.StallReason(); got != StallDmgBatch {
		t.Fatalf("%d lines in one batch: stall %q, want dmgbatch", maxDmgBatch+1, StallName(got))
	}
	if sc.s.g.dmgBn != maxDmgBatch {
		t.Fatalf("batch holds %d records, want %d", sc.s.g.dmgBn, maxDmgBatch)
	}
}
