package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

func init() {
	exprKinds[vSpellsLastTurn] = kop.ExSpellsCastLastTurn
}

// werewolfConds lowers the Innistrad werewolf intervening-ifs on a Mode$
// Phase trigger (gorge engine/rules/trigmatch/actions.go
// phaseTriggerMatches, which reads each as a presence flag, as Forge's
// Trigger does): WerewolfTransformCondition$ "if no spells were cast last
// turn" is "no player cast a spell last turn" (N 0 == 0); WerewolfUntransformCondition$
// "if a player cast two or more spells last turn" is the most one player cast
// >= 2 (kop.ExSpellsCastLastTurn, N 0 and N 1). Last turn's window is closed,
// so the trigger-time check gorge makes is also the resolution-time answer
// and the kernel's re-check agrees. Both on one trigger, or either beside a
// Mode$ other than Phase, refuses (gorge reads the transform one first).
func werewolfConds(tp *params, mode string, conds *[]lCond) error {
	_, tr := tp.get("WerewolfTransformCondition")
	_, untr := tp.get("WerewolfUntransformCondition")
	switch {
	case !tr && !untr:
		return nil
	case mode != "Phase":
		return refuse("werewolf transform condition on Mode$ %s", mode)
	case tr && untr:
		return refuse("both werewolf transform conditions on one trigger")
	case tr:
		*conds = append(*conds, lCond{Kind: cValue, A: lValue{Kind: vSpellsLastTurn}, Op: opEQ, B: cst(0)})
	default:
		*conds = append(*conds, lCond{Kind: cValue, A: lValue{Kind: vSpellsLastTurn, N: 1}, Op: opGE, B: cst(2)})
	}
	return nil
}
