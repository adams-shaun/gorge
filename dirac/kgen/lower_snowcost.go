package kgen

// {S} activation costs (CR 107.4h). The kernel pays a {S} pip from a snow unit
// of the floating pool (dirac/mana.go payFromPool, the tally in mana_snow.go),
// so two shapes it cannot tally are refused here.

// snowCostOK reports whether the cost being lowered may read the {S} token: a
// printed activated ability's own Cost$ (lc.actCost), never an escape cost, an
// unless cost or a printed spell's ManaCost.
func (lc *lowerCtx) snowCostOK() bool { return lc.actCost && !lc.escCost }

// snowSourceCheck refuses a snow mana source (CardDef.Snow, and neither an
// Artifact, a Desert nor a Cave, whose mana gorge tallies as typed) whose mana
// ability can add more than one unit, or is PersistentMana$ or restricted: the
// kernel's snow tally counts one unit per ability resolution (mana_snow.go
// snowTag) and keeps no restricted or persistent batch's snow share.
func (lc *lowerCtx) snowSourceCheck() {
	out := lc.out
	if !out.Snow || out.Types&tArtifact != 0 {
		return
	}
	for _, s := range out.Subtypes {
		if s == "Desert" || s == "Cave" {
			return
		}
	}
	persistent := lc.face.MentionsParam("PersistentMana")
	for _, a := range out.Abilities {
		if a.Kind != akMana {
			continue
		}
		m := a.Mana
		switch {
		case persistent:
			lc.refuseNode("card", "snow mana source adding %v", "PersistentMana$ mana")
		case m.Restrict != nil || m.RestrictAb != nil || m.NonSpell:
			lc.refuseNode("card", "snow mana source adding %v", "restricted mana")
		case m.Each || !isConstN(m.Amount, 1):
			lc.refuseNode("card", "snow mana source adding %v", "more than one unit")
		default:
			continue
		}
		return
	}
}
