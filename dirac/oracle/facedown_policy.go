package oracle

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// FaceDownPolicy is the evidence sweep's face-down steering policy. It prefers,
// in order, a turn-face-up special action, a face-down cast and an ETB mana
// source; everything else is base's answer. The face-down cast and the
// turn-face-up are priced from the floating pool alone (face_facedown.go's
// offerFaceDown/offerTurnUpSl), so a game only reaches them after an ETB
// mana source has added to the pool. The overlay does not play lands or
// otherwise change the base policy's decisions: when none of the three is
// offered, the game is exactly base's.
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
				if floater == 0 && isETBManaOption(c, a) {
					floater = a
				}
			}
		}
		switch {
		case up != 0:
			return up
		case down != 0:
			return down
		case floater != 0:
			return floater
		}
		return base.Pick(c)
	})
}

// faceDownScore ranks a face-down cast for the sweep: a card whose turn-up
// trigger the evidence wants first, then the cheapest printed turn-up cost
// (fewest cost fields), so a cheap carrier is cast face down before an
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
			score -= len(strings.Fields(cost))
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

// isETBManaOption reports whether a casts a card whose enters-the-battlefield
// trigger adds mana (Priest of Gix, Iridescent Tiger). It reads the gorge face
// the option names; a kernel-led run has no GorgeSide, so it answers false and
// FaceDownPolicy falls back to the face/turn-up preference alone.
func isETBManaOption(c *PickCtx, a Canon) bool {
	if c.G == nil {
		return false
	}
	return isETBManaFace(optionFace(c, a))
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

// isETBManaFace reports whether f has an enters-the-battlefield trigger whose
// effect chain reaches a mana effect.
func isETBManaFace(f *cards.Face) bool {
	if f == nil {
		return false
	}
	for _, t := range f.Triggers {
		if t.Mode != "ChangesZone" {
			continue
		}
		for sa := t.Effect; sa != nil; sa = sa.Sub {
			if sa.API == "Mana" {
				return true
			}
		}
	}
	return false
}

// FaceDownSweepEligible reports whether set holds both halves a face-down
// sweep needs: a lowered Morph-family card and an enters-the-battlefield mana
// source. A pool with no floater can never offer a face-down cast (the offer
// is priced from the floating pool alone), so the sweep leaves its policy
// unchanged rather than steering a game the policy cannot reach.
func FaceDownSweepEligible(set cardset.Set) bool {
	if set == nil {
		return false
	}
	carrier, floater := false, false
	supported := set.Supported()
	for i := 0; i < set.NumCards() && !(carrier && floater); i++ {
		if !supported[i] {
			continue
		}
		d := set.Def(i)
		if d.MorphFamily != 0 {
			carrier = true
		}
		if cardDefHasETBMana(d) {
			floater = true
		}
	}
	return carrier && floater
}

// cardDefHasETBMana reports whether d's kernel abilities carry an
// enters-the-battlefield trigger with an AddMana op.
func cardDefHasETBMana(d *kop.CardDef) bool {
	for i := range d.Abilities {
		a := &d.Abilities[i]
		if a.Kind != kop.AbTriggered || a.Trig.Event != kop.TrigETB {
			continue
		}
		for _, in := range a.Prog {
			if in.Op == kop.OpAddMana {
				return true
			}
		}
	}
	return false
}
