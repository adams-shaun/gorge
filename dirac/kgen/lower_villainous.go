package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// villainousChoice lowers api:VillainousChoice (gorge effVillainousChoice,
// engine/effects/charm.go): each victim, in Defined$ order, picks one of the
// Choices$ branches (a KModes ask over EVERY choice, payable or not: C 1 on
// the kernel's choose-branch op) and that branch runs with the victim
// remembered ("Defined$ Remembered" in a body is the victim). A victim that
// is a target (ValidTgts$ with no Defined$) is the sole chooser; the choice
// is the same single ask.
//
// The shape is GenericChoice's per-player loop (lower_zone.go genericChoice,
// TempRemember Chooser) minus its fallback branch: a RepeatEach over the
// victims, the choose-branch op for the loop player, and one Branch block per
// choice whose Remembered reads are the loop player.
func (lc *lowerCtx) villainousChoice(ps *params) error {
	for _, k := range []string{"AtRandom", "Random", "ShowChoice", "ChoiceRestriction", "NumRandomChoices", "FallbackAbility"} {
		if _, ok := ps.get(k); ok {
			return refuse("VillainousChoice with %s$", k)
		}
	}
	choices := strings.Split(ps.str("Choices"), ",")
	if len(choices) < 2 {
		return refuse("VillainousChoice with %d choices", len(choices))
	}
	for i, name := range choices {
		choices[i] = strings.TrimSpace(name)
		if cards.ResolveSVar(lc.face.SVars, choices[i]) == nil {
			return refuse("VillainousChoice branch %q", name)
		}
	}
	if _, ok := ps.get("Defined"); !ok {
		return lc.villainousTargeted(ps, choices)
	}
	if _, ok := ps.get("ValidTgts"); ok {
		return refuse("VillainousChoice with both Defined$ and ValidTgts$")
	}
	who, err := lc.defined(ps.str("Defined"))
	if err != nil {
		return err
	}
	at := len(lc.ab.Prog)
	lc.emit(lInstr{Op: "RepeatEach", Obj: who})
	lc.emit(lInstr{Op: "GenericChoice", Obj: lRef{Kind: rEach}, A: cst(len(choices)), C: 1})
	lc.repeatPl++ // Player.IsRemembered in a body is the victim, as in a RepeatEach body
	err = lc.villainousBranches(choices)
	lc.repeatPl--
	if err != nil {
		return err
	}
	if err := lc.villainousRemember(at, lRef{Kind: rEach}); err != nil {
		return err
	}
	lc.ab.Prog[at].Skip = len(lc.ab.Prog) - at - 1
	return nil
}

// villainousTargeted is the ValidTgts$ shape: the one targeted player chooses.
func (lc *lowerCtx) villainousTargeted(ps *params, choices []string) error {
	if lc.ab.Kind != akTriggered && lc.ab.Kind != akSpell && lc.ab.Kind != akActivated {
		return refuse("VillainousChoice of a target player in a %d ability", lc.ab.Kind)
	}
	obj, err := lc.objOrTarget(ps, "", zBattlefield)
	if err != nil {
		return err
	}
	if obj.Kind != rTarget {
		return refuse("VillainousChoice of a target player without a target slot")
	}
	t := lc.ab.Targets[obj.N]
	if t.F == nil || t.F.Players == 0 || len(t.F.Clauses) != 0 || t.AtRes ||
		!t.Min.isConst() || t.Min.N != 1 || !t.Max.isConst() || t.Max.N != 1 {
		return refuse("VillainousChoice of a target player without one mandatory player target")
	}
	at := len(lc.ab.Prog)
	lc.emit(lInstr{Op: "GenericChoice", Obj: obj, A: cst(len(choices)), C: 1})
	if err := lc.villainousBranches(choices); err != nil {
		return err
	}
	return lc.villainousRemember(at, obj)
}

// villainousBranches emits one Branch block per choice.
func (lc *lowerCtx) villainousBranches(choices []string) error {
	for i, name := range choices {
		bat := len(lc.ab.Prog)
		lc.emit(lInstr{Op: "Branch", C: i})
		if err := lc.chain(cards.ResolveSVar(lc.face.SVars, name), nil); err != nil {
			return err
		}
		lc.ab.Prog[bat].Skip = len(lc.ab.Prog) - bat - 1
	}
	return nil
}

// villainousRemember rewrites the Remembered reads from instruction at on to
// the chooser: gorge remembers the victim for the chosen body. A Remembered
// read after the branch has itself remembered cards (RememberFound$,
// RememberChanged$ ...) is gorge's mixed list of the victim and those cards
// (a player define takes the victim, a card define the cards), which the
// kernel's card-only remembered set cannot tell apart: the card refuses.
func (lc *lowerCtx) villainousRemember(at int, victim lRef) error {
	remembering := false
	for i := at; i < len(lc.ab.Prog); i++ {
		in := &lc.ab.Prog[i]
		switch {
		case in.Op == "Branch":
			remembering = false
		case in.Op == "ClearRemembered":
			remembering = false
		case in.Obj.Kind == rRemembered && remembering:
			return refuse("VillainousChoice body reads Remembered after remembering cards (%s)", in.Op)
		case in.Obj.Kind == rRemembered:
			in.Obj = victim
		}
		if strings.HasPrefix(in.Op, "Remember") || strings.Contains(in.CS, "Remember") {
			remembering = true
		}
	}
	return nil
}
