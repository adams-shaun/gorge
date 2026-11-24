package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The as-enters nonland card-name choice and the trigger that reads it (Kai
// Winn, Faithless Leader): an as-enters replacement whose body has its
// controller name a nonland card, and a SpellAbilityCast trigger on an
// opponent casting a spell or activating an ability of a card with the chosen
// name. gorge asks the name at the entry boundary
// over the sorted nonland names of its universe (effects
// NameChoicesFromListCtx) and keeps it on Object.ChosenName; the line reads
// it through the NamedCard predicate on the cast spell, or on the source of an
// activated ability (trigmatch spellAbilityCastSpellMatches /
// abilityCastMatches). The kernel lowers the choice to the enters program's
// first instruction, OpExt ExtChooseName, keeping the answer in the
// permanent's chosen byte as a CardID (dirac/repl_choosename.go), and the
// trigger to a SpellCast line carrying Tf4NamedCast, which also listens for
// activations. Only these exact spellings lower: another ValidCards$, a
// Defined$ other than You, a Sub chain (bar Spyglass's and Alhammarret's, High
// Arbiter's reveal-then-name chains), a ValidCard$ other than NamedCard, or
// a trigger beside a card that does not name a card as it enters all refuse.

// entersNamingCard reports whether face f names a card as it enters through
// an ETBReplacement NameCard.
func entersNamingCard(f *cards.Face) bool {
	if f == nil {
		return false
	}
	for i := range f.Repls {
		r := &f.Repls[i]
		if r.ParamStr(cards.PKKeyword) != "ETBReplacement" {
			continue
		}
		for a := r.With; a != nil; a = a.Sub {
			if a.API == "NameCard" {
				return true
			}
		}
	}
	return false
}

// spyglassChain reports whether sa is exactly Sorcerous Spyglass's as-enters
// chain: ChoosePlayer -> RevealHand (Look$ True over the chosen player) ->
// NameCard -> Cleanup (ClearChosenPlayer$ True). The chosen player is one of
// the opponents (choosePlayerPod), the look is gorge's look_ack (a decision,
// no state), and the name is any card's, lands included.
func spyglassChain(sa *cards.SA) bool {
	if sa == nil || sa.API != "ChoosePlayer" {
		return false
	}
	rh := sa.Sub
	if rh == nil || rh.API != "RevealHand" || rh.Params["Defined"] != "ChosenPlayer" || !strings.EqualFold(rh.Params["Look"], "True") {
		return false
	}
	nc := rh.Sub
	if nc == nil || nc.API != "NameCard" || nc.Sub == nil || nc.Sub.API != "Cleanup" || nc.Sub.Sub != nil {
		return false
	}
	return true
}

// lookChooseTypeChain reports whether sa is the as-enters look-then-type
// chain (Arachne, Psionic Weaver): ChoosePlayer -> RevealHand (Look$ True over
// the chosen player) -> ChooseType (Type$ Card) -> Cleanup
// (ClearChosenPlayer$ True). It is the Spyglass chain's shape with a card-type
// choice in place of the name.
func lookChooseTypeChain(sa *cards.SA) bool {
	if sa == nil || sa.API != "ChoosePlayer" {
		return false
	}
	rh := sa.Sub
	if rh == nil || rh.API != "RevealHand" || rh.Params["Defined"] != "ChosenPlayer" || !strings.EqualFold(rh.Params["Look"], "True") {
		return false
	}
	ct := rh.Sub
	if ct == nil || ct.API != "ChooseType" || ct.Params["Type"] != "Card" || ct.Sub == nil || ct.Sub.API != "Cleanup" || ct.Sub.Sub != nil {
		return false
	}
	return strings.EqualFold(ct.Sub.Params["ClearChosenPlayer"], "True")
}

// handNameChain reports whether sa is exactly Alhammarret, High Arbiter's
// as-enters chain: RevealHand (Defined$ Player.Opponent, RememberRevealed$
// True, no Look$) -> NameCard (Defined$ You, ValidCards$ Card.nonLand,
// ChooseFromDefinedCards$ Remembered) -> Cleanup (ClearRemembered$ True). The
// reveal is public (gorge poses no decision) and the name is one of the
// nonland cards it showed, so the kernel folds the reveal into the name ask's
// options (ExtChooseName B = 3) and poses no ask when none was shown.
func handNameChain(sa *cards.SA) bool {
	if sa == nil || sa.API != "RevealHand" || sa.Params["Defined"] != "Player.Opponent" || !strings.EqualFold(sa.Params["RememberRevealed"], "True") {
		return false
	}
	if _, look := sa.Params["Look"]; look {
		return false
	}
	nc := sa.Sub
	if nc == nil || nc.API != "NameCard" || nc.Params["ChooseFromDefinedCards"] != "Remembered" {
		return false
	}
	cl := nc.Sub
	return cl != nil && cl.API == "Cleanup" && cl.Sub == nil && strings.EqualFold(cl.Params["ClearRemembered"], "True")
}

// etbChooseName lowers an ETB replacement's NameCard node: the entering
// permanent's own, unconditional, sole choice among the nonland cards. The
// Spyglass chain (lower_choosename.go spyglassChain) lowers the chain's own
// links beside it — the pod-wide opponent choice (choosePlayerPod) is ahead
// of it in the same program.
func (lc *lowerCtx) etbChooseName(ps *params, node, sa *cards.SA, other bool, cond *lCond) error {
	chain, hand := spyglassChain(sa), handNameChain(sa)
	if other || cond.Kind != cNone ||
		!chain && !hand && (len(lc.ab.Prog) != 0 || node != sa || sa.Sub != nil) {
		return refuse("ETB NameCard beside another ETB replacement part")
	}
	if d, ok := ps.get("Defined"); !ok || strings.TrimSpace(d) != "You" {
		return refuse("ETB NameCard Defined$ %q", d)
	}
	in := lInstr{Op: "ChooseName", Obj: lRef{Kind: rYou}, Note: "choice of a nonland card name as it enters"}
	if chain {
		// Spyglass: any card name (no ValidCards$), after the hand look (B = 2)
		if v, ok := ps.get("ValidCards"); ok {
			return refuse("ETB NameCard ValidCards$ %q beside the look chain", v)
		}
		ps.consume("AILogic")
		in.B, in.Note = cst(2), "choice of any card name as it enters, after the look at the chosen opponent's hand"
	} else if v := ps.str("ValidCards"); v != "Card.nonLand" {
		return refuse("ETB NameCard ValidCards$ %q", v)
	}
	if hand {
		// Alhammarret: only the nonland cards the opponents revealed (B = 3)
		ps.consume("ChooseFromDefinedCards")
		in.B, in.Note = cst(3), "choice of a nonland card name among the opponents' revealed hands as it enters"
	}
	ps.consume("ValidDescription", "SpellDescription")
	lc.emit(in)
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
func buildChooseName(ki *kop.Instr, mode int) {
	ki.Op, ki.A, ki.B = kop.OpExt, kop.Const(int(kop.ExtChooseName)), kop.Const(mode)
}
