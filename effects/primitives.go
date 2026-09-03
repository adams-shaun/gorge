package effects

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
)

// Native primitives needed for the stack (Task 14) to actually resolve
// something. Tasks 15-17 built the registry, Defined and Num plumbing but no
// concrete API implementations; Task 18 owns the full Forge primitive set and
// is expected to fold DealDamage's creature/planeswalker handling and Mana's
// cost-payment handling in here, replacing these via Register's documented
// replace-on-reregister semantics (registry.go). Kept intentionally minimal:
// exactly what castSpell/resolveAbility's own tests exercise -- damage
// against a targeted player, and a basic land's mana ability.
func init() {
	Register("DealDamage", dealDamage)
	Register("Mana", addMana)
}

// dealDamage implements "SP$/AB$ DealDamage" against players. A permanent
// target is marked with damage (the Damage event already knows how to route
// by Obj vs Player), but nothing yet checks it for lethality -- state-based
// actions are Task 21's job.
func dealDamage(h Host, c *Ctx, sa *cards.SA) {
	n := Num(h, c, sa, "NumDmg", 0)
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			h.Emit(events.Event{Kind: events.Damage, Player: t.Player, Amount: n})
		} else {
			h.Emit(events.Event{Kind: events.Damage, Obj: t.Obj, Amount: n})
		}
	}
}

// addMana implements "AB$ Mana": add Amount mana of Produced's colour to the
// activating player's pool. Produced is a single WUBRG/C letter, the same
// vocabulary state.ManaIndex reads.
func addMana(h Host, c *Ctx, sa *cards.SA) {
	amt := Num(h, c, sa, "Amount", 1)
	produced := sa.Params["Produced"]
	if produced == "" {
		produced = "C"
	}
	h.Emit(events.Event{Kind: events.ManaAdd, Player: c.Controller, Counter: produced, Amount: amt})
}
