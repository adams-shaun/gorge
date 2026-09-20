package rules

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// K:Ascend (CR 702.131, task ascend1): "If you control ten or more
// permanents, you get the city's blessing for the rest of the game."
//
// The keyword is read DIRECTLY here rather than expanded in
// cards/keywords.go, exactly like kw:Start your engines (rules/speed.go's
// checkSpeedStart) and kw:MaxSpeed: Forge's own expansion (CardFactoryUtil)
// minted a Mode$ Always static trigger plus a static ability, machinery this
// engine has no representation for, and the equivalent read is the emit-side
// scan below plus the spell-resolution grant. Registering the primitive makes
// the coverage census count the 30 K:Ascend carriers as supported.
func init() {
	effects.RegisterNonAPI("kw:Ascend")
}

// checkBlessingGrants is the permanent half of the Ascend grant: called from
// Engine.emit's post-fold hook on every battlefield entry (MoveZone), token
// mint (TokenCreate, CardToken) and control transfer (ControlChange). For
// each still-alive unblessed seat it checks CR 702.131a's gate directly off
// the FOLDED state -- ten or more permanents on the battlefield under the
// seat's control, at least one of them carrying the Ascend keyword -- and
// emits the one-way BlessingChange latch. Running after the fold means a
// ten-permanent entry grants its own arrival, and the recursive emit the
// scan makes sees a now-blessed seat, so the scan terminates.
func (e *Engine) checkBlessingGrants() {
	if e.G.Over {
		return
	}
	for _, p := range e.G.AliveFrom(0) {
		if int(p) >= len(e.G.Players) || e.G.Players[p].Lost || e.G.Players[p].Blessing {
			continue
		}
		board := e.G.Zone(state.ZBattlefield, p)
		if len(board) < 10 {
			continue
		}
		ascend := false
		for _, id := range board {
			if e.HasKeyword(id, "Ascend") {
				ascend = true
				break
			}
		}
		if ascend {
			e.emit(events.Event{Kind: events.BlessingChange, Player: p,
				Text: "city's blessing"})
		}
	}
}

// grantSpellBlessing is the spell half of the Ascend grant (CR 702.131a's
// non-permanent case, matching Forge's resolvePreAbilities "do blessing there
// before condition checks"): an instant or sorcery with K:Ascend grants its
// controller the blessing AS THE SPELL RESOLVES, before the spell's own body
// and any of its condition checks read the latch. Called from
// rules/stack.go's resolveTop spell branch right after the Resolve event and
// right before effects.Resolve. Permanent faces are excluded -- their grant
// is the continuous scan above -- and the stack object itself is never on the
// battlefield, so it does not count toward the ten.
func (e *Engine) grantSpellBlessing(o *state.Object, f *cards.Face) {
	if f == nil || f.IsPermanent() || !f.HasKeyword("Ascend") {
		return
	}
	p := o.Controller
	if int(p) >= len(e.G.Players) || e.G.Players[p].Lost || e.G.Players[p].Blessing {
		return
	}
	if len(e.G.Zone(state.ZBattlefield, p)) < 10 {
		return
	}
	e.emit(events.Event{Kind: events.BlessingChange, Player: p,
		Text: "city's blessing"})
}
