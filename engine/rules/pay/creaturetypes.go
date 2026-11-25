package pay

import (
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules/chars"
	costvocab "github.com/adams-shaun/gorge/engine/rules/cost"
	"github.com/adams-shaun/gorge/engine/state"
)

// This file is the payment layer's read of the creature-type relationship a
// tapXType<2/Creature.sharesCreatureTypeWith> cost places on the pair it
// taps (Weight of Conscience, CostPart.SameCreatureType). The offer gate
// (NonManaCastableP) and the planner ask it, while the tap election publishes
// the SAME property through decision.SetPropMode/SetProps, so a pair the
// offer reserved and the pair Validate/Clamp accept cannot disagree.

// creatureTypeTokens returns id's current creature subtype tokens, lower-case
// and filtered to the CR creature-type vocabulary (chars.CreatureTypeTokens).
// e.Chars().DerivedTypes already materialises a layer-4 AddAllCreatureTypes$
// grant; the intrinsic Changeling/AddAllCreatureTypes$ CDA marker is expanded
// here so it reads every type too (CR 702.73a).
func creatureTypeTokens(e Engine, id state.ObjID) []string {
	all := false
	if o := e.Game().Obj(id); o != nil && o.Face() != nil {
		all = effects.IntrinsicAllCreatureTypes(o)
	}
	return chars.CreatureTypeTokens(effects.TypeMatchWords(e.Chars().DerivedTypes(id), all))
}

// CreatureTypeSets returns one creature-type token set per id, in input order.
func CreatureTypeSets(e Engine, ids []state.ObjID) [][]string {
	sets := make([][]string, len(ids))
	for i, id := range ids {
		sets[i] = creatureTypeTokens(e, id)
	}
	return sets
}

// SameCreatureTypeCapacity is the largest number of candidates that can be
// tapped together under the shared-creature-type constraint: the size of the
// largest creature-type class (decision.SetPropCapacity's SetPropShared
// reading, the same rule the election's Validate and the bot's Clamp use).
func SameCreatureTypeCapacity(sets [][]string) int {
	return decision.SetPropCapacity(decision.SetPropShared, sets)
}

// SameCreatureTypePair returns the first pair of ids whose token sets share a
// creature type, in input order, so an offer gate can reserve exactly the
// objects a legal payment will tap. ok is false when no pair shares a type.
func SameCreatureTypePair(ids []state.ObjID, sets [][]string) (a, b state.ObjID, ok bool) {
	if len(ids) != len(sets) {
		return 0, 0, false
	}
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			if SameCreatureTypeCapacity([][]string{sets[i], sets[j]}) >= 2 {
				return ids[i], ids[j], true
			}
		}
	}
	return 0, 0, false
}

// SameCreatureTypeShares reports whether two objects share at least one
// creature type with each other (the tap-pair relation).
func SameCreatureTypeShares(e Engine, a, b state.ObjID) bool {
	sets := [][]string{creatureTypeTokens(e, a), creatureTypeTokens(e, b)}
	return SameCreatureTypeCapacity(sets) >= 2
}

// reserveTapPart validates one literal tapXType part's candidates and claims
// the objects its payment will tap in reserved (one permanent cannot pay two
// parts). A CostPart.SameCreatureType part (Weight of Conscience) claims a
// pair that shares a creature type with each other; an ordinary part claims
// the first N candidates. It returns false when the board cannot fill the
// part, the fail-closed direction the offer gate needs.
func reserveTapPart(e Engine, part costvocab.CostPart, avail []state.ObjID, reserved map[state.ObjID]bool) bool {
	if part.SameCreatureType {
		if part.Dyn != "" || part.N != 2 {
			return false
		}
		sets := CreatureTypeSets(e, avail)
		if SameCreatureTypeCapacity(sets) < 2 {
			return false
		}
		a, b, ok := SameCreatureTypePair(avail, sets)
		if !ok {
			return false
		}
		reserved[a], reserved[b] = true, true
		return true
	}
	if len(avail) < int(part.N) {
		return false
	}
	for i := 0; i < int(part.N); i++ {
		reserved[avail[i]] = true
	}
	return true
}
