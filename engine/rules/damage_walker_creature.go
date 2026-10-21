package rules

import (
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// recomputeWalkerCreatureMarker is CR 120.3c/120.3e's recipient form for a
// planeswalker that is also a creature only through a layer effect (an
// animated Gideon, Sarkhan the Masterless's dragons): events.Apply's Damage
// fold removes loyalty from any printed planeswalker but marks damage only on
// a printed creature or a "creature"-tagged event, because it cannot read the
// layer state. Every Damage event to an object passes here after the whole
// replacement pass, so the tag is derived once, for every emitter (combat
// damage, spell damage, a redirect onto the walker, and the negative clears of
// cleanup, regeneration and healing), from the same layer-accurate IsCreature
// the infect and wither markers use. Only an untagged or "creature"-tagged
// event is touched: an infect or wither marker already carries the creature
// half, and a printed creature needs no tag.
func (e *Engine) recomputeWalkerCreatureMarker(ev *events.Event) {
	if ev.Obj == 0 || (ev.Counter != "" && ev.Counter != "creature") {
		return
	}
	o := e.G.Obj(ev.Obj)
	if o == nil || o.Zone != state.ZBattlefield {
		return
	}
	f := o.Face()
	if f == nil || !f.IsPlaneswalker() || f.IsCreature() {
		return
	}
	if e.IsCreature(ev.Obj) {
		ev.Counter = "creature"
	} else {
		ev.Counter = ""
	}
}
