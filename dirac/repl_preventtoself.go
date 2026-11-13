package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// ReplPreventDamageToSelf: a permanent's printed bodyless prevention of
// damage dealt to itself (gorge R:Event$ DamageDone | Prevent$ True |
// ValidTarget$ Card.Self or Creature.Self): Istvan, Butcher of Eln (from
// creatures on the battlefield), Guardian Naga (during your turn). gorge
// (rules/replacement_damage.go) matches it per damage event, so combat
// damage is prevented source by source (replCombatGen's per-event
// preventCombatEv); a prevented event is no damage, no lifelink, no damage
// trigger. A can't-prevent static keeps it off (CR 615.12). Beside another
// damage replacement that can match the same event gorge asks the CR 616.1
// order: it is a self-damage kind (rbSelfDmg), so initStall refuses that
// pair (StallReplPlus).

// PrevToSelfStats counts the events it prevented (the oracle test's proof
// the pool reaches it).
var PrevToSelfStats struct{ Hits atomic.Int64 }

// prevToSelf reports whether the permanent at sl's own ReplPreventDamageToSelf
// prevents damage from the source (srcSl on the battlefield, else the card
// srcCard) controlled by srcCtrl.
func (s *State) prevToSelf(k *prep, sl Slot, srcCtrl uint8, srcSl Slot, srcCard CardID, combat bool) bool {
	if DebugSelfDamageReplOff || k.gen.replBits[s.cardAt(sl)]&rbPrevToSelf == 0 || s.genLost(k, sl) {
		return false
	}
	lc := s.perms[sl].ctrl
	cd := &k.t.Cards[s.cardAt(sl)]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplPreventDamageToSelf {
			continue
		}
		c := a.Repl.C
		if c&kop.ReplPrevToCombat != 0 && !combat || c&kop.ReplPrevToYourTurn != 0 && s.active != lc ||
			c&kop.ReplPrevToCreature != 0 && !s.isCreature(k, sl) || c&kop.ReplPrevToSrcInPlay != 0 && srcSl == NoSlot {
			continue
		}
		if f := a.Repl.Filter; f != 0 {
			if srcSl != NoSlot {
				if !s.matchPermGen(k, f, srcSl, lc, sl, layerFull) {
					continue
				}
			} else if !s.matchCardGen(k, f, srcCard, srcCtrl, lc, NoObj) {
				continue
			}
		}
		if s.noPrev(k, combat) {
			return false
		}
		PrevToSelfStats.Hits.Add(1)
		return true
	}
	return false
}
