//go:build dirac_mp

package oracle

import (
	"errors"
	"path/filepath"
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
	// no tail-stall check: past the recorded actions the FirstPolicy
	// continuation can reach one of the pool's remaining unmodelled classes
	// (replleave), which the recorded game never reached
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

// TestMBCDepartedSweepAfterBatchReprosLockstep: gorge's state-based pass
// (rules/sba.go) computes the lethal-damage casualties and applies the batch
// before sweepDepartedPlayers moves a departed seat's objects (CR 800.4a), so
// an Aura that seat controls (Witness Protection on the Angel of Vitality,
// making it a 1/1) is still in force when its bearer's damage is judged. The
// kernel swept the departed seat first, the Angel reverted to 4/4 and
// survived its damage (decision 1434 of 1434; the 2026-11-13 4-seat MBC lock
// pool, pair 46).
func TestMBCDepartedSweepAfterBatchReprosLockstep(t *testing.T) {
	t.Parallel()
	mbcResidualRepro(t, "departed_sweep_after_the_sba_batch")
}

// TestMBCAnsweredCasualtyTokenCeaseReprosLockstep: gorge rechecks between a
// parked casualty's answer and the next ask (rules/sba.go: the answer
// re-enters the check; the recheck destroys nothing, which ends the sbaChain,
// and the tokens the answered moves left off the battlefield cease there),
// so a token Rest in Peace exiled on a casualty's answer is gone by the next
// casualty's ask. The kernel posed the next ask with the token still in
// exile (CR 111.7, 704.5d; decision 1934 of 1934; the 2026-11-13 4-seat MBC
// lock pool, pair 61).
func TestMBCAnsweredCasualtyTokenCeaseReprosLockstep(t *testing.T) {
	t.Parallel()
	mbcResidualRepro(t, "answered_casualty_ceases_tokens_before_the_next_ask")
}

// TestMBCDepartedSeatReplacementInertReprosLockstep: gorge's replacement walks
// (rules/object_query.go forEachObject) visit only living seats' zones, so a
// departed seat's Rest in Peace no longer exiles other seats' casualties (CR
// 800.4a) even before the leave sweep has moved its permanent. The kernel's
// ExileOnLeave scans (exileOnLeaveN, exileAnyN, graveMoveSources) still
// consulted it, so a creature dying after the seat's loss went to exile
// instead of the graveyard (Graaz, Unstoppable Juggernaut; decision 1397 of
// 1397; the 2026-11-13 4-seat MBC lock pool, pair 19).
func TestMBCDepartedSeatReplacementInertReprosLockstep(t *testing.T) {
	t.Parallel()
	mbcResidualRepro(t, "departed_seat_replacement_inert")
}

// TestMBCLethalRecheckKeepsTheChainReprosLockstep: gorge's chain-clear check
// (rules/sba.go) runs before ceaseDeadTokens, so a pass that queues a lethal
// casualty keeps the sbaChain and skips the sweep -- the re-entry pass after
// one casualty's answer re-queues the next lethal creature and poses its ask
// with the token an earlier answer exiled still in exile; the token ceases
// only when a pass queues nothing pre-sweep, after the last answer. The
// kernel ended the chain at every re-run ask, ceasing Rest in Peace's token
// before the second lethal creature's ask (CR 111.7, 704.5d; decision 2201
// of 2201; the 2026-11-13 4-seat MBC lock pool, pair 5).
func TestMBCLethalRecheckKeepsTheChainReprosLockstep(t *testing.T) {
	t.Parallel()
	mbcResidualRepro(t, "lethal_recheck_keeps_the_chain")
}
