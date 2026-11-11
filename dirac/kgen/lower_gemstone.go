package kgen

import "strings"

// Gemstone Caverns stays refused, by name, on both of its nodes; the reasons
// below say what the kernel lacks (each is a ticket of its own).
//
// Its opening-hand keyword (CR 103.6a: "if this is in your opening hand and
// you're not the starting player, you may begin the game with it on the
// battlefield with a luck counter on it; if you do, exile a card from your
// hand") is gorge's Gemstone arm of rules/opening_hand.go: the !PlayFirst gate
// skips the starting player (newOpeningRound), the luck counter is put by the
// pregame PutCounter sub, and the mandatory hand exile is a KChoose over the
// hand (opening_exile). The kernel's Leyline ask (turn_gen.go leylineStart,
// CardDef.Leyline) only moves a card from hand to battlefield, for every
// player: no gate, no counter, no follow-up card choice.
//
// Its mana ability ("{T}: Add {C}. If this has a luck counter on it, instead
// add one mana of any color") is an AB$ Mana whose production is split across
// a Condition*-gated root and a Condition*-gated DB$ Mana sub (River of Tears
// has the same split on lands played). gorge's planner defers such a source
// (pay.PaymentPlanShapeTierOf: source:conditional), its relaxed census unit is
// the root's production alone ({C}, even with a luck counter), and a lucky
// source's sub asks a colour inside the prefix search and the mana window.
// The kernel's only deferred mana source is Pyromancer's Goggles' fixed,
// artifact-typed {R} with no ask (mana_deferred.go).

// refuseOpeningGated is the refusal of a MayEffectFromOpeningHand keyword
// whose action is not the plain FromHand self-to-battlefield move.
func refuseOpeningGated(param string) string {
	if _, req, _ := strings.Cut(param, ":"); strings.Contains(req, "!PlayFirst") {
		return "opening-hand effect gated on not being the starting player (!PlayFirst; Gemstone Caverns' luck counter and mandatory hand exile): the kernel's Leyline ask only moves the card, for every player"
	}
	return "opening-hand effect " + strings.TrimSpace(param) + ": the kernel's Leyline ask models only the plain FromHand self-to-battlefield move"
}

// conditionalManaSplit reports whether the mana ability being lowered splits
// its production across a Condition*-gated root and this Condition*-gated
// DB$ Mana sub (Gemstone Caverns, River of Tears).
func (lc *lowerCtx) conditionalManaSplit(ps *params) bool {
	root := lc.chainRoot
	if root == nil || root.API != "Mana" || strings.TrimSpace(root.Params["ConditionCheckSVar"]) == "" {
		return false
	}
	_, ok := ps.get("ConditionCheckSVar")
	return ok
}

const refuseConditionalMana = "a mana ability whose production is split across a Condition-gated root and a Condition-gated DB$ Mana sub (Gemstone Caverns, River of Tears): gorge defers the source (pay source:conditional), relaxes it to the root's production and asks the sub's colour in its prefix search; the kernel's only deferred source is Pyromancer's Goggles' fixed {R} (mana_deferred.go)"
