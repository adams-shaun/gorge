package effects

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() { Register("SacrificeAll", effSacrificeAll) }

// effSacrificeAll implements the mass-sacrifice primitive (CR 701.16 in its
// multi-object shape; 124 corpus files carry the API). Two shapes cover the
// whole measured population:
//
//   - A Defined$-carrying line (Defined$ Targeted/Remembered/ChosenCard/
//     Self/...): every OBJECT target Defined resolves that is still on the
//     battlefield is sacrificed by its own controller, exactly the object
//     path of effSacrifice -- the targeting step already chose which objects,
//     so ValidCards$ does not re-filter them (and would misfire on the
//     corpus's "Card.ChosenCardStrict"-style specs whose extra predicates the
//     filter grammar fails closed on). `Controller$ You` (15 lines, all
//     planeswalker ultimates' "you sacrifice it" riders) is the same object
//     sacrifice -- the controller named is the sacrificer, which the move's
//     "sacrificed" text records either way.
//
//   - A ValidCards$-carrying (or bare) line with no Defined$: each player
//     sacrifices EVERY permanent they control matching the spec (All Is
//     Dust's "each player sacrifices all permanents they control that are one
//     or more colors"). The default spec is "Permanent" -- CR 701.16's
//     "player sacrifices a permanent" reading, and the same default
//     effSacrifice uses. The spec is evaluated with the resolving spell's
//     controller as the spec's "You" (Emrakul's Creature.YouCtrl means the
//     spell's controller's creatures), matching effDestroyAll's own context
//     choice.
//
// Sacrifice ignores Indestructible and never consults the regeneration
// shield (sacrifice is not destruction, CR 701.16a), the same convention
// effSacrifice documents. RememberSacrificed$ True captures the LKI snapshot
// of each sacrificed object into Ctx.Sacrificed for a chained SubAbility$, the
// same channel effSacrifice feeds.
//
// Deliberately unread (6 corpus lines, all `UnlessCost$` + `UnlessPayer$`
// compensated-sacrifice riders): the Unless machinery that effCounter prices
// is not wired here; those lines' sacrifice degrades to unconditional. A Note
// marks the first one so the gap is visible in a transcript rather than
// silent.
func effSacrificeAll(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	spec := sa.Params["ValidCards"]
	if spec == "" {
		spec = "Permanent"
	}
	remember := sa.Params["RememberSacrificed"] != ""
	unlessMarked := false
	sacrifice := func(id state.ObjID) {
		o := g.Obj(id)
		if o == nil || o.Zone != state.ZBattlefield {
			return
		}
		if remember {
			c.Sacrificed = append(c.Sacrificed, state.SacrificedInfoOf(g, id))
		}
		h.Emit(events.Event{Kind: events.MoveZone, Obj: id,
			From: state.ZBattlefield, To: state.ZGraveyard, Text: "sacrificed"})
	}
	if def := sa.Params["Defined"]; def != "" || sa.Params["ValidTgts"] != "" {
		for _, t := range Defined(h, c, sa) {
			if t.IsPlayer {
				continue
			}
			sacrifice(t.Obj)
		}
		if sa.Params["UnlessCost"] != "" && !unlessMarked {
			unlessMarked = true
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "SacrificeAll UnlessCost$ unread; the sacrifice is unconditional"})
		}
		return
	}
	for _, p := range g.AliveFrom(0) {
		// Snapshot the zone: the emits below mutate it underneath us.
		ids := append([]state.ObjID(nil), g.Zone(state.ZBattlefield, p)...)
		for _, id := range ids {
			if MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
				sacrifice(id)
			}
		}
	}
	if sa.Params["UnlessCost"] != "" && !unlessMarked {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "SacrificeAll UnlessCost$ unread; the sacrifice is unconditional"})
	}
}
