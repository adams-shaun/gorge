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
func trackDepartedColors(m map[state.ObjID]string, g *state.Game, r colorReader, ev events.Event) map[state.ObjID]string {
	if ev.Kind != events.MoveZone {
		return m
	}
	if ev.From == state.ZBattlefield && ev.To != state.ZBattlefield {
		if o := g.Obj(ev.Obj); o != nil && o.Zone == state.ZBattlefield {
			if m == nil {
				m = make(map[state.ObjID]string)
			}
			m[ev.Obj] = r.Colors(ev.Obj)
			return m
		}
	}
	delete(m, ev.Obj)
	return m
}

// colorReader is the slice of the Engine the departure capture and the bound
// read need: layer-derived colours and the shared layer-5 table.
type colorReader interface {
	Colors(state.ObjID) string
	derivedColorTable() []effects.ObjectColors
}

// matchesDepartedSource is matchesSpec for an ability source that left the
// battlefield: the spec reads host's last-known colours (CR 113.7a). It
// reports handled false when host has no standing departure capture or is on
// the battlefield or the stack, and the caller keeps the plain matchesSpec.
// matchesSpec itself rebinds the shared layer-5 table over sc's, so the read
// goes straight to the filter with a private copy of that table plus host's
// captured entry; the shared table is never mutated.
func matchesDepartedSource(g *state.Game, m map[state.ObjID]string, r colorReader, spec string, host state.ObjID, sc effects.SpecContext) (match, handled bool) {
	letters, ok := m[host]
	if !ok {
		return false, false
	}
	if o := g.Obj(host); o == nil || o.Zone == state.ZBattlefield || o.Zone == state.ZStack {
		return false, false
	}
	src := r.derivedColorTable()
	table := make([]effects.ObjectColors, 0, len(src)+1)
	for _, c := range src {
		if c.ID != host {
			table = append(table, c)
		}
	}
	sc.Layers.DerivedColors = append(table, effects.ObjectColors{ID: host, Mask: effects.ColorMaskFromLetters(letters)})
	return effects.MatchesSpecCtxPtr(g, spec, host, &sc), true
}
