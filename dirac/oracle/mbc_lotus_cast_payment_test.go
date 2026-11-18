package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// tail-74 pair 1 (seed 8247769561243559618): the seat's deck holds Lotus
// Blossom and Aven Windreader, and at decision 477 it activates Aven's "{1}{U}
// look at the top card of target player's library" while the pool is empty and
// only the Blossom -- a dynamic {T}, sacrifice source whose X is its petal
// counters -- can fund it. CR 605.1a makes that source a mana ability the seat
// may activate while paying the activation's costs, and gorge's planner cannot
// price an evaluated-amount source whose cost sacrifices it, so gorge's
// potential walk offers the play as a prefix script: runScript activates the
// source (tap, sacrifice, float its production in the colour the play needs)
// BEFORE the play is pushed, then the play pays from the floating pool. The
// kernel priced the source in its offer census and tapped it at the mana step,
// after targets, so the state at the target ask still held the Blossom on the
// battlefield with its counters and an empty pool. The kernel now floats the
// source before the build's push (dirac/mana_dynsacgap.go), and the recorded
// game replays clean on a raw table of its own decks' cards.
func TestMBCLotusBlossomCastPaymentLockstep(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro(filepath.Join("testdata", "repro", "lotus_blossom_pays_before_push.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	pol := &ScriptPolicy{Actions: r.Actions}
	res := replayDecodedScript(t, synthReg(t), cardset.Raw(printedDelveSet(t, r.Cards)), r, pol)
	if pol.next != len(r.Actions) || pol.Skipped != 0 {
		t.Fatalf("recording served %d of %d actions (%d skipped): its game path has drifted", pol.next, len(r.Actions), pol.Skipped)
	}
	if !res.Clean() {
		t.Fatalf("Lotus Blossom's prefix-script payment no longer replays clean: div %+v, stall %q", res.Div, res.Stall)
	}
}
