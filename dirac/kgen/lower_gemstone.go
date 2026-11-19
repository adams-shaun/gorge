package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// Gemstone Caverns' opening-hand keyword (CR 103.6a: "if this is in your
// opening hand and you're not the starting player, you may begin the game
// with it on the battlefield with a luck counter on it; if you do, exile a
// card from your hand") is gorge's Gemstone arm of rules/opening_hand.go: the
// !PlayFirst gate skips the starting player (newOpeningRound), the luck
// counter is put by the pregame PutCounter sub, and the mandatory hand exile
// is a KChoose over the hand (opening_exile). It lowers onto the kernel's
// Leyline ask (turn_gen.go leylineStart, CardDef.Leyline) with the Gemstone
// marks (CardDef.LeylineGem, LeylineCtr/LeylineCtrN; kernel zone_gemstone.go):
// the ask skips the starting player, the card enters with the counters, and
// its player picks the exiled hand card.
//
// Its mana ability lowers on its own (lower_condsplit.go: a Condition-split
// mana source, as River of Tears').

// lowerOpeningGated lowers a MayEffectFromOpeningHand keyword whose action is
// not the plain FromHand self-to-battlefield move, or returns the refusal.
// Only the Gemstone shape lowers: the !PlayFirst gate on a Self Hand ->
// Battlefield ChangeZone, a literal PutCounter on Self, then a mandatory
// one-card Hand -> Exile ChangeZone of any card.
func (lc *lowerCtx) lowerOpeningGated(param string) string {
	name, req, _ := strings.Cut(param, ":")
	name = strings.TrimSpace(name)
	if strings.TrimSpace(req) != "!PlayFirst" {
		return "opening-hand effect " + strings.TrimSpace(param) + ": the kernel's Leyline ask models only the plain FromHand self-to-battlefield move"
	}
	gated := "opening-hand effect gated on not being the starting player (" + strings.TrimSpace(param) + "): "
	sa := cards.ResolveSVar(lc.face.SVars, name)
	if sa == nil || sa.API != "ChangeZone" {
		return gated + "the action is not a Self Hand -> Battlefield move"
	}
	fp := newParams(sa.Params)
	if fp.str("Defined") != "Self" || fp.str("Origin") != "Hand" || fp.str("Destination") != "Battlefield" {
		return gated + "the action is not a Self Hand -> Battlefield move"
	}
	fp.consume("SubAbility")
	if u := fp.unread(); len(u) != 0 {
		return gated + "unread params " + strings.Join(u, " ")
	}
	pc := sa.Sub
	if pc == nil || pc.API != "PutCounter" {
		return gated + "no PutCounter sub (only Gemstone Caverns' luck-counter shape lowers)"
	}
	pp := newParams(pc.Params)
	ck, err := counterKind(pp.str("CounterType"))
	n := pp.str("CounterNum")
	if err != nil || pp.str("Defined") != "Self" || (n != "1" && n != "2" && n != "3") {
		return gated + "PutCounter is not a literal named counter on Self"
	}
	pp.consume("SubAbility")
	if u := pp.unread(); len(u) != 0 {
		return gated + "PutCounter unread params " + strings.Join(u, " ")
	}
	ex := pc.Sub
	if ex == nil || ex.API != "ChangeZone" || ex.Sub != nil {
		return gated + "no mandatory hand exile after the counter"
	}
	ep := newParams(ex.Params)
	if ep.str("Origin") != "Hand" || ep.str("Destination") != "Exile" || ep.str("ChangeType") != "Card" ||
		ep.str("ChangeNum") != "1" || ep.str("Mandatory") != "True" {
		return gated + "the exile is not a mandatory one-card hand exile"
	}
	if u := ep.unread(); len(u) != 0 {
		return gated + "exile unread params " + strings.Join(u, " ")
	}
	lc.out.Leyline = true
	lc.out.LeylineGem = true
	lc.out.LeylineCtr = ck
	lc.out.LeylineCtrN = uint8(n[0] - '0')
	return ""
}
