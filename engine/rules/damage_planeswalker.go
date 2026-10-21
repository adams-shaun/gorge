package rules

import (
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// markDamageNotPlaneswalker tags a Damage event whose recipient's printed face
// is a planeswalker but whose current, layer-derived types are not (CR 306.8:
// damage removes loyalty only from a planeswalker; Luxior, Giada's Gift and
// Imprisoned in the Moon remove the type). events.Apply cannot read the layer
// system, so the tag (events.DamageNotPlaneswalker on Text) carries the fact
// into the fold, which then marks the damage instead of removing loyalty. It
// runs after the replacement pass, beside the infect and wither markers, so a
// redirected hit is classified by its final recipient. Every other Damage
// event is left untouched.
func (e *Engine) markDamageNotPlaneswalker(ev *events.Event) {
	if ev.Obj == 0 || ev.Amount <= 0 {
		return
	}
	o := e.G.Obj(ev.Obj)
	if o == nil || o.Zone != state.ZBattlefield {
		return
	}
	if f := o.Face(); f == nil || !f.IsPlaneswalker() || e.IsPlaneswalker(ev.Obj) {
		return
	}
	ev.Text = events.DamageNotPlaneswalker
}
