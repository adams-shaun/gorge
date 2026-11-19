package effects

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// effectRegisterRestriction registers an Effect-delivered restriction. Forge
// creates the Effect card under EffectOwner$ (the Effect card's controller),
// so a CantAttack body's `Creature.YouCtrl` and its "your next turn" lifetime
// belong to that player, not to the resolving ability's controller (Willie
// Lumpkin, Postman: the damaged player's creatures cannot attack Willie's
// controller, through the end of the damaged player's next turn). With no
// EffectOwner$ the registration is unchanged. Other restriction modes keep the
// resolving controller (their EffectOwner$ reading is a separate ticket), and
// an EffectOwner$ this build cannot resolve registers nothing, with a Note.
func effectRegisterRestriction(h Host, c *Ctx, ownerSel, mode string, ce state.ContinuousEffect) {
	if ownerSel == "" || cards.StaticModeOf(mode) != cards.StaticCantAttack {
		effectContinuous(h, ce)
		return
	}
	owners, ok := EffectOwnerPlayers(h, c, ownerSel)
	if !ok || len(owners) == 0 {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
			Text: "unresolvable EffectOwner$ " + ownerSel + " (restriction not registered)"})
		return
	}
	for _, owner := range owners {
		ce.Controller = owner
		effectContinuous(h, ce)
	}
}
