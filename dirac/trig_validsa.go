package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// two SpellCast ValidSA$ heads (gorge trigmatch/cast.go
// ValidSAMatches, :731).
//
// Tf3ManaFromTreasure (Spell.ManaFromTreasure, Smaug): the cast's payment
// spent mana a Treasure produced. The pool tallies the floating units a
// Treasure produced (artTag, poolTre) and every pool spend sets the cast's
// build bit (treSync, bflTre) for the Treasure units it took; castTrigGen
// runs while that build is live.
//
// Tf3OrTargets ("Spell.X,Spell.IsTargeting Valid Y", Danitha): the spell
// matches Filter OR some target of it matches Filter2 (gorge
// spellIsTargetingMatchesPtr tries every target; a player target never
// matches a creature spec).

// castTreasureOK is the Tf3ManaFromTreasure gate: the cast still being built
// spent Treasure mana (a stack copy or a later read never matches).
//
//go:noinline
func (s *State) castTreasureOK(e *stackEntry) bool {
	if debugValidSAMutate == 1 {
		return true // test only: every cast spent Treasure mana
	}
	if debugValidSAMutate == 2 {
		return false // test only: no cast spent Treasure mana
	}
	return s.bld().e.kind == seSpell && s.bld().e.obj == e.obj && s.bld().bfl&bflTre != 0
}

// castTargetsAny is the Tf3OrTargets target arm: some object target of spell
// e matches f, read with the trigger's source and controller. A battlefield
// target is matched as a permanent; a target in any other zone as a card
// controlled by its owner (gorge's Controller off the battlefield).
//
//go:noinline
func (s *State) castTargetsAny(k *prep, f kop.FilterID, e *stackEntry, l trigListener) bool {
	if debugValidSAMutate == 3 {
		return false // test only: the target arm never matches
	}
	var codes [kop.MaxTargets + maxXTgt]uint8 // a multi-object slot's extras too
	n := s.chainTargets(e, &codes)
	for i := 0; i < n; i++ {
		t := codes[i]
		if isPlayerOrNone(TCode(t)) {
			continue
		}
		ob := &s.objs[t]
		if ob.zone == kop.ZoneBattlefield {
			if s.matchPermGen(k, f, ob.slot, l.ctrl, l.sl, layerFull) {
				return true
			}
		} else if s.matchCardTgt(k, f, ob.card, s.ctrlOfObj(Obj(t)), l.ctrl, Obj(t), l.sl, l.obj) {
			return true
		}
	}
	return false
}

// markTreasureSrc marks every Treasure-subtyped card csTreasure (the loop of
// gen.go's castRingOn block, run here for a Tf3ManaFromTreasure table).
func markTreasureSrc(t *kop.Table, p *prep) {
	for i, n := range t.Subtypes {
		if n == "Treasure" {
			for c := range t.Cards {
				if t.Cards[c].Subtypes.Has(i) {
					p.cards[c].castSrc |= csTreasure
				}
			}
			return
		}
	}
}

// debugValidSAMutate is the lockstep mutation switch (test only): 1 the
// Treasure gate always holds, 2 never, 3 the target arm never matches.
var debugValidSAMutate uint8

// SetDebugValidSAMutate sets the mutation switch; tests only.
func SetDebugValidSAMutate(m uint8) { debugValidSAMutate = m }
