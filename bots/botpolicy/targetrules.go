package botpolicy

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// This file holds three small,
// pure target-selection rules re-expressed from Forge's ComputerUtilCard
// behaviour in gorge's own vocabulary. Nothing here is translated from Forge
// Java; each rule is stated as a sentence over Board facts and is measured by
// the forgegate harness before it ships.
//
// The rules are off by default. They are armed only through TargetRules,
// which an opt-in bot constructor supplies; every zero-value configuration
// leaves the ranking in target.go byte-identical.

// TargetRules selects the A2 target-selection rules. The zero value is
// "every rule off".
type TargetRules struct {
	// LandPriority promotes a land the opponent's deck depends on for colour.
	LandPriority bool
	// ProminentColour boosts removal aimed at a creature of the opponent's
	// dominant colour.
	ProminentColour bool
	// FilterFatalCopies deprioritises a target that is a strictly worse copy
	// of another creature the same opponent controls.
	FilterFatalCopies bool
}

// Enabled reports whether any A2 rule is armed.
func (r TargetRules) Enabled() bool {
	return r.LandPriority || r.ProminentColour || r.FilterFatalCopies
}

// The bonuses below live on the same integer scale as threat() (a 4/4 flier
// is about 40), and all of them are strictly smaller than the gap between
// target.go's tiers, so none of them can move a target across a tier: they
// order options WITHIN the value tier that already decides a kill is worth
// making. That is deliberate -- a targeting rule should break near-ties among
// plausible targets, never argue a removal onto a worse job.
const (
	// landDominantBonus is a land in the opponent's own dominant colour.
	landDominantBonus int32 = 20
	// landSplashBonus is a coloured land outside that colour.
	landSplashBonus int32 = 8
	// prominentColourBonus breaks near-ties toward the opponent's engine.
	prominentColourBonus int32 = 2
)

// publicColourMix returns, per colour slot (cards.ColourMask bit order), how
// many of player's PUBLIC cards carry that colour identity. It reads
// Board.PublicCards alone -- battlefield, graveyard, exile and stack, all
// facts the deciding seat may legally see -- never the player's hidden deck
// list. Colour identity, not "is this card coloured", is the measure because
// it is what CR 903.4 defines: a land that can only make black still counts
// toward black, which is exactly how a deck's colour posture is read from
// the cards it plays. A face-down card and a colourless card contribute
// nothing, so "unknown" is never read as a colour.
func (b Board) publicColourMix(player state.PlayerID) [5]int32 {
	var mix [5]int32
	for _, pc := range b.PublicCards.All() {
		if pc.Controller != player || pc.FaceDown || pc.Colour == 0 {
			continue
		}
		addColourMask(&mix, pc.Colour, 1)
	}
	return mix
}

// addColourMask adds n copies of identity mask m to mix.
func addColourMask(mix *[5]int32, m cards.ColourMask, n int32) {
	for i := 0; i < 5; i++ {
		if m&(1<<i) != 0 {
			mix[i] += n
		}
	}
}

// dominantColour is the slot holding the most of mix, ties broken by slot
// order. It returns -1 when the public cards name no colour: a colourless
// board has no prominent colour, and "unknown" must never be read as "white".
func dominantColour(mix [5]int32) int {
	best, bestN := -1, int32(0)
	for i := 0; i < 5; i++ {
		if mix[i] > bestN {
			best, bestN = i, mix[i]
		}
	}
	return best
}

// a2RemovalBonus is the A2 contribution to a removal target's value tier.
// It is zero when no A2 rule is armed, so the default ranking stays
// byte-identical.
func (b Board) a2RemovalBonus(o decision.Option) int32 {
	if !b.Targets.LandPriority && !b.Targets.ProminentColour {
		return 0
	}
	dominant := dominantColour(b.publicColourMix(o.Controller))
	bonus := int32(0)
	if b.Targets.LandPriority {
		bonus += b.landPriority(o.Obj, dominant)
	}
	if b.Targets.ProminentColour {
		if c, ok := b.Cards.Lookup(o.Obj); ok {
			bonus += prominentColourContribution(c, dominant)
		}
	}
	return bonus
}

