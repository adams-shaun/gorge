package effects

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

func init() {
	Register("TapOrUntapAll", effTapOrUntapAll)
}

// effTapOrUntapAll is Forge's TapOrUntapAllEffect (Turnabout): the
// resolving ability's controller makes ONE election, tap or untap, then
// every matching battlefield permanent is put in the elected state: "tap
// all untapped ..., or untap all tapped ...". Player-kind ValidTgts$ scopes
// the sweep to the selected player's permanents. Forge asks the election
// unconditionally (chooseBinary before the card walk), so it is asked even
// over an empty sweep. The tap leg is effTapAll's EmitTap by the
// controller; the untap leg is effUntapAll's tryUntapBy (a stun counter is
// removed instead, CR 122.1d). One resolution is one tapping or untapping
// action, so the TapAll / UntapAll batch fires once.
func effTapOrUntapAll(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	spec := sa.ParamStr(cards.PKValidCards)
	if spec == "" {
		spec = "Permanent"
	}
	scope := targetPlayerKindScope(h, c, sa)
	tap := askTapOrUntapAll(h, c, sa)
	defer beginActionBatch(h)()
	for _, p := range g.AliveFrom(0) {
		ids := append([]state.ObjID(nil), g.Zone(state.ZBattlefield, p)...)
		for _, id := range ids {
			if !inSweepScope(g, id, scope) || !MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
				continue
			}
			o := g.Obj(id)
			if o == nil || o.Zone != state.ZBattlefield {
				continue
			}
			if !tap {
				tryUntapBy(h, id, c.Controller)
			} else if !o.Tapped {
				h.EmitTap(id, c.Controller, false)
			}
		}
	}
}

// askTapOrUntapAll poses the one tap-or-untap election (ResumeKind
// "taporuntapall") to the resolving ability's controller and reports
// whether "tap" was elected. The options are in Forge's binary order, "tap"
// then "untap", and name no object. Deterministic contract: a host that
// cannot ask (and a bot that clamps) takes option 0, "tap"; an empty answer
// reads as "tap" as well. ValidMessage$ (Forge's prompt text) is not read:
// the prompt is generic.
func askTapOrUntapAll(h Host, c *Ctx, sa *cards.SA) bool {
	seat := c.Controller
	d := &decision.Decision{Player: seat, Kind: decision.KChoose, Min: 1, Max: 1,
		ResumeKind: "taporuntapall", ResumeSA: sa, Source: c.Source,
		Prompt: "Tap or untap all of them?"}
	d.Options = []decision.Option{
		{Index: 0, Kind: "tap", Label: "Tap all", Player: seat},
		{Index: 1, Kind: "untap", Label: "Untap all", Player: seat},
	}
	if ans, ok := AskTape(h, d); ok && len(ans) > 0 {
		return ans[0].Kind != "untap"
	}
	return true
}
