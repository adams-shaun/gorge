package rules

import "github.com/adams-shaun/gorge/engine/state"

// sbaLethalDamageText is the casualty text of a CR 704.5g/h lethal-damage destruction.
const sbaLethalDamageText = "lethal damage"

// forgetHealedCasualties lets the next pass of this call re-read a creature whose
// lethal-damage destruction a replacement answered by removing its damage (umbra
// armor, CR 702.89a). CR 704.3 rechecks the board after every batch, and the
// destruction's substitute can leave the creature at toughness 0 or less (the Aura
// that gave +X/+X is the one destroyed), which the attempt memory would otherwise
// carry to the next call. The memory exists so a replacement that blocks an attempt
// forever does not spend the pass budget; a creature with nothing marked cannot be
// found lethal by damage again, so only the zero-toughness sweep can reach it, and
// that attempt is a "toughness <= 0" one, which is never forgotten.
func forgetHealedCasualties(g *state.Game, b *sbaBatch, tried *sbaAttempts) {
	for _, c := range b.dead {
		if c.text != sbaLethalDamageText || !tried.objs[c.id] {
			continue
		}
		o := g.Obj(c.id)
		if o == nil || o.Zone != state.ZBattlefield || o.Damage > 0 || o.Counter("Deathtouched") != 0 {
			continue
		}
		delete(tried.objs, c.id)
	}
}
