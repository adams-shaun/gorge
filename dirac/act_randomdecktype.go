package dirac

import (
	"os"
	"sort"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// dbgRandomDeck is a temporary diagnosis print.
var dbgRandomDeck = os.Getenv("GORGE_DEBUG_RANDOMDECK") != ""

// randomDeckType is OpChooseType with B = ChooseTypeRandomDeck: the AtRandom$
// Type$ CreatureInTargetedDeck pick (Aswan Jaguar). gorge effChooseType's
// AtRandom arm poses no ask: the option list is the sorted creature subtypes
// the resolution's player target's library prints (gorge
// CreatureInTargetedDeckLabels over c.Targets, deduped and sorted), one word
// is taken whole and two or more draw rng.IntN(len(list)) once (gorge
// labels[h.Rand(len(labels))]; the kernel's rng is gorge's), and the answer is
// the source's chosen type (gorge foldChoose's "type"), which a later
// Creature.ChosenType filter reads through srcChosen. An empty list keeps
// gorge's no-ask fallback, the first creature subtype word of an object the
// resolving controller controls, in object order, else "Human" (gorge
// creatureTypeFallback): the pool table carries "Human" (kgen's opChooseType
// refuses a pool that lacks it) and a word a pool card prints is always a pool
// subtype.
//
//go:noinline
func (s *State) randomDeckType(k *prep, in *kop.Instr) {
	// the pool subtypes a creature-type word names: TypeVocab maps every
	// creature-type word (gorge's vocabulary, in its order) to the matchup
	// subtype, 255 when no object of the matchup can have it
	var words [kop.MaxSubtypes]bool
	for _, si := range k.t.TypeVocab {
		if si != 255 {
			words[si] = true
		}
	}
	var set [kop.MaxSubtypes]bool
	if p := s.randomDeckPlayer(in); p != 255 {
		pl := &s.pl[p]
		for _, o := range pl.lib[pl.libLo:pl.libHi] {
			c := s.objs[o].card
			cd := &k.t.Cards[c]
			for i := 0; i < len(k.t.Subtypes); i++ {
				if cd.Subtypes.Has(i) && words[i] {
					set[i] = true
				}
			}
		}
	}
	// the chosen words in gorge's sorted order (gorge sort.Strings over the
	// same subtype words)
	var list []uint8
	for i, on := range set {
		if on {
			list = append(list, uint8(i))
		}
	}
	sort.Slice(list, func(a, b int) bool { return k.t.Subtypes[list[a]] < k.t.Subtypes[list[b]] })
	code := uint8(255)
	switch {
	case len(list) == 0:
		code = s.creatureFallbackSub(k, &words, s.res.e.ctrl)
	case len(list) == 1:
		code = list[0] // no draw: gorge takes the one word whole
	default:
		code = list[s.rng.IntN(len(list))]
	}
	if dbgRandomDeck {
		print("randomdeck: ctrl=", s.res.e.ctrl, " list=[")
		for i, si := range list {
			if i > 0 {
				print(",")
			}
			print(k.t.Subtypes[si])
		}
		print("] picked=", k.t.Subtypes[code], "\n")
	}
	r := &s.res
	r.chosen = objWord(code)
	r.bad |= resChosenType
	if o, sl := s.resolutionTypeSource(); sl != NoSlot {
		s.perms[sl].chosen = r.chosen
	} else if o != NoObj {
		s.objs[o].chosen = uint8(r.chosen)
		s.touch(o)
	}
}

// randomDeckPlayer is the seat the instruction's C names: the target slot's
// player (gorge CreatureInTargetedDeckLabels' player target over c.Targets).
// 255 when the slot did not resolve to a player, so the sample takes the
// fallback.
func (s *State) randomDeckPlayer(in *kop.Instr) uint8 {
	sl, ok := tgtSlot(kop.Ref(in.C))
	if !ok {
		return 255
	}
	t := s.tgtCode(sl)
	if !isPlayerTarget(t) {
		return 255
	}
	return CodeSeat(t)
}

// creatureFallbackSub is gorge creatureTypeFallback reduced to a pool subtype:
// the first creature subtype word of an object the controller controls, in
// object order (gorge scans g.Objs, the printed type line and the current
// face's printed types), else "Human" (the pool table's index; kgen refuses a
// pool that lacks it). A face-down permanent's printed face has no types
// (gorge Face's face-down face).
func (s *State) creatureFallbackSub(k *prep, words *[kop.MaxSubtypes]bool, ctrl uint8) uint8 {
	for i := Obj(0); i < s.nobj; i++ {
		o := &s.objs[i]
		if o.zone == kop.ZoneBattlefield {
			if s.faceDown(k, o.slot) || s.perms[o.slot].ctrl != ctrl {
				continue
			}
		} else if o.owner() != ctrl {
			continue
		}
		c := o.card
		if k.cards[c].types&kop.TCreature == 0 {
			continue
		}
		cd := &k.t.Cards[c]
		for j := 0; j < len(k.t.Subtypes); j++ {
			if cd.Subtypes.Has(j) && words[j] {
				return uint8(j)
			}
		}
	}
	// "Human": kgen's opChooseType refused a pool table without it, so this is
	// an unreachable shape (StallLowering)
	for j := 0; j < len(k.t.Subtypes); j++ {
		if k.t.Subtypes[j] == "Human" {
			return uint8(j)
		}
	}
	s.capFull(StallLowering)
	return 255
}
