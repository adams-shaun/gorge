package oracle

import (
	"path/filepath"
	"testing"
)

// Stingscourger's echo payment ({2}, paid by its controller at the beginning of
// their upkeep) is a trigger-cost mana window. On the recorded board Ashnod's
// Altar is a sacrifice-any source (no {T}) and Elven Chorus grants every
// creature "{T}: Add one mana of any color". The first 587 recorded actions are
// the game both engines agree on; the tail's FirstPolicy then takes the payment
// (choose:trigger_cost_pay). The kernel's window walk must search the Altar's
// sacrifice and the grant taps exactly as gorge does, not answer them early.
// The recorded script continues past the activation, so it never reaches this
// payment on its own: truncate at the shared prefix and let FirstPolicy play.
func TestMBCEchoPaySacAnyGrantsReprosLockstep(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro(filepath.Join("testdata", "repro", "sac_any_gap_beside_grants_activation.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	const prefix = 587 // the last action the two engines agree on
	pol := &ScriptPolicy{Actions: r.Actions[:prefix]}
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, pol)
	// The precondition: the recorded prefix really was played, so the result
	// below is the engines agreeing through the payment, not a script that
	// silently stopped early.
	if pol.next != prefix || pol.Skipped != 0 {
		t.Fatalf("precondition: script served %d of %d actions (%d skipped)", pol.next, prefix, pol.Skipped)
	}
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v (detail %q ref %v ker %v last %q)", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail, res.Div.RefOptions, res.Div.KerOptions, res.Div.LastSourced)
	}
	if res.Decisions < 592 {
		t.Fatalf("precondition: replay stopped at %d decisions, before the divergence at 592", res.Decisions)
	}
	stallCheck(t, r, res, false)
}
