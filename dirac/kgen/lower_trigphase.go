package kgen

import "github.com/adams-shaun/gorge/engine/state"

// spendPhaseConds appends a trigger's ManaNotSpent$ / ManaSpent$ cast-spend
// gates and its Phase$ step gate to conds, the header gates lowerTriggered
// conjoins into tr.Cond. mode is the trigger's Mode$.
func (lc *lowerCtx) spendPhaseConds(tp *params, mode string, conds *[]lCond) error {
	if v, ok := tp.get("ManaNotSpent"); ok {
		// "sacrifice it unless {W} was spent to cast it" (CR 601.2h): the
		// trigger fires only while the gate reads unspent, gorge
		// trigger_condition.go manaColourSpendHolds
		c, err := lc.trigManaSpendCond(v, true)
		if err != nil {
			return err
		}
		*conds = append(*conds, c)
	}
	if v, ok := tp.get("ManaSpent"); ok {
		// "if {G} was spent to cast it, destroy target artifact" (CR 601.2h)
		c, err := lc.trigManaSpendCond(v, false)
		if err != nil {
			return err
		}
		*conds = append(*conds, c)
	}
	if v, ok := tp.get("Phase"); ok && mode != "Phase" {
		// "Whenever you cast an instant during your main phase" (Dovin's
		// Acuity), "enters during combat" (The Blue Spirit): Forge's Phase$
		// gate, which gorge reads only as the trigger fires
		// (trigger_phase_gate.go phaseGate, before the mode switch). The
		// lowered gate is an intervening-if the kernel also re-checks at
		// resolution; the two agree only where a trigger resolves in the step
		// it fired in. That holds wherever players get priority (CR 117.5: a
		// step ends only with an empty stack; a cleanup-step trigger gets
		// priority and another cleanup step, CR 514.3a), but not in the untap
		// step: nobody gets priority there (CR 502.4), so an untap-step
		// trigger resolves in upkeep and the re-check would drop it (The
		// Ninth Doctor, The Millennium Calendar). A set naming Untap refuses.
		// Mode$ Phase names its own step in the mode switch (tr.Step), so it
		// skips this.
		if set, _ := state.ParsePhases(v); set.Has(state.StepUntap) {
			return refuse("trigger Phase$ %q includes the untap step (no priority; the gate would be re-checked in upkeep)", v)
		}
		c, err := lc.phasesCondition(v)
		if err != nil {
			return err
		}
		*conds = append(*conds, c)
	}
	return nil
}
