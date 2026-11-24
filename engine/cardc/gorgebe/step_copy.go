package gorgebe

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

func init() {
	registerStep(ir.VerbCreateTokenCopy, printCreateTokenCopy)
	registerStep(ir.VerbCopySpell, printCopySpell)
}

// printCreateTokenCopy prints CopyPermanent$: Defined$ is the copied object (or
// the target slot declares it), a haste or other keyword grant is AddKeywords$
// and a sacrifice at the next end step is AtEOTTrig$ Sacrifice.
func printCreateTokenCopy(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	bad := unprintable("verb:create-token-copy", st.Src)
	if st.Obj == nil || st.Who != nil || st.Optional || st.Enter != nil {
		return nil, 0, bad
	}
	n := &node{API: "CopyPermanent"}
	if err := p.operand(n, "Defined", st.Obj); err != nil {
		return nil, 0, err
	}
	if st.N != nil && !(st.N.Kind == ir.VConst && st.N.N == 1) {
		a, err := p.amount(st.N)
		if err != nil {
			return nil, 0, err
		}
		n.set("NumCopies", a)
	}
	var kws []string
	sac := false
	for _, e := range st.Effects {
		if e.Kind != ir.EffAddAbility || len(e.Abilities) != 1 || e.Filter != nil || e.Ref != nil || e.Cost != nil {
			return nil, 0, bad
		}
		a := e.Abilities[0]
		switch {
		case a.Kind == ir.AbKeyword && a.Keyword != nil && a.Keyword.N == nil && a.Keyword.Cost == nil && a.Keyword.Filter == nil:
			kws = append(kws, a.Keyword.Name)
		case isEndStepSacrifice(a) && !sac:
			sac = true
		default:
			return nil, 0, bad
		}
	}
	if len(kws) > 0 {
		n.set("AddKeywords", strings.Join(kws, " & "))
	}
	if sac {
		n.set("AtEOTTrig", "Sacrifice")
	}
	return n, 1, nil
}

// isEndStepSacrifice reports whether a is "at the beginning of the end step,
// sacrifice this".
func isEndStepSacrifice(a *ir.Ability) bool {
	t := a.Trigger
	if a.Kind != ir.AbTriggered || t == nil || t.Event != ir.EvEndStep || a.Body == nil || len(a.Body.Steps) != 1 {
		return false
	}
	s := t.Subject
	if s == nil || s.Subject != ir.SubjPlayer || s.Op != ir.FPred || s.Pred != ir.PAny {
		return false
	}
	b := a.Body.Steps[0]
	return b.Verb == ir.VerbSacrifice && b.Obj != nil && b.Obj.Kind == ir.RefSelf && !b.Optional && b.Who == nil
}

// printCopySpell prints CopySpellAbility$. The spell an event names is
// Defined$ of the event object, with the number of copies; this spell itself
// is Defined$ Parent, copied by the player Who (the unless-cost steps around
// it add UnlessCost$).
func printCopySpell(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	bad := unprintable("verb:copy-spell", st.Src)
	if st.Obj == nil {
		return nil, 0, bad
	}
	if st.Obj.Kind == ir.RefSelf {
		if st.N != nil || st.Who == nil {
			return nil, 0, bad
		}
		who, err := p.defined(st.Who)
		if err != nil {
			return nil, 0, err
		}
		return &node{API: "CopySpellAbility", Params: []kv{{"Defined", "Parent"}, {"Controller", who}, {"MayChooseTarget", "True"}}}, 1, nil
	}
	if st.Obj.Kind != ir.RefEventObject || st.Who != nil || st.Optional {
		return nil, 0, bad
	}
	d, err := p.defined(st.Obj)
	if err != nil {
		return nil, 0, err
	}
	n := &node{API: "CopySpellAbility", Params: []kv{{"Defined", d}}}
	if st.N != nil && !(st.N.Kind == ir.VConst && st.N.N == 1) {
		a, err := p.amount(st.N)
		if err != nil {
			return nil, 0, err
		}
		n.set("Amount", a)
	}
	n.set("MayChooseTarget", "True")
	return n, 1, nil
}
