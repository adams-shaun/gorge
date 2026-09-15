package effects

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() { Register("Myriad", effMyriad) }

// effMyriad implements CR 702.109's Myriad: when a creature with Myriad
// attacks, for each opponent OTHER than the defending player, the controller
// creates a token that is a copy of the attacking creature, tapped and
// attacking that opponent. The trigger (cards/keywords.go) fires on the
// source's own DeclareAttackers event, so the combat is already in progress;
// this effect emits one MyriadCopy event per non-defending opponent (the
// DefendingPlayer comes from the trigger's active defender, carried in the
// effect context). The tokens are created tapped and attacking, which is the
// Myriad copy's entry condition.
//
// events.MyriadCleanup exiles these marked tokens as end combat ends. The
// copy is the source's current card/face, preserving its printed
// characteristics for the token's battlefield lifetime.
func effMyriad(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	src := g.Obj(c.Source)
	if src == nil || src.Face() == nil {
		return
	}
	// The defending player is the player the source is attacking, captured by
	// triggerRemembered (a DeclareAttackers trigger appends the defending
	// player as the last Remembered entry), so it survives the resolveTop
	// rebuild of the Ctx. The remaining alive players are the other opponents
	// a Myriad token attacks.
	defender := state.PlayerID(c.Controller)
	for i := len(c.Remembered) - 1; i >= 0; i-- {
		if c.Remembered[i].IsPlayer {
			defender = c.Remembered[i].Player
			break
		}
	}
	for _, q := range g.AliveFrom(c.Controller) {
		if q == c.Controller || q == defender {
			continue
		}
		// Player is the token's controller (the attacking player) and IDs[0]
		// is the opponent it attacks.
		h.Emit(events.Event{Kind: events.MyriadCopy, Obj: c.Source, Player: c.Controller, IDs: []state.ObjID{state.ObjID(q)}})
	}
}
