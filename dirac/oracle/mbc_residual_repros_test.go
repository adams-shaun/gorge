//go:build dirac_mp

package oracle

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// mbcResidualRepro replays one four-seat MBC lock-pool recording (a kfuzz
// -kernel mbc sweep's divergent-pair repro, testdata/mbc_residual) and fails
// on a divergence, an undeclared kernel stall, or a replay that stops short
// of the recorded decisions.
func mbcResidualRepro(t *testing.T, name string) {
	t.Helper()
	base, _ := testCorpus(t)
	set, err := cardset.ByName("mbc")
	if errors.Is(err, cardset.ErrNoTables) || errors.Is(err, cardset.ErrNoCorpus) {
		t.Skipf("no mbc table (make dirac-tables): %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	set = cardset.AllowUnverified(set)
	r, err := ReadRepro(filepath.Join("testdata", "mbc_residual", name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.Matchup(base.Reg)
	if err != nil {
		t.Fatal(err)
	}
	if m.NSeat() < 3 {
		t.Fatalf("precondition: %d seats: want a multi-seat pod", m.NSeat())
	}
	lists := make([][]cardset.Entry, m.NSeat())
	for s, d := range append([][]string{r.Cards[0], r.Cards[1]}, r.MoreCards...) {
		// a pool's table names a two-faced card "Front // Back" (ReproEntries)
		lists[s] = ReproEntries(base.Reg, set, d)
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + 200
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	res := Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
	if res.Div != nil {
		t.Fatalf("divergence at %d of %d: %s %s %v", res.Div.At, len(r.Actions), res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if res.Decisions < len(r.Actions) {
		t.Fatalf("the replay ended after %d of %d decisions (stall %q)", res.Decisions, len(r.Actions), res.Stall)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) {
		t.Fatalf("kernel stalled: %s", res.Stall)
	}
}

// TestMBCSelfCastEverySeatReprosLockstep: a "when you cast this spell"
// trigger (CR 603.2) is the spell's own, on the stack; gorge's trigger walk
// visits the stack on seat 0's pass (rules/object_query.go forEachObject),
// so a caster beyond seat 0 queues it before its own battlefield listeners.
// The kernel fired it only for casters 0 and 1, so seat 2's Writhing
// Chrysalis cast left its token trigger off the kernel's stack (decision
// 1607 of 1607; the 2026-11-13 4-seat MBC lock pool, pair 11).
func TestMBCSelfCastEverySeatReprosLockstep(t *testing.T) {
	t.Parallel()
	mbcResidualRepro(t, "selfcast_fires_for_every_caster_seat")
}
