package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// repeatOptional lowers an ungated RepeatOptional$ True api:Repeat (gorge
// effRepeat's open-ended do/while; Ad Nauseam). The body runs once; after
// every pass that changed something the resolving controller is asked
// "Repeat this process?" (repeat_optional, yes / no), and a pass that posed
// no decision and changed nothing ends the loop unasked (gorge's CR 732.2a
// shortcut).
//
// The kernel cannot watch a pass for gorge's "changed nothing"
// (rules.Engine.StateChangedSince), so the class is lowered by body shape,
// one whose change test the kernel can compute (dirac/act_repeatoptional.go):
// Dig DigNum$ 1 ChangeNum$ All of your own library with no decision, then
// LoseLife of your own, then Cleanup (Ad Nauseam). A pass over a card moves
// it. A pass over an empty library reveals nothing (a Note) and moves
// nothing; its 0 life loss is no LifeChange, but the AFLifeLost StoreSVar
// LoseLife always writes counts as a change unless it repeats the previous
// pass's value, which the head op reads off the kernel's last life loss.
//
// Any other body shape, a RepeatOptionalDecider$, a gate beside the election
// (Fiery Gambit, Delina) or a MaxRepeat$ cap stays refused. The loop count is
// the kernel's uint8 (255 passes); gorge's malformed-input cap is 1000, which
// no library reaches.
func (lc *lowerCtx) repeatOptional(ps *params) error {
	for _, k := range []string{"RepeatNum", "MaxRepeat", "RepeatCheckSVar", "RepeatDefined", "RepeatOptionalDecider", "RepeatOptionalForEachPlayer", "ValidTgts"} {
		if _, ok := ps.get(k); ok {
			return refuse("Repeat RepeatOptional$ beside %s$", k)
		}
	}
	if !strings.EqualFold(strings.TrimSpace(ps.str("RepeatOptional")), "True") {
		return refuse("Repeat RepeatOptional$ %q", ps.str("RepeatOptional"))
	}
	body := cards.ResolveSVar(lc.face.SVars, ps.str("RepeatSubAbility"))
	if body == nil {
		return refuse("Repeat body %q", ps.str("RepeatSubAbility"))
	}
	pre, ok := repeatOptionalShape(body)
	pure := false
	if !ok {
		pre, ok = repeatOptionalDigExile(body)
		pure = ok
	}
	if !ok {
		return refuse("Repeat RepeatOptional$ body shape %s$ (no change test)", body.API)
	}
	at := len(lc.ab.Prog)
	always := lCond{Kind: cValue, A: lValue{Kind: vLibrarySizeOf, Ref: lRef{Kind: rYou}}, Op: opGE, B: cst(0)}
	lc.emit(lInstr{Op: "RepeatWhile", A: cst(255), Cond: always, Note: "RepeatOptional$: again while the controller says yes"})
	headB := 0
	if pure {
		headB = 2 // no life-loss write beside the dig: an empty pass changes nothing, first or not
	}
	lc.emit(lInstr{Op: "RepeatOptional", B: cst(headB), Cond: pre, Note: "a pass that changes nothing is the last"})
	if err := lc.chain(body, nil); err != nil {
		return err
	}
	lc.emit(lInstr{Op: "RepeatOptional", B: cst(1), Note: "Repeat this process? (repeat_optional)"})
	lc.ab.Prog[at].Skip = len(lc.ab.Prog) - at - 1
	return nil
}

// repeatOptionalShape matches Ad Nauseam's body and returns the pass's
// progress condition: your library holds a card.
func repeatOptionalShape(n *cards.SA) (lCond, bool) {
	you := func(p string) bool { p = strings.TrimSpace(p); return p == "" || p == "You" }
	if n == nil || n.API != "Dig" || !you(n.Params["Defined"]) || strings.TrimSpace(n.Params["DigNum"]) != "1" ||
		n.Params["ChangeNum"] != "All" || n.Params["ChangeValid"] != "Card" || n.Params["Optional"] != "" || n.Params["RevealOptional"] != "" {
		return lCond{}, false
	}
	l := n.Sub
	if l == nil || l.API != "LoseLife" || !you(l.Params["Defined"]) {
		return lCond{}, false
	}
	if c := l.Sub; c == nil || c.API != "Cleanup" || c.Sub != nil {
		return lCond{}, false
	}
	return lCond{Kind: cValue, A: lValue{Kind: vLibrarySizeOf, Ref: lRef{Kind: rYou}}, Op: opGE, B: cst(1)}, true
}

// repeatOptionalDigExile matches Dance with Calamity's body: one Dig of your
// own top card, ChangeNum$ All, to exile with RememberChanged$ and nothing
// else (no LoseLife StoreSVar write). A pass over a card moves it, a change;
// a pass over an empty library moves nothing and writes nothing, so it is the
// last, first pass or not (the head op's B 2). The Dig lowering reads every
// parameter and refuses what it does not model.
func repeatOptionalDigExile(n *cards.SA) (lCond, bool) {
	you := func(p string) bool { p = strings.TrimSpace(p); return p == "" || p == "You" }
	if n == nil || n.API != "Dig" || n.Sub != nil || !you(n.Params["Defined"]) || strings.TrimSpace(n.Params["DigNum"]) != "1" ||
		n.Params["ChangeNum"] != "All" || n.Params["ChangeValid"] != "Card" || n.Params["Optional"] != "" || n.Params["RevealOptional"] != "" ||
		n.Params["DestinationZone"] != "Exile" || n.Params["RememberChanged"] != "True" {
		return lCond{}, false
	}
	return lCond{Kind: cValue, A: lValue{Kind: vLibrarySizeOf, Ref: lRef{Kind: rYou}}, Op: opGE, B: cst(1)}, true
}
