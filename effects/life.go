package effects

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
)

func init() {
	Register("GainLife", effGainLife)
	Register("LoseLife", effLoseLife)
}

// effGainLife and effLoseLife both clamp a negative LifeAmount$ to zero,
// mirroring Ruling T14-f's DealDamage/Mana clamps: LifeChange's Apply case is
// a plain "+= Amount", so an unclamped negative would silently flip the
// direction of the effect (a life-gain spell that drains, or vice versa)
// instead of doing nothing.
func effGainLife(h Host, c *Ctx, sa *cards.SA) {
	n := Num(h, c, sa, "LifeAmount", 1)
	if n < 0 {
		n = 0
	}
	for _, t := range Defined(h, c, sa) {
		h.Emit(events.Event{Kind: events.LifeChange, Player: PlayerOf(h, c, t), Amount: n})
	}
}

func effLoseLife(h Host, c *Ctx, sa *cards.SA) {
	n := Num(h, c, sa, "LifeAmount", 1)
	if n < 0 {
		n = 0
	}
	for _, t := range Defined(h, c, sa) {
		h.Emit(events.Event{Kind: events.LifeChange, Player: PlayerOf(h, c, t), Amount: -n})
	}
}
