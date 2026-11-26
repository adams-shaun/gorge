package kgen

import (
	"reflect"
	"strings"
)

// A face-down keyword whose turn-up cost is a single pick part
// (Gathan Raiders and Putrid Raptor discard a card; Fathom Seer and Raven
// Guild Initiate return a creature or land you control; Gift of Doom
// sacrifices another creature, Skirk Volcanist two Mountains; CR 702.37e).
// gorge's keyword turn-up (rules morph_turnup.go) prices the cost's pick
// part with its mana through morphTurnUpPayable's NonManaCastable, so the
// action is offered only while the controller holds the candidates, and the
// taken action poses the pick ask (turnUpSacAsk / turnUpDiscardAsk /
// turnUpReturnAsk) before the payment settles. CardDef.MorphPick carries the
// part (MorphCost is {0}; dirac/face_morphpick.go); a pick beside mana, life
// or a reveal, an announced count and a self return stay refused.

// morphPickParse parses a Morph-family turn-up cost as one pick part. ok is
// false when the token is no pick part at all (the caller keeps its own
// "morph cost %q" refusal); ok with an error is a pick shape the turn-up
// window cannot pose.
func (lc *lowerCtx) morphPickParse(cs string) (pick lCost, ok bool, err error) {
	if cs == "" {
		return lCost{}, false, nil
	}
	c, cerr := lc.costHand(cs)
	if cerr != nil || !c.Mana.None {
		// no mana symbols in the token: the pick is the whole cost
		return lCost{}, false, nil
	}
	rest := c
	rest.Mana = manaCost{}
	switch {
	case c.Sac != nil:
		rest.Sac, rest.SacN = nil, 0
	case c.DiscardN > 0 && c.DiscardSpec == "":
		// a Random or Hand selection method is an unless cost's (gorge
		// d66db41d8), never a turn-up's
		rest.DiscardN, rest.DiscardF = 0, nil
	case c.Return != nil:
		rest.Return, rest.ReturnN = nil, 0
	default:
		return lCost{}, false, nil
	}
	if !reflect.DeepEqual(rest, lCost{}) || strings.Contains(cs, ":") {
		return lCost{}, true, refuse("morph pick cost %q: one fixed Discard/Sac/Return part alone", cs)
	}
	return c, true, nil
}
