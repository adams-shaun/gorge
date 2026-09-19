package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() { Register("SacrificeAll", effSacrificeAll) }

// effSacrificeAll implements the mass-sacrifice primitive (CR 701.16). With a
// Defined$ object target it sacrifices those battlefield objects; otherwise
// every player sacrifices every permanent matching ValidCards$ (default
// Permanent). RememberSacrificed$ preserves LKI for a following sub-ability.
//
// UnlessCost$ is a real may-pay continuation: the named payer can pay to
// prevent the whole sacrifice. The common engine continuation owns mana
// payment, so this effect only poses the ask and interprets its answer.
func effSacrificeAll(h Host, c *Ctx, sa *cards.SA) {
	if cost := strings.TrimSpace(sa.Params["UnlessCost"]); cost != "" {
		ans := c.UnlessPay
		c.UnlessPay = ""
		switch ans {
		case "pay":
			return
		case "decline":
			// Continue into the sacrifice below.
		default:
			payer := sacrificeAllPayer(h, c, sa)
			d := &decision.Decision{Player: payer, Kind: decision.KModes,
				Min: 1, Max: 1, Source: c.Source, ResumeKind: "unless_pay", ResumeSA: sa,
				Prompt: "Pay " + cost + " to prevent the sacrifice, or decline",
				Options: []decision.Option{
					{Index: 0, Kind: "mode", Label: "Pay " + cost + " — prevent the sacrifice", Obj: c.Source, Player: payer},
					{Index: 1, Kind: "mode", Label: "Don't pay — sacrifice", Obj: c.Source, Player: payer},
				}}
			if h.Ask(d) {
				return
			}
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "may pay declined (UnlessCost not asked on this host)"})
		}
	}

	g := h.Game()
	spec := sa.Params["ValidCards"]
	if spec == "" {
		spec = "Permanent"
	}
	remember := sa.Params["RememberSacrificed"] != ""
	sacrifice := func(id state.ObjID) {
		if h.SacrificeBlocked(id) {
			// A CantSacrifice restriction (Call for Aid) or face static: the
			// permanent stays. Not remembered either — it was not sacrificed.
			return
		}
		o := g.Obj(id)
		if o == nil || o.Zone != state.ZBattlefield {
			return
		}
		if remember {
			c.Sacrificed = append(c.Sacrificed, state.SacrificedInfoOf(g, id))
			// Forge's RememberSacrificed$ also remembers the card (mirroring
			// effSacrifice's rememberLKICapture), which is what a following
			// ConditionDefined$ Remembered, Remembered$Amount or
			// RememberedCard reads, and event-backs it on the source so a
			// later, independently resolving ability sees the same list.
			c.Remembered = append(copyTargets(c.Remembered), state.Target{Obj: id})
			eventRemember(h, c, id)
		}
		h.Emit(events.Sacrifice(id))
	}
	if def := sa.Params["Defined"]; def != "" || sa.Params["ValidTgts"] != "" {
		for _, t := range Defined(h, c, sa) {
			if !t.IsPlayer {
				sacrifice(t.Obj)
			}
		}
		return
	}
	for _, p := range g.AliveFrom(0) {
		ids := append([]state.ObjID(nil), g.Zone(state.ZBattlefield, p)...)
		for _, id := range ids {
			if MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
				sacrifice(id)
			}
		}
	}
}

// sacrificeAllPayer resolves every payer spelling used by the corpus's six
// compensated-sacrifice scripts. EnchantedController is the controller of
// this Aura's attached permanent; the target is deliberately not used because
// the trigger has no target. Unknown future spellings fail conservatively to
// the resolving controller, the same default the other unless-pay effects use.
func sacrificeAllPayer(h Host, c *Ctx, sa *cards.SA) state.PlayerID {
	switch strings.TrimSpace(sa.Params["UnlessPayer"]) {
	case "", "You":
		return c.Controller
	case "EnchantedController":
		if src := h.Game().Obj(c.Source); src != nil && src.AttachedTo != 0 {
			if enchanted := h.Game().Obj(src.AttachedTo); enchanted != nil {
				return enchanted.Controller
			}
		}
	case "Targeted", "TargetedController", "TargetedOrController":
		if len(c.Targets) > 0 {
			return PlayerOf(h, c, c.Targets[0])
		}
	}
	return c.Controller
}
