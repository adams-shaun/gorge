package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// beginPhaseRepl lowers a printed phase-skip: R:Event$ BeginPhase ... Skip$
// True (gorge replacement_phase.go: events.StepChange maps to ReplBeginPhase,
// phaseStep maps the Phase$ value onto the Untap / Upkeep / Draw step
// constants and fails closed on any other, ValidPlayer$ You scopes the skip
// to the replacement controller's own turn while a line with no ValidPlayer$
// applies every turn, and continuePhaseReplacements applies each matched
// skip, its landing StepChange re-matched so a chain of skips keeps skipping
// with the step numbers strictly increasing; Cleanup is never skipped). The
// kernel replays exactly that walk at the step boundary (repl_beginphase.go):
// every candidate only skips, so no order ask is modelled.
func (lc *lowerCtx) beginPhaseRepl(rp *params, rl *lRepl) error {
	az := rp.str("ActiveZones")
	if az != "Battlefield" {
		return refuse("repl-hard: BeginPhase ActiveZones %q", az)
	}
	// Optional$ True parks gorge's phase ask of the affected player and the
	// ReplaceWith$ body runs on its election (Fasting); the kernel poses no
	// phase ask, so any body refuses by name.
	if rp.str("Optional") == "True" || rp.peek("ReplaceWith") != "" {
		return refuse("repl-hard: BeginPhase Optional body")
	}
	// Hellbent$ True gates the skip on an empty hand (Gibbering Descent, one
	// corpus line): a hand read no kernel replacement condition expresses.
	if rp.str("Hellbent") == "True" {
		return refuse("repl-hard: BeginPhase Hellbent")
	}
	if !rp.is("Skip", "True") {
		return refuse("repl-hard: BeginPhase Skip %q", rp.str("Skip"))
	}
	step, ok := beginPhaseStep(rp.str("Phase"))
	if !ok {
		return refuse("repl-hard: BeginPhase Phase %q", rp.str("Phase"))
	}
	// ValidPlayer$ (the shared repl arm): You is the controller's own turn;
	// Player and no ValidPlayer$ apply every turn (Sands of Time, Stasis).
	if _, ok := rp.get("ValidPlayer"); ok {
		op, err := lc.modeParam("repl", "BeginPhase", "ValidPlayer", rp)
		if err != nil {
			return err
		}
		switch op.Who {
		case whoYou:
			rl.Bits = kop.ReplPhYou
		case whoAny: // every turn
		default:
			return refuse("repl-hard: BeginPhase ValidPlayer opponent scope")
		}
	}
	rl.Kind = "BeginPhaseSkip"
	rl.Phase = uint8(step)
	return nil
}

// beginPhaseStep maps a Forge Phase$ value onto the kernel step constants the
// corpus names (gorge phaseStep). Only the three known values lower; any
// other fails closed, leaving the phase to run.
func beginPhaseStep(ph string) (kop.Step, bool) {
	switch ph {
	case "Untap":
		return kop.StepUntap, true
	case "Upkeep":
		return kop.StepUpkeep, true
	case "Draw":
		return kop.StepDraw, true
	}
	return 0, false
}
