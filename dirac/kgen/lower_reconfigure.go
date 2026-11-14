package kgen

import "strings"

// K:Reconfigure (CR 702.151) is gorge's pair of activated abilities
// (engine/cards/kw_reconfigure.go), each with Keyword$ Reconfigure and
// SorcerySpeed$ True:
//
//   - AB$ Attach | ValidTgts$ Creature.YouCtrl: "attach this permanent to
//     another target creature you control". The unattached card is itself a
//     creature, and gorge excludes an attach SA's own source from the target
//     ask (cast_targets.go excludeSelf), so the filter gains Other here
//     (reconfigureOther); the rest is the Equip shape.
//   - AB$ Attach | Unattach$ True: "unattach this permanent", activated only
//     while it is attached (gorge's offer gate withholds it from an
//     unattached source). The gate is the ability's Present condition over
//     Card.Self+AttachedTo Creature (CR 702.151a), the body OpAttach with
//     kop.AttachUnattach (unattachSelf).
//
// The card's own flag (lc.out.Reconfigure) turns on the kernel's type switch
// (CR 702.151b, dirac/kw_reconfigure.go).

// reconfigureOther narrows a Reconfigure attach half's ValidTgts$ to another
// creature you control. Any other spelling is left to the ordinary target
// lowering (and so to its refusal or its own reading).
func (lc *lowerCtx) reconfigureOther(ps *params) {
	if lc.ab.Kind != akActivated || strings.TrimSpace(ps.m["Keyword"]) != "Reconfigure" {
		return
	}
	if v := strings.TrimSpace(ps.m["ValidTgts"]); v == "Creature.YouCtrl" {
		ps.m["ValidTgts"] = v + "+Other"
	}
}

// unattachSelf lowers `AB$ Attach | Unattach$ True` (Keyword$ Reconfigure):
// no target, the source itself, gated on being attached to a creature.
func (lc *lowerCtx) unattachSelf(ps *params, v string) error {
	if strings.TrimSpace(v) != "True" || lc.ab.Kind != akActivated || strings.TrimSpace(ps.m["Keyword"]) != "Reconfigure" {
		return refuse("Attach Unattach$ %q outside a Reconfigure ability", v)
	}
	for _, k := range []string{"ValidTgts", "Defined", "Object", "Choices", "Optional", "RememberAttached"} {
		if _, ok := ps.m[k]; ok {
			return refuse("Unattach$ with %s$", k)
		}
	}
	if lc.ab.ActCond.Kind != cNone {
		return refuse("two activation conditions")
	}
	f, err := lc.filter("Card.Self+AttachedTo Creature")
	if err != nil {
		return err
	}
	lc.ab.ActCond = lCond{Kind: cPresent, F: f, Op: opGE, A: cst(1)}
	lc.emit(lInstr{Op: "Attach", CS: "Unattach"})
	return nil
}
