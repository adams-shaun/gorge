package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// tail-74 pair 31 (seed 2889252279370268457): the seat holds a untapped Lotus
// Blossom -- a dynamic {T}, sacrifice source whose X is its petal counters --
// and casts Sweep the Skies ({X}{U}{U}, converge) with only the Blossom able
// to make the second {U}. gorge's cast payment planner cannot price an
// evaluated-amount source whose cost sacrifices it (the probe needs a TapOnly
// shape), so its payment leaves the Blossom untouched and the lower witness
// taps nothing it cannot fund (the cast then pays through the manual CR 601.2g
// window at the mana step); the kernel's payment plan (mana.go phase 1 pass 2)
// and its lower-witness census (mana_window.go lowerWitnessEx over
// availUngranted) priced the source and tapped and sacrificed it at the cast's
// application, before the X ask. The kernel now leaves the source out of the
// cast's payment plan, the lower witness, and the plain census, and counts its
// evaluated amount only where gorge's cast-window census does (the X ask,
// availXWindow), so the recorded game replays clean on a raw table of its own
// decks' cards.
func TestMBCLotusBlossomCastPaymentDynSacLockstep(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro(filepath.Join("testdata", "repro", "lotus_blossom_cast_payment_dynsac.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	pol := &ScriptPolicy{Actions: r.Actions}
	res := replayDecodedScript(t, synthReg(t), cardset.Raw(printedDelveSet(t, r.Cards)), r, pol)
	if pol.next != len(r.Actions) || pol.Skipped != 0 {
		t.Fatalf("recording served %d of %d actions (%d skipped): its game path has drifted", pol.next, len(r.Actions), pol.Skipped)
	}
	if !res.Clean() {
		t.Fatalf("Lotus Blossom's cast payment no longer replays clean: div %+v, stall %q", res.Div, res.Stall)
	}
}
