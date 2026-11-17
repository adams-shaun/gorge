package oracle

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// FaceDownPolicy is the evidence sweep's face-down steering policy. It prefers,
// in order, a turn-face-up special action, a cast of a triggered mana source
// and a face-down cast; everything else is base's answer. The face-down cast
// and the turn-face-up are priced from the floating pool alone
// (face_facedown.go's offerFaceDown/offerTurnUpSl), so a game only reaches them
// after a mana trigger has added to the pool, and the mana source is taken
// before the face-down cast so its floats bank past the cast: a face-down
// carrier then stands beside the next float event, which is what offers the
// turn-up. The overlay does not play lands or otherwise change the base
// policy's decisions: when none of the three is offered, the game is exactly
// base's.
func FaceDownPolicy(base Policy) Policy {
	return PolicyFunc(func(c *PickCtx) Canon {
		var down, up, floater Canon
		downScore, upScore := 0, 0
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCastFace:
				if !isFaceDownCast(a) {
					continue
				}
				if s := faceDownScore(c, a); down == 0 || s > downScore {
					down, downScore = a, s
				}
			case ActSpecial:
				if !isTurnFaceUp(a) {
					continue
				}
				if s := faceDownScore(c, a); up == 0 || s > upScore {
					up, upScore = a, s
				}
			case ActCast:
				if floater == 0 && isTriggerManaOption(c, a) && sweepWantsFloats(c) {
					floater = a
				}
			}
		}
		switch {
		case up != 0:
			return up
		case floater != 0:
			return floater
		case down != 0:
			return down
		}
		return base.Pick(c)
	})
}

// sweepWantsFloats reports whether the decision's player has a face-down
// future the floats can fund: a Morph-family card in hand (the face-down cast
// it will ask floats for) or a face-down permanent already on the battlefield
// (the turn-up it will ask floats for). When neither holds, banking floats
// only funds the base policy's next cast -- the pool empties at the end of the
// step whatever the policy does -- so the overlay leaves the mana source to
// the base policy's own answer. A kernel-led run has no state to read, so it
// keeps the unconditional preference.
func sweepWantsFloats(c *PickCtx) bool {
	if c.G == nil {
		return true
	}
	p := state.PlayerID(c.D.Player)
	for _, id := range c.G.E.G.Zone(state.ZHand, p) {
		ob := c.G.E.G.Obj(id)
		if ob == nil || ob.Card == nil || len(ob.Card.Faces) == 0 {
			continue
		}
		f := ob.Card.Faces[0]
		for _, k := range []string{"Morph", "Megamorph", "Disguise"} {
			if _, ok := f.KeywordParam(k); ok {
				return true
			}
		}
	}
	for _, id := range c.G.E.G.Zone(state.ZBattlefield, p) {
		if ob := c.G.E.G.Obj(id); ob != nil && ob.FaceDown {
			return true
		}
	}
	return false
}

// faceDownScore ranks a face-down cast for the sweep: a card whose turn-up
// trigger the evidence wants first, then the cheapest printed turn-up cost
// (fewest mana symbols), so a cheap carrier is cast face down before an
// expensive one when both are offered.
func faceDownScore(c *PickCtx, a Canon) int {
	f := optionFace(c, a)
	if f == nil {
		return 0
	}
	score := 0
	for _, t := range f.Triggers {
		if t.Mode == "TurnFaceUp" {
			score += 8
		}
	}
	for _, k := range []string{"Morph", "Megamorph", "Disguise"} {
		if cost, ok := f.KeywordCostParam(k); ok {
			score -= strings.Count(cost, "{")
			break
		}
	}
	return score
}

// faceDownCodes are the face routes a face-down cast uses (canon.FaceCodeMorphed
// through canon.FaceCodeDisguised); every other ActCastFace is an ordinary face
// cast (a modal DFC, an adventure, a split half).
func isFaceDownCast(a Canon) bool {
	switch a.Mode() & 0xf {
	case faceCodeMorphed, faceCodeMegamorphed, faceCodeDisguised:
		return true
	}
	return false
}

