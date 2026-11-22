package oracle

import "testing"

// TestMBCCopiedTriggerSvarSiblingsLockstep replays the class repro: an
// embalmed Vizier of Many Faces copy's modal triggered ability (one
// target-bearing mode) resolving with its per-mode targets rechecked at
// resolution (CR 608.2b, engine/rules recheckCharmTargets) against the face
// that OWNS the trigger, not the source's active face (Vizier's). That
// recheck is one of the two sibling sites the copied-trigger-svar-siblings
// fix routed through triggerModeCtx.
//
// The second sibling -- the sequential per-mode Charm re-ask
// (charmSeqSVars) -- only runs for a copied trigger with TWO OR MORE
// target-bearing modes CHOSEN, which needs a choose-two-or-more triggered
// Charm (e.g. Mishra, Claimed by Gix's choose-three). No lockstep pair for
// that variant is recorded: no pool game has yet reached a copied
// multi-target-mode modal trigger's second slot re-ask (the shape is
// unobserved -- the same reason copied_trigger_mode_targets was latent until
// dr-tail-49). A recorded repro therefore cannot pin it; the engine/rules
// suite pins the whole two-mode shape over the same embalm-copy route in
// engine/rules/copied_trigger_svar_siblings_test.go, where both siblings are
// exercised red-to-green on inline fixtures.
func TestMBCCopiedTriggerSvarSiblingsLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "copied_trigger_mode_targets")
}
