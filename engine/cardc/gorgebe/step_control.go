package gorgebe

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

func init() {
	registerStep(ir.VerbGainControl, printGainControl)
	registerStep(ir.VerbAttach, printAttach)
}

// sameTarget reports whether a and b name the same target slot.
func sameTarget(a, b *ir.Ref) bool {
	a, b = slotRef(a), slotRef(b)
	return a != nil && b != nil && a.Kind == ir.RefTarget && b.Kind == ir.RefTarget && a.Slot == b.Slot
}

// printGainControl prints GainControl$. Forge writes the untap and the
// granted keywords that follow ("untap it, it gains haste") as parameters of
// the same node, so those steps are consumed with it.
func printGainControl(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	if st.Optional || st.Who != nil || st.Obj == nil {
		return nil, 0, unprintable("verb:gain-control", st.Src)
	}
	n := &node{API: "GainControl"}
	if err := p.operand(n, "Defined", st.Obj); err != nil {
		return nil, 0, err
	}
	used := 1
	if len(steps) > used && steps[used].Verb == ir.VerbUntap && !steps[used].Optional && sameTarget(steps[used].Obj, st.Obj) {
		n.set("Untap", "True")
		used++
	}
	if len(steps) > used && steps[used].Verb == ir.VerbApplyEffect && !steps[used].Optional && steps[used].Dur == st.Dur &&
		sameTarget(steps[used].Obj, st.Obj) && onlyBareKeywords(steps[used]) {
		var kws []string
		for _, e := range steps[used].Effects {
			k, _ := bareKeywords(e)
			kws = append(kws, k...)
		}
		n.set("AddKWs", strings.Join(kws, " & "))
		used++
	}
	switch st.Dur {
	case ir.DurEndOfTurn:
		n.set("LoseControl", "EOT")
	case ir.DurPermanent:
	default:
		return nil, 0, unprintable("gain-control:dur-"+st.Dur.String(), st.Src)
	}
	if err := zoptional(n, st); err != nil {
		return nil, 0, err
	}
	return n, used, nil
}

// onlyBareKeywords reports whether every effect of st grants bare keywords.
func onlyBareKeywords(st *ir.Step) bool {
	if len(st.Effects) == 0 {
		return false
	}
	for _, e := range st.Effects {
		if e.Kind != ir.EffAddAbility {
			return false
		}
		if _, ok := bareKeywords(e); !ok {
			return false
		}
	}
	return true
}

// printAttach prints Attach$: ValidTgts$ declares what the source attaches
// to; the attached object is the source itself (Forge's default).
func printAttach(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	bad := unprintable("verb:attach", st.Src)
	o := st.Obj
	if st.Optional || st.Who != nil || o == nil || st.Other == nil {
		return nil, 0, bad
	}
	if !(o.Kind == ir.RefSelf || (o.Kind == ir.RefEventObject && p.evSelf)) {
		return nil, 0, bad
	}
	n := &node{API: "Attach"}
	if slotRef(st.Other).Kind != ir.RefTarget {
		return nil, 0, bad
	}
	if err := p.operand(n, "Defined", st.Other); err != nil {
		return nil, 0, err
	}
	if _, ok := n.get("ValidTgts"); !ok {
		return nil, 0, bad
	}
	return n, 1, nil
}

// preventCombatRider reports whether st is "prevent all combat damage that
// would be dealt to it this turn" for the object ob: Forge writes it as one
// more keyword of the Pump$ that grants the object its abilities.
func preventCombatRider(st *ir.Step, ob *ir.Ref) bool {
	if st == nil || st.Verb != ir.VerbCreateReplacement || st.Optional || st.Dur != ir.DurEndOfTurn || len(st.Abilities) != 1 {
		return false
	}
	a := st.Abilities[0]
	if a.Kind != ir.AbReplacement || a.Trigger == nil || a.Cond != nil || a.Body == nil || len(a.Body.Steps) != 1 || len(a.Body.Targets) != 0 {
		return false
	}
	t, b := a.Trigger, a.Body.Steps[0]
	return t.Event == ir.EvWouldDealDamage && t.Word == "combat" && t.Subject == nil && t.Object == nil && sameTarget(t.Ref, ob) &&
		b.Verb == ir.VerbPrevent && b.Obj == nil && b.N == nil && !b.Optional && b.Cond == nil
}

// preventCombatKW is the keyword that grants that prevention.
const preventCombatKW = "Prevent all combat damage that would be dealt to CARDNAME."