// isLand is the Land-typed lookup both land rules share.
func (b Board) isLand(id state.ObjID) (Card, bool) {
	c, ok := b.Cards.Lookup(id)
	return c, ok && c.Land
}

// landPriority is what removing this land is worth on top of its printed
// value, re-expressing "which land hurts most" in facts the Board proves:
//
//   - a land carrying the opponent's dominant colour is the colour their
//     deck cannot function without (a mono-black deck's Swamp), so it ranks
//     above a bare non-creature;
//   - a coloured land outside that colour is a splash source, worth less;
//   - a colourless land earns nothing extra: it is never the coloured source
//     a deck is built around.
//
// The rule does not guess at utility-land text, because "what else this land
// does" is not a Board fact, and an unprovable claim is not a rule.
func (b Board) landPriority(id state.ObjID, dominant int) int32 {
	c, ok := b.isLand(id)
	if !ok || c.Colour == 0 {
		return 0
	}
	if dominant >= 0 && c.Colour&(1<<dominant) != 0 {
		return landDominantBonus
	}
	return landSplashBonus
}

// prominentColourContribution is the ProminentColour rule's share for one
// creature: a creature of the opponent's dominant colour is the deck's
// engine, so it outranks an off-colour creature of equal threat. It is zero
// for a colourless creature (not an engine) and for a deck with no dominant
// colour.
func prominentColourContribution(c Card, dominant int) int32 {
	if !c.Creature || dominant < 0 || c.Colour&(1<<dominant) == 0 {
		return 0
	}
	return prominentColourBonus
}

// worseCopy reports whether target `id` is a strictly worse copy of another
// creature the same opponent controls: same type-line signature, and the
// other is STRICTLY better on the facts the policy prices (power, remaining
// toughness, and a superset of keywords). Two identical copies are neither
// worse than the other, so neither is demoted. Removing the worse copy spends
// a removal the bigger copy will still need, which is the "don't remove a
// worse copy" rule.
//
// The type test uses Card.TypeSig, the same-set signature both adapters fill,
// so "copy" means exactly "same printed types", never a similar creature. A
// target whose type is unreadable (signature 0) never matches.
func (b Board) worseCopy(id state.ObjID, me state.PlayerID) bool {
	target, ok := b.Creatures.Lookup(id)
	if !ok || target.Controller == me {
		return false
	}
	tsig := b.typeSigOf(id)
	if tsig == 0 {
		return false
	}
	for oid, other := range b.Creatures.All() {
		if oid == id || other.Controller != target.Controller {
			continue
		}
		if b.typeSigOf(oid) != tsig {
			continue
		}
		if strictlyBetter(other, target) {
			return true
		}
	}
	return false
}

// typeSigOf is the Card census's type signature for id, 0 when unreadable.
func (b Board) typeSigOf(id state.ObjID) uint64 {
	c, ok := b.Cards.Lookup(id)
	if !ok {
		return 0
	}
	return c.TypeSig
}

// strictlyBetter reports whether a is better than t on the policy's facts:
// a is not worse on any of them, and t is worse on at least one. Equality is
// explicitly NOT strictly better, so identical copies never demote each other.
func strictlyBetter(a, t Creature) bool {
	return notWorseThan(a, t) && !notWorseThan(t, a)
}

// notWorseThan reports whether a is at least as good as t on every fact the
// policy prices: power, remaining toughness, and a superset of keywords.
func notWorseThan(a, t Creature) bool {
	if a.Power < t.Power || a.remTough() < t.remTough() {
		return false
	}
	for _, k := range t.Keywords {
		if !a.hasKeyword(k) {
			return false
		}
	}
	return true
}

// worseCopyPenalty demotes a strictly worse copy by more than every positive
// A2 bonus can add, so a better copy is always chosen first among the same
// opponent's creatures while the worse copy stays a legal (and preferred)
// answer when it is the only target offered.
const worseCopyPenalty int32 = landDominantBonus + landSplashBonus + prominentColourBonus + 1
