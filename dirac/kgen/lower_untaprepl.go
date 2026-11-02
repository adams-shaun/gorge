package kgen

import (
	"strings"
)

// othersDoesntUntap lowers a printed R:Event$ Untap | Layer$ CantHappen |
// ValidStepTurnToController$ You line whose ValidCard$ is not the bare
// Card.Self (an Aura's / Equipment's "enchanted creature doesn't untap
// during its controller's untap step", Card.Self with a property): gorge
// cards.ReplUntap matched against the untapped permanent with the line's
// source and controller (rules/replacement_match.go), only in its
// controller's untap step. ActiveZones$ must be Battlefield (or absent:
// Forge's default). The kernel runs these in untapStepGen, which also poses
// gorge's CR 616.1 ask when two or more match one permanent. The
// ValidStepTurnToController$ Player.Activator Effect form is
// an Effect, not lowered here.
func (lc *lowerCtx) othersDoesntUntap(rp *params, rl *lRepl, vc string) error {
	return lc.untapReplFilter(rp, rl, vc, "OthersDoesntUntap")
}

// untapReplFilter is the shared body of the Untap replacement filter forms:
// othersDoesntUntap (step-scoped) and "AnyUntap" (no
// ValidStepTurnToController$: every Untap event).
func (lc *lowerCtx) untapReplFilter(rp *params, rl *lRepl, vc, kind string) error {
	if az, ok := rp.get("ActiveZones"); ok && strings.TrimSpace(az) != "Battlefield" {
		return refuse("Untap replacement ActiveZones$ %q", az)
	}
	f, err := lc.filter(vc)
	if err != nil {
		return err
	}
	rl.Kind, rl.F = kind, f
	return nil
}
