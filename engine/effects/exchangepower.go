package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

func init() { Register("ExchangePower", effExchangePower) }

// effExchangePower implements DB$ ExchangePower (Forge's
// ExchangePowerEffect; Serene Master's "Whenever CARDNAME blocks, exchange
// its power and the power of target creature it's blocking until end of
// combat"): the source (Defined$ Self) and the chosen creature target
// exchange power. The two powers are read NOW, over the board as the ability
// resolves (CR 608.2h), and each side registers ONE layer-7b set
// (state.SubSet, the StaticSet shape the life-exchange transaction and the
// SetPower$ statics share) fixing that side's power to the other's: a value
// exchange pinned at the moment the ability resolves, which is what the
// printed text exchanges. Duration$ carries the grant's lifetime; the corpus
// value UntilEndOfCombat (CR 511.2) rides the same combat expiry
// rules.expireUntilEndOfCombat gives every combat-scoped grant, and any
// other value keeps durationTiming's ordinary mapping (absent: until end of
// turn). Neither side needs ValidTgts$ read here: the target ask is the
// generic machinery's.
func effExchangePower(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	a, b := c.Source, state.ObjID(0)
	for _, t := range c.Targets {
		if !t.IsPlayer && t.Obj != 0 {
			b = t.Obj
			break
		}
	}
	if b == 0 {
		return
	}
	oa, ob := g.Obj(a), g.Obj(b)
	if oa == nil || ob == nil || oa.Zone != state.ZBattlefield || ob.Zone != state.ZBattlefield {
		return
	}
	pa, pb := h.Power(a), h.Power(b)
	if pb == pa {
		// An exchange of equal powers changes nothing; registering two
		// no-op sets would only pollute Engine.continuous (the
		// registerPumpEffects zero-grant convention).
		return
	}
	dur := strings.TrimSpace(sa.ParamStr(cards.PKDuration))
	_, untilEOT := durationTiming(dur)
	h.AddContinuous(state.ContinuousEffect{
		Source: a, Affects: "Card.Self", Controller: c.Controller,
		Layer: state.LPT, Sub: state.SubSet, HasSet: true,
		SetPower: pb, SetPowerPresent: true, StaticSet: true,
		Duration: dur, UntilEOT: untilEOT,
	})
	h.AddContinuous(state.ContinuousEffect{
		Source: b, Affects: "Card.Self", Controller: c.Controller,
		Layer: state.LPT, Sub: state.SubSet, HasSet: true,
		SetPower: pa, SetPowerPresent: true, StaticSet: true,
		Duration: dur, UntilEOT: untilEOT,
	})
}
