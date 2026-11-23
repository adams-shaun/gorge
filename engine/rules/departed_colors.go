package rules

import (
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// trackDepartedColors keeps the layer-derived colours a permanent had on the
// battlefield for as long as the object stays where that departure put it
// (CR 113.7a / 608.2h: an ability whose source has left the battlefield reads
// the source as it last existed). A trigger queued after a simultaneous
// departure (CR 603.10a) has no per-stack snapshot, so a "can't be the target
// of blue sources" restriction would otherwise read the graveyard card's
// printed colour: a Quicksilver Gargantuan that died as a colourless copy
// would still look blue. Any later move of the same object retires the entry,
// so a graveyard ability (scavenge, unearth) reads the card's own colours.
func (e *Engine) trackDepartedColors(ev events.Event) {
	if ev.Kind != events.MoveZone {
		return
	}
	if ev.From == state.ZBattlefield && ev.To != state.ZBattlefield {
		if o := e.G.Obj(ev.Obj); o != nil && o.Zone == state.ZBattlefield {
			if e.departedColors == nil {
				e.departedColors = make(map[state.ObjID]string)
			}
			e.departedColors[ev.Obj] = e.Colors(ev.Obj)
			return
		}
	}
	delete(e.departedColors, ev.Obj)
}

// matchesDepartedSource is matchesSpec for an ability source that left the
// battlefield: the spec reads host's last-known colours (CR 113.7a). It
// reports handled false when host has no standing departure capture or is on
// the battlefield or the stack, and the caller keeps the plain matchesSpec.
// matchesSpec itself rebinds the shared layer-5 table over sc's, so the read
// goes straight to the filter with a private copy of that table plus host's
// captured entry; the shared table is never mutated.
func (e *Engine) matchesDepartedSource(spec string, host state.ObjID, sc effects.SpecContext) (match, handled bool) {
	letters, ok := e.departedColors[host]
	if !ok {
		return false, false
	}
	if o := e.G.Obj(host); o == nil || o.Zone == state.ZBattlefield || o.Zone == state.ZStack {
		return false, false
	}
	src := e.derivedColorTable()
	table := make([]effects.ObjectColors, 0, len(src)+1)
	for _, c := range src {
		if c.ID != host {
			table = append(table, c)
		}
	}
	sc.Layers.DerivedColors = append(table, effects.ObjectColors{ID: host, Mask: effects.ColorMaskFromLetters(letters)})
	return effects.MatchesSpecCtxPtr(e.G, spec, host, &sc), true
}
