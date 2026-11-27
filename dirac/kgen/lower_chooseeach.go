package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// chooseEachGroupTypes maps a ChooseEach$ group word to its permanent type, in
// the kernel's walk order (dirac chooseEachTypes): the groups must be spelled
// in this order, because the kernel asks each chooser the types in it and
// gorge asks in the ChooseEach$ order.
var chooseEachGroupTypes = []struct {
	word string
	t    kop.TypeMask
}{
	{"Artifact", kop.TArtifact}, {"Battle", kop.TBattle}, {"Creature", kop.TCreature},
	{"Enchantment", kop.TEnchantment}, {"Land", kop.TLand}, {"Planeswalker", kop.TPlaneswalker},
}

// chooseEachType lowers ChooseCard with ChooseEach$ (gorge effChooseCard's
// per-group walk: each chooser picks one card per group, a card eligible for
// several groups pickable once per group): Liliana, Dreadhorde General's -9
// (each opponent, every permanent type, Choices$ Permanent) and Cataclysmic
// Gearhulk's ETB (each player, an artifact, a creature, an enchantment and a
// planeswalker from among the nonland permanents they control), the
// SacrificeAll of the nonchosen permanents following. Every other shape
// refuses by name.
func (lc *lowerCtx) chooseEachType(ps *params, each string) error {
	var who lRef
	switch ps.str("Defined") {
	case "Opponent":
		who = lRef{Kind: rEachOpp}
	case "Player":
		who = lRef{Kind: rEachPlayer}
	default:
		return refuse("ChooseEach shape")
	}
	nonLand := false
	switch ps.str("Choices") {
	case "Permanent":
	case "Permanent.nonLand":
		nonLand = true
	default:
		return refuse("ChooseEach shape")
	}
	if ps.str("ControlledByPlayer") != "Chooser" {
		return refuse("ChooseEach shape")
	}
	var mask kop.TypeMask
	at := 0
	for _, g := range strings.Split(each, " & ") {
		i := at
		for i < len(chooseEachGroupTypes) && chooseEachGroupTypes[i].word != strings.TrimSpace(g) {
			i++
		}
		if i == len(chooseEachGroupTypes) {
			return refuse("ChooseEach shape")
		}
		mask |= chooseEachGroupTypes[i].t
		at = i + 1
	}
	if nonLand && mask&kop.TLand != 0 {
		return refuse("ChooseEach shape") // a Land group of a nonland pool is never offered
	}
	const all = kop.TArtifact | kop.TBattle | kop.TCreature | kop.TEnchantment | kop.TLand | kop.TPlaneswalker
	in := lInstr{Op: "ChooseEachType", Obj: who}
	if nonLand {
		in.C = kop.ChooseEachNonLand
	}
	if mask != all {
		in.A = cst(int(mask))
	}
	ps.consume("Mandatory", "Reveal")
	lc.emit(in)
	return nil
}

// chooseCardEachAmount lowers a mandatory each-player ChooseCard whose pool is
// each chooser's own objects (Defined$ Player, ControlledByPlayer$ Chooser):
// Planetary Annihilation's "each player chooses six lands they control". gorge
// asks each chooser (AliveFrom(controller)) for Amount$ cards from the
// chooser's own pool (min = max = Amount$, clamped to the pool), and
// SacrificeAll of the nonchosen lands follows. The kernel's
// kop.ChooseCardEachAmount matches the pool from the asker, so the filter's
// YouCtrl is the chooser's. ok false: not this shape (the ordinary path
// refuses it by name).
func (lc *lowerCtx) chooseCardEachAmount(ps *params) (bool, error) {
	if _, bad := ps.get("ValidTgts"); bad || lc.repeatPl > 0 {
		return false, nil
	}
	for _, k := range []string{"AtRandom", "StartingWith", "ForgetOtherRemembered", "RememberChosen", "ForgetChosen", "MinAmount", "ChoiceZone", "Optional", "WithTotalPower", "DefinedCards"} {
		if _, ok := ps.get(k); ok {
			return false, nil
		}
	}
	amt, err := lc.amount(ps, "Amount", 1)
	if err != nil {
		return true, err
	}
	if !amt.isConst() || amt.N < 1 {
		return false, nil
	}
	f, err := lc.filter(addFilterProp(ps.str("Choices"), "YouCtrl"))
	if err != nil {
		return true, err
	}
	for _, k := range []string{"Reveal", "Secretly", "ControlAndNot"} {
		if v, ok := ps.get(k); ok && !strings.EqualFold(strings.TrimSpace(v), "True") {
			return true, refuse("ChooseCard %s$ %q", k, v)
		}
	}
	ps.consume("Defined", "Choices", "ControlledByPlayer", "Mandatory", "Amount", "ChoiceTitle", "Reveal", "RevealTitle", "AILogic")
	lc.emit(lInstr{Op: "ChooseCard", Obj: lRef{Kind: rEachPlayer}, F: f, C: zBattlefield, A: amt, CS: "EachAmount"})
	return true, nil
}

// chooseCardCarrier lowers a ChooseCard that only carries its card target
// (Together Forever's "{1}: Choose target creature with a counter on it. When
// that creature dies this turn, ..."): ValidTgts$ names a card filter and no
// Defined$ chooser, Choices$ or bound is given. gorge chooseCardTargetsOnly
// reads the targeted PLAYERS alone as the choosers (CR 115.1), a card target
// makes nobody one, so the resolution asks nothing and records nothing; the
// target slot rides to the chain's later reads (a delayed trigger's
// RememberObjects$ Targeted). ok false: not this shape.
func (lc *lowerCtx) chooseCardCarrier(ps *params) (bool, error) {
	vt, ok := ps.get("ValidTgts")
	if !ok || strings.TrimSpace(vt) == "Opponent" {
		return false, nil
	}
	for _, k := range []string{"Defined", "Choices", "Mandatory", "Optional", "Amount", "MinAmount", "ChoiceZone", "ControlledByPlayer", "ChooseEach", "AtRandom", "RememberChosen", "ForgetChosen", "ForgetOtherRemembered", "StartingWith", "TargetMin", "TargetMax", "TgtZone", "WithTotalPower", "DefinedCards", "Reveal", "Secretly"} {
		if _, bad := ps.get(k); bad {
			return false, nil
		}
	}
	// the target must be a card filter: a player target would make its player the chooser
	f, err := lc.filter(vt)
	if err != nil {
		return true, err
	}
	if f.Players != 0 || len(f.Clauses) == 0 {
		return false, nil
	}
	if _, err := lc.objOrTarget(ps, "", zBattlefield); err != nil {
		return true, err
	}
	ps.consume("TgtPrompt")
	return true, nil
}
