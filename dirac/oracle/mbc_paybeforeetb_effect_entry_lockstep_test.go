package oracle

import (
	"testing"
)

// A card whose entry is replaced by ReplPayBeforeETB (Mox Diamond) that a
// pick-and-move effect puts onto the battlefield (Arms Race's ChooseMove
// from hand) asks its controller the pay-before-entering election with the
// move parked: the discard pays and the card enters, the decline (or no
// eligible land) sends it to its owner's graveyard, and the replacement
// chain's Cleanup ClearRemembered runs before the effect's own
// RememberChanged$ adds the entrant. The kernel's entry used to be already
// made at that point, so it stalled unmodelled (StallPayETB) instead of
// asking (synth tail-73 pair 2).
func TestMBCTail73ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "paybeforeetb_effect_entry")
}

// guard the preconditions the replay depends on: the recording is a
// non-stall-declared lockstep game on a synthetic pool, so the kernel's
// paybeforeetb stall is itself the failure.
func TestPayETBEffectEntryReproHeader(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro("testdata/repro/paybeforeetb_effect_entry.repro.json")
	if err != nil {
		t.Fatal(err)
	}
	if r.Stall {
		t.Fatal("precondition: the recording must not declare a stall")
	}
	if r.Pool != "tail-73" || r.Surface == "" || len(r.Actions) == 0 {
		t.Fatalf("precondition: pool %q surface %q actions %d", r.Pool, r.Surface, len(r.Actions))
	}
}
