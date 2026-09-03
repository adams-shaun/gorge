package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() {
	Register("ChangeZone", effChangeZone)
	Register("ChangeZoneAll", effChangeZoneAll)
	Register("Destroy", effDestroy)
	Register("DestroyAll", effDestroyAll)
	Register("Sacrifice", effSacrifice)
}

// ParseZone maps a Forge zone name to a state.Zone. Unknown names resolve to
// the graveyard, which is where the overwhelming majority of movement goes
// and is a safe default for an unmodelled destination.
func ParseZone(s string) state.Zone {
	switch strings.TrimSpace(s) {
	case "Hand":
		return state.ZHand
	case "Battlefield":
		return state.ZBattlefield
	case "Library":
		return state.ZLibrary
	case "Exile":
		return state.ZExile
	case "Stack":
		return state.ZStack
	}
	return state.ZGraveyard
}

func effChangeZone(h Host, c *Ctx, sa *cards.SA) {
	to := ParseZone(sa.Params["Destination"])
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		o := h.Game().Obj(t.Obj)
		if o == nil {
			continue
		}
		// Origin$, when given, is a precondition: the object must actually be
		// where the script expects, or the movement does not happen. This is
		// also this build's only CR 608.2b guard for ChangeZone: a target
		// moved away by an earlier effect in the same resolution, or by a
		// response that has already resolved, is simply skipped rather than
		// moved a second time or moved from the wrong zone.
		if from, ok := sa.Params["Origin"]; ok && o.Zone != ParseZone(from) {
			continue
		}
		h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: o.Zone, To: to})
	}
}

func effChangeZoneAll(h Host, c *Ctx, sa *cards.SA) {
	from, to := ParseZone(sa.Params["Origin"]), ParseZone(sa.Params["Destination"])
	spec := sa.Params["ChangeType"]
	if spec == "" {
		spec = "Card"
	}
	g := h.Game()
	for _, p := range g.AliveFrom(0) {
		// Snapshot the zone: emitting move events mutates it underneath us.
		ids := append([]state.ObjID(nil), g.Zone(from, p)...)
		for _, id := range ids {
			if MatchesSpecFrom(g, spec, id, c.Controller, c.Source) {
				h.Emit(events.Event{Kind: events.MoveZone, Obj: id, From: from, To: to})
			}
		}
	}
}

// effDestroy is a single-target removal effect: exactly the shape CR 608.2b
// target rechecking exists for. Today the only recheck is "does the target
// still exist, and is it still on the battlefield" -- a target that stayed on
// the battlefield but became newly ineligible some other way (e.g. it gained
// Indestructible in response, or protection from the source) between
// targeting and resolution is not rechecked. See the Task 18 report.
func effDestroy(h Host, c *Ctx, sa *cards.SA) {
	for _, t := range Defined(h, c, sa) {
		o := h.Game().Obj(t.Obj)
		if t.IsPlayer || o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		if o.Face() != nil && o.Face().HasKeyword("Indestructible") {
			continue
		}
		h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID,
			From: state.ZBattlefield, To: state.ZGraveyard, Text: "destroyed"})
	}
}

func effDestroyAll(h Host, c *Ctx, sa *cards.SA) {
	spec := sa.Params["ValidCards"]
	if spec == "" {
		spec = "Permanent"
	}
	g := h.Game()
	for _, p := range g.AliveFrom(0) {
		ids := append([]state.ObjID(nil), g.Zone(state.ZBattlefield, p)...)
		for _, id := range ids {
			o := g.Obj(id)
			if o.Face() != nil && o.Face().HasKeyword("Indestructible") {
				continue
			}
			if MatchesSpecFrom(g, spec, id, c.Controller, c.Source) {
				h.Emit(events.Event{Kind: events.MoveZone, Obj: id,
					From: state.ZBattlefield, To: state.ZGraveyard, Text: "destroyed"})
			}
		}
	}
}

// Sacrifice ignores Indestructible: sacrificing is not destruction. Same
// CR 608.2b caveat as effDestroy: only existence-and-zone is rechecked.
func effSacrifice(h Host, c *Ctx, sa *cards.SA) {
	for _, t := range Defined(h, c, sa) {
		o := h.Game().Obj(t.Obj)
		if t.IsPlayer || o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID,
			From: state.ZBattlefield, To: state.ZGraveyard, Text: "sacrificed"})
	}
}
