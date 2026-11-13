package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The as-enters choice of another creature and the ability that reads it
// (Dauntless Bodyguard): K:ETBReplacement:Other:<SVar> with DB$ ChooseCard |
// Defined$ You | Choices$ Creature.YouCtrl+Other | Mandatory$ True, and
// A:AB$ Pump | Cost$ Sac<1/CARDNAME> | Defined$ Valid Card.ChosenCardStrict.
// gorge runs the body as the permanent enters (effChooseCard: a KChoose of
// "card" options, none posed for an empty pool) and keeps the answer on
// Object.Chosen; the ChosenCard / ChosenCardStrict filter arm reads it,
// through the source's last known choice once the sacrifice moved it
// (effects matchPositive, TriggerContext.SourceChosen). The kernel lowers
// the body to the enters program's only instruction, OpExt
// ExtChooseCardEnter over the Choices$ filter, keeping the answer in the
// permanent's chosen byte, and seeds the card's resolutions' chosen set
// from it (dirac/repl_choosecard.go). Only this exact body lowers: another
// Defined$, Choices$ or an optional choice, a chain, or a choice beside
// another enters part refuse. ChosenCardStrict stays an unknown filter word
// elsewhere: the Pump's spelling is read as the chosen referent
// (chosenStrictPump), and the other carriers (Azra Oddsmaker's emblem
// trigger's ValidSource$) wait on their own gorge fixes.

// enterCardChoices is the one Choices$ spelling the as-enters object choice
// lowers.
const enterCardChoices = "Creature.YouCtrl+Other"

// etbChooseCard lowers an ETB replacement's ChooseCard node: the entering
// permanent's own, unconditional, sole, mandatory choice of another creature
// its controller controls.
func (lc *lowerCtx) etbChooseCard(ps *params, node, sa *cards.SA, other bool, cond *lCond) error {
	if other || node != sa || sa.Sub != nil || cond.Kind != cNone || len(lc.ab.Prog) != 0 {
		return refuse("ETB ChooseCard beside another ETB replacement part")
	}
	if d, ok := ps.get("Defined"); !ok || strings.TrimSpace(d) != "You" {
		return refuse("ETB ChooseCard Defined$ %q", d)
	}
	if v := strings.TrimSpace(ps.str("Choices")); v != enterCardChoices {
		return refuse("ETB ChooseCard Choices$ %q", v)
	}
	if !ps.is("Mandatory", "True") {
		return refuse("ETB ChooseCard without Mandatory$ True")
	}
	f, err := lc.filter(enterCardChoices)
	if err != nil {
		return err
	}
	ps.consume("AILogic", "SpellDescription")
	lc.emit(lInstr{Op: "ChooseCardEnter", Obj: lRef{Kind: rYou}, F: f, Note: "choice of another creature you control as it enters"})
	return nil
}

// buildChooseCardEnter is OpExt ExtChooseCardEnter; the caller compiles the
// battlefield filter into F.
func buildChooseCardEnter(ki *kop.Instr) {
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtChooseCardEnter))
}

// entersChoosingCard reports whether face f chooses one object as it enters
// through the ETB ChooseCard body etbChooseCard lowers.
func entersChoosingCard(f *cards.Face) bool {
	if f == nil {
		return false
	}
	for i := range f.Repls {
		r := &f.Repls[i]
		if r.ParamStr(cards.PKKeyword) == "ETBReplacement" && r.With != nil && r.With.API == "ChooseCard" &&
			r.With.Sub == nil && strings.TrimSpace(r.With.Params["Choices"]) == enterCardChoices {
			return true
		}
	}
	return false
}

// chosenStrictPump reads a Pump's Defined$ Valid Card.ChosenCardStrict on a
// card that chooses one object as it enters as Defined$ ChosenCard: gorge's
// two spellings name the same set (TestDauntlessBodyguardSacrificeProtects
// runs both), and the set is that one object. The single referent grants the
// keyword to the object whatever its types, as gorge's Pump does; the
// Valid sweep (OpPumpAllEach) pumps creatures only.
func (lc *lowerCtx) chosenStrictPump(ps *params) {
	if d := strings.TrimSpace(ps.m["Defined"]); d == "Valid Card.ChosenCardStrict" && entersChoosingCard(lc.face) {
		ps.m["Defined"] = "ChosenCard"
	}
}
