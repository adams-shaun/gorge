package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// genericChoiceTargeted lowers a GenericChoice a targeted player answers
// (Defined$ absent and a player ValidTgts$: gorge
// GenericChoicePlayerChoosers makes that player the chooser, effCharm asks
// them at resolution). The target slot is the choice's own target, so the
// branches' Defined$ Targeted is that player. The optional FallbackAbility$
// runs when no branch's unless cost can be paid; OpChooseBranch's fallback
// branch is index len(choices).
func (lc *lowerCtx) genericChoiceTargeted(ps *params) error {
	for _, k := range []string{"AtRandom", "Random", "ShowChoice", "ChoiceRestriction", "NumRandomChoices"} {
		if _, ok := ps.get(k); ok {
			return refuse("GenericChoice of a target player with %s$", k)
		}
	}
	choices := strings.Split(ps.str("Choices"), ",")
	if len(choices) < 1 {
		return refuse("GenericChoice with %d choices", len(choices))
	}
	for _, name := range choices {
		if cards.ResolveSVar(lc.face.SVars, strings.TrimSpace(name)) == nil {
			return refuse("GenericChoice branch %q", name)
		}
	}
	if fb := ps.str("FallbackAbility"); fb != "" && cards.ResolveSVar(lc.face.SVars, fb) == nil {
		return refuse("GenericChoice fallback %q", fb)
	}
	if lc.ab.Kind != akTriggered && lc.ab.Kind != akSpell && lc.ab.Kind != akActivated {
		return refuse("GenericChoice of a target player in a %d ability", lc.ab.Kind)
	}
	obj, err := lc.objOrTarget(ps, "", zBattlefield)
	if err != nil {
		return err
	}
	if obj.Kind != rTarget {
		return refuse("GenericChoice of a target player without a target slot")
	}
	t := lc.ab.Targets[obj.N]
	if t.F == nil || t.F.Players == 0 || len(t.F.Clauses) != 0 || t.AtRes ||
		!t.Min.isConst() || t.Min.N != 1 || !t.Max.isConst() || t.Max.N != 1 {
		return refuse("GenericChoice of a target player without one mandatory player target")
	}
	lc.emit(lInstr{Op: "GenericChoice", Obj: obj, A: cst(len(choices))})
	emit := func(name string, branch int) error {
		sa := cards.ResolveSVar(lc.face.SVars, strings.TrimSpace(name))
		bat := len(lc.ab.Prog)
		lc.emit(lInstr{Op: "Branch", C: branch})
		if err := lc.chain(sa, nil); err != nil {
			return err
		}
		lc.ab.Prog[bat].Skip = len(lc.ab.Prog) - bat - 1
		return nil
	}
	for i, name := range choices {
		if err := emit(name, i); err != nil {
			return err
		}
	}
	if fb := ps.str("FallbackAbility"); fb != "" {
		if err := emit(fb, len(choices)); err != nil {
			return err
		}
	}
	return nil
}

// genericChoiceYou lowers a GenericChoice the controller answers (Defined$
// You or absent: gorge charmGenericPlayers declines it and effCharm asks
// the controller one mode). A triggered ability's is asked as the trigger
// is put on the stack (gorge's CR 603.3c placement ask keeps it modal):
// lowered as a one-mode charm. A spell's or an activated ability's is
// asked at resolution (KModes over every choice): OpChooseBranch and one
// Branch per choice. A branch headed by an unless cost lowers in every kind:
// neither the kernel's mode offer (dirac/cast.go modeFeasible: targets,
// ChoiceRestriction, Spree mode costs) nor gorge's placement ask
// (trigger_queue.go CharmEligibleModes + triggerModesWithTargets) filters an
// unpayable branch, whose unless ask is decline-only. The FallbackAbility$ /
// random shapes refuse.
func (lc *lowerCtx) genericChoiceYou(ps *params) error {
	if _, ok := ps.get("Defined"); !ok {
		// gorge GenericChoicePlayerChoosers: no Defined$ and a player
		// ValidTgts$ make the targeted player the chooser
		if vt, ok := ps.get("ValidTgts"); ok {
			f, err := lc.filter(vt)
			if err != nil {
				return refuse("GenericChoice of a target player: %v", err)
			}
			if f.Players != 0 {
				if len(f.Clauses) != 0 {
					return refuse("GenericChoice of a target player with a compound filter")
				}
				return lc.genericChoiceTargeted(ps)
			}
		}
	}
	for _, k := range []string{"FallbackAbility", "AtRandom", "Random", "ShowChoice", "ChoiceRestriction", "NumRandomChoices"} {
		if _, ok := ps.get(k); ok {
			return refuse("GenericChoice of the controller with %s$", k)
		}
	}
	choices := strings.Split(ps.str("Choices"), ",")
	if len(choices) < 2 {
		return refuse("GenericChoice with %d choices", len(choices))
	}
	for _, name := range choices {
		sa := cards.ResolveSVar(lc.face.SVars, strings.TrimSpace(name))
		if sa == nil {
			return refuse("GenericChoice branch %q", name)
		}
	}
	if _, ok := ps.get("ValidTgts"); ok {
		// the choice's own target (a branch's Defined$ Targeted)
		if _, err := lc.objOrTarget(ps, "", zBattlefield); err != nil {
			return err
		}
	}
	if lc.ab.Kind == akTriggered {
		if lc.mode >= 0 || lc.ab.Modes != 0 || !lc.head || lc.depth != 1 {
			return refuse("GenericChoice of the controller below a triggered ability's head")
		}
		return lc.charm(ps)
	}
	if lc.ab.Kind != akSpell && lc.ab.Kind != akActivated {
		return refuse("GenericChoice of the controller in a %d ability", lc.ab.Kind)
	}
	at := len(lc.ab.Prog)
	// C 1: every branch is offered, payable or not (gorge effCharm's
	// resolution KModes does not filter an UnlessCost$ choice: Highway
	// Robbery's discard with an empty hand)
	lc.emit(lInstr{Op: "GenericChoice", Obj: lRef{Kind: rYou}, A: cst(len(choices)), C: 1})
	for i, name := range choices {
		sa := cards.ResolveSVar(lc.face.SVars, strings.TrimSpace(name))
		bat := len(lc.ab.Prog)
		lc.emit(lInstr{Op: "Branch", C: i})
		if err := lc.chain(sa, nil); err != nil {
			return err
		}
		lc.ab.Prog[bat].Skip = len(lc.ab.Prog) - bat - 1
	}
	_ = at
	return nil
}
