package kgen

// equipShapeRefusal reports why a, an Equip ability of the gorge expansion
// (an activated Attach at sorcery speed), is not a shape kgen keeps ("" if it
// is). A restricted equip (CR 702.6a: "Equip legendary creature {3}", "Equip
// Shaman, Warlock, or Wizard {1}", "Equip creature token {1}") keeps its
// narrowed target: each clause is a creature (or a subtype) you control, and
// the rest of the clause is lowered by cb.target like any other target
// filter, which refuses a property it cannot hold under its own reason. A
// second equip ability is an ordinary activated ability with the same shape
// and its own cost.
func equipShapeRefusal(a *lAbility) string {
	switch {
	case len(a.Targets) != 1:
		return "equip shape: target count"
	case len(a.Prog) != 1 || a.Prog[0].Op != "Attach":
		return "equip shape: effect"
	case !a.Sorcery:
		return "equip shape: timing"
	case !isEquipTargetYouCtrl(a.Targets[0].F):
		return "equip shape: target filter"
	}
	return ""
}

// isEquipTargetYouCtrl reports an equip target: every clause a creature (or a
// subtype, which implies one) you control, no player alternative.
func isEquipTargetYouCtrl(f *lFilter) bool {
	if f == nil || f.Players != 0 || len(f.Clauses) == 0 {
		return false
	}
	for _, c := range f.Clauses {
		if c.Ctrl != whoYou || c.Owner != 0 || c.NotTypes != 0 || c.TypesAll != 0 {
			return false
		}
		if c.Types != tCreature && !(c.Types == 0 && c.Subtype != "") {
			return false
		}
	}
	return true
}
