package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The as-enters nonland card-name choice and the trigger that reads it (Kai
// Winn, Faithless Leader): K:ETBReplacement:Other:<SVar> with DB$ NameCard |
// Defined$ You | ValidCards$ Card.nonLand, and T: Mode$ SpellAbilityCast |
// ValidCard$ Card.NamedCard | ValidSA$ Spell,Activated |
// ValidActivatingPlayer$ Opponent. gorge asks the name at the entry boundary
// over the sorted nonland names of its universe (effects
// NameChoicesFromListCtx) and keeps it on Object.ChosenName; the line reads
// it through the NamedCard predicate on the cast spell, or on the source of an
// activated ability (trigmatch spellAbilityCastSpellMatches /
// abilityCastMatches). The kernel lowers the choice to the enters program's
// first instruction, OpExt ExtChooseName, keeping the answer in the
// permanent's chosen byte as a CardID (dirac/repl_choosename.go), and the
// trigger to a SpellCast line carrying Tf4NamedCast, which also listens for
// activations. Only these exact spellings lower: another ValidCards$, a
// Defined$ other than You, a Sub chain, a ValidCard$ other than NamedCard, or
// a trigger beside a card that does not name a card as it enters all refuse.

// entersNamingCard reports whether face f names a card as it enters through
// an ETBReplacement NameCard.
func entersNamingCard(f *cards.Face) bool {
	if f == nil {
		return false
	}
	for i := range f.Repls {
		r := &f.Repls[i]
		if r.With != nil && r.With.API == "NameCard" && r.ParamStr(cards.PKKeyword) == "ETBReplacement" {
			return true
		}
	}
	return false
}

// etbChooseName lowers an ETB replacement's NameCard node: the entering
// permanent's own, unconditional, sole choice among the nonland cards.
func (lc *lowerCtx) etbChooseName(ps *params, node, sa *cards.SA, other bool, cond *lCond) error {
	if other || node != sa || sa.Sub != nil || cond.Kind != cNone || len(lc.ab.Prog) != 0 {
		return refuse("ETB NameCard beside another ETB replacement part")
	}
	if d, ok := ps.get("Defined"); !ok || strings.TrimSpace(d) != "You" {
		return refuse("ETB NameCard Defined$ %q", d)
	}
	if v := ps.str("ValidCards"); v != "Card.nonLand" {
		return refuse("ETB NameCard ValidCards$ %q", v)
	}
	ps.consume("ValidDescription", "SpellDescription")
	lc.emit(lInstr{Op: "ChooseName", Obj: lRef{Kind: rYou}, Note: "choice of a nonland card name as it enters"})
	return nil
}

// namedCastTrigger lowers Mode$ SpellAbilityCast over the source's chosen
// name into a SpellCast line with the named-cast flag (kop.Tf4NamedCast).
func (lc *lowerCtx) namedCastTrigger(tp *params, tr *lTrigger, zone string) error {
	if zone != "" {
		return refuse("SpellAbilityCast trigger on an Effect")
	}
	if tr.Zone != 0 && tr.Zone != zBattlefield {
		return refuse("SpellAbilityCast TriggerZones %q", tr.Zone)
	}
	if v := tp.str("ValidCard"); v != "Card.NamedCard" {
		return refuse("SpellAbilityCast ValidCard$ %q", v)
	}
	if v := tp.str("ValidSA"); v != "Spell,Activated" {
		return refuse("SpellAbilityCast ValidSA$ %q", v)
	}
	if v := tp.str("ValidActivatingPlayer"); v != "Opponent" {
		return refuse("SpellAbilityCast ValidActivatingPlayer$ %q", v)
	}
	if !entersNamingCard(lc.face) {
		return refuse("SpellAbilityCast Card.NamedCard beside no as-enters NameCard")
	}
	tr.Event, tr.Whose, tr.NamedCast = "SpellCast", whoOpp, true
	return nil
}

// buildChooseName is OpExt ExtChooseName (the first instruction of an enters
// program).
func buildChooseName(ki *kop.Instr) {
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtChooseName))
}
