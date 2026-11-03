package effects

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func init() {
	Register("Intensify", effIntensify)
	// The printed starting value is read through state.Object.IntensityTotal;
	// the keyword has no ability of its own.
	RegisterNonAPI("kw:Starting intensity")
}

// effIntensify is Alchemy's "intensify" / "perpetually increase the
// intensity": each chosen card's intensity rises by Amount$ (default 1).
// Forge's IntensifyEffect: the cards are AllDefined$ (a filter over every card
// in the game, in any zone, matched from the activating player's side, so
// `YouOwn` is the controller) or else Defined$/the targets, which falls back to
// the source. The increase belongs to the card, not to a zone stint, so it is
// one AlterAttribute event per card that Apply folds onto the object and that
// no zone change clears. DefinedDesc$ is stack text only.
func effIntensify(h Host, c *Ctx, sa *cards.SA) {
	amount := Num(h, c, sa, "Amount", 1)
	g := h.Game()
	var ids []state.ObjID
	if spec := sa.Params["AllDefined"]; spec != "" {
		sc := c.SpecContext(c.Controller)
		for i := range g.Objs {
			o := &g.Objs[i]
			if o.Ability != nil || o.Face() == nil {
				continue
			}
			if MatchesSpecCtx(g, spec, o.ID, sc) {
				ids = append(ids, o.ID)
			}
		}
	} else {
		for _, t := range Defined(h, c, sa) {
			if !t.IsPlayer {
				ids = append(ids, t.Obj)
			}
		}
	}
	if amount == 0 {
		return
	}
	for _, id := range ids {
		h.Emit(events.Event{Kind: events.AlterAttribute, Obj: id, Text: "Intensity", Amount: amount})
	}
}
