package kgen

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// Prototype (CR 702.160, 718.3; gorge engine/state/prototype.go, rules
// legal_walk_hand.go and cast_begin.go "prototype"). A card printed with
// K:Prototype:<cost>:<P>:<T> lowers as a two-entry face card (kop.FacePrototype):
// the front is the card itself and face 1 is the same card with the
// prototype characteristics, its mana cost the inset cost, its colours that
// cost's, its power and toughness the inset pair, and every ability, type
// and keyword unchanged. The kernel casts face 1 from the hand as a face cast
// (dirac/face_castface.go; gorge's "prototype" cast option) and the object is
// face 1 on the stack and on the battlefield only (CR 718.3a), so every
// characteristic read sees the prototype without a flag in State.

// protoCard reports a pool card printed with K:Prototype on its only face.
// A token script, a multi-face card and a card with an alternate layout are
// not prototype cards (gorge reads the keyword off the one face).
func protoCard(c *cards.Card, token bool) bool {
	return !token && c != nil && len(c.Faces) == 1 && c.AlternateMode == "" && c.Faces[0] != nil &&
		c.Faces[0].HasKeyword("Prototype")
}

// prototypeRefusal is the reason a prototype card does not lower, "" when it
// does. The inset must be a plain fixed mana cost (gorge prices it through
// ParseCost, a colour per pip), the face a creature-shaped P/T with a fixed
// printed number (the inset replaces it, CR 718.3a), and nothing may
// replace the colours the inset cost gives (a colour indicator, Devoid).
func prototypeRefusal(c *cards.Card) string {
	f := c.Faces[0]
	raw, _, _, ok := state.PrototypeParams(f)
	if !ok {
		return "K:Prototype parameters"
	}
	mc, err := parseManaCost(raw)
	if err != nil || mc.None || mc.X || mc.XX || len(mc.Hybrid) != 0 || len(mc.Twobrid) != 0 || len(mc.Phyrexian) != 0 || mc.C != 0 {
		return "Prototype inset cost " + raw
	}
	if f.PT == "" || f.CharacteristicDefining() {
		return "Prototype on a card without a fixed printed P/T"
	}
	if f.HasKeyword("Devoid") || f.Colors != "" {
		return "Prototype beside a colour override"
	}
	return ""
}

// protoInset applies the prototype face's characteristics to the lowered
// record (lowerRecord, for face entry 1 of a FacePrototype card): the
// inset cost, its colours and the inset power and toughness.
func (lc *lowerCtx) protoInset() {
	f := lc.face
	raw, p, t, ok := state.PrototypeParams(f)
	if !ok {
		lc.refuseNode("K:Prototype", "Prototype parameters")
		return
	}
	mc, err := parseManaCost(raw)
	if err != nil {
		lc.refuseNode("K:Prototype", "%v", err)
		return
	}
	out := lc.out
	out.Cost, out.CostEmpty = mc, false
	out.Colors = colorsOf(&cards.Face{ManaCost: raw})
	out.NoPT, out.PTStar = false, false
	out.P, out.T = int(p), int(t)
}