// turnFaceUpSpecial is the special action kind canon.SpecialCode gives
// "turn_face_up"; the low nibble of an ActSpecial mode.
var turnFaceUpSpecial = func() uint8 { c, _ := specialCode("turn_face_up"); return c }()

// isTurnFaceUp reports whether a is the turn-face-up special action.
func isTurnFaceUp(a Canon) bool { return a.Mode()&0xf == turnFaceUpSpecial }

// isTriggerManaOption reports whether a casts a card whose triggered ability
// adds mana (Priest of Gix's enters trigger, Loyal Fire Sage's attacks
// trigger, Crime Novelist's sacrifice trigger). It reads the gorge face the
// option names; a kernel-led run has no GorgeSide, so it answers false and
// FaceDownPolicy falls back to the face/turn-up preference alone.
func isTriggerManaOption(c *PickCtx, a Canon) bool {
	if c.G == nil {
		return false
	}
	return isTriggerManaFace(optionFace(c, a))
}

// optionFace is the gorge face a canonical priority option names, or nil when
// the option names no object (or the run is kernel-led, with no GorgeSide).
func optionFace(c *PickCtx, a Canon) *cards.Face {
	if c.G == nil {
		return nil
	}
	o, ok := c.G.GorgeOption(a)
	if !ok {
		return nil
	}
	ob := c.G.E.G.Obj(o.Obj)
	if ob == nil {
		return nil
	}
	return ob.Face()
}

// maxManaWalkDepth bounds the Charm-body recursion of saReachesMana.
const maxManaWalkDepth = 8

// isTriggerManaFace reports whether f has a triggered ability whose effect
// chain reaches a mana effect. Every trigger mode counts and the walk follows
// the SubAbility$ chain and a Charm's Choices$ bodies (Galadriel, Light of
// Valinor's any-colour arm): the floating pool does not care which event added
// the mana, and the kernel's face-down cast and turn-up offers price both from
// the pool alone.
func isTriggerManaFace(f *cards.Face) bool {
	if f == nil {
		return false
	}
	for i := range f.Triggers {
		if saReachesMana(f.Triggers[i].Effect, f, 0) {
			return true
		}
	}
	return false
}

// saReachesMana reports whether sa or a body it reaches is a Mana effect.
func saReachesMana(sa *cards.SA, f *cards.Face, depth int) bool {
	if sa == nil || depth > maxManaWalkDepth {
		return false
	}
	if sa.API == "Mana" {
		return true
	}
	if sa.API == "Charm" {
		for _, n := range cards.SplitModeNames(sa.ParamStr(cards.PKChoices)) {
			if body := cards.ResolveSVar(f.SVars, n); saReachesMana(body, f, depth+1) {
				return true
			}
		}
	}
	return saReachesMana(sa.Sub, f, depth)
}

// FaceDownSweepEligible reports whether set holds both halves a face-down
// sweep needs: a lowered Morph-family card -- the kernel def fact the
// face-down cast and turn-face-up offers key on -- and a card whose triggered
// ability adds mana. A pool with no floater can never offer a face-down cast
// (the offer is priced from the floating pool alone), so the sweep leaves its
// policy unchanged rather than steering a game the policy cannot reach. The
// mana half reads the compiled faces the runtime policy reads
// (isTriggerManaFace), so the pool selection and the steering policy cannot
// disagree about what a floater is.
func FaceDownSweepEligible(reg *cards.Registry, set cardset.Set) bool {
	if set == nil || reg == nil {
		return false
	}
	carrier, floater := false, false
	supported := set.Supported()
	names := set.Names()
	for i := 0; i < set.NumCards() && !(carrier && floater); i++ {
		if !supported[i] {
			continue
		}
		if set.Def(i).MorphFamily != 0 {
			carrier = true
		}
		if c, ok := reg.Lookup(names[i]); ok {
			for _, f := range c.Faces {
				if isTriggerManaFace(f) {
					floater = true
					break
				}
			}
		}
	}
	return carrier && floater
}
