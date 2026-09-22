package effects

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() { Register("TimeTravel", effTimeTravel) }

// effTimeTravel implements Doctor Who's Time Travel action. The affected
// objects are walked in the game's deterministic order: owned suspended
// cards first, followed by the controller's battlefield permanents. Each
// object gets its own optional add/remove/skip election. A resumed walk
// carries its object index and repetition number in the decision's target
// field (rules decodes that field into Ctx).
func effTimeTravel(h Host, c *Ctx, sa *cards.SA) {
	choice := c.TimeTravelChoice
	done := c.TimeTravelDone
	idx, round := c.TimeTravelIndex, c.TimeTravelRound
	c.TimeTravelChoice, c.TimeTravelDone = "", false

	amount := int(Num(h, c, sa, "Amount", 1))
	if amount < 1 {
		return
	}
	objects := c.TimeTravelObjects
	if len(objects) == 0 {
		objects = timeTravelObjects(h.Game(), c.Controller)
	}
	if len(objects) == 0 {
		return
	}
	if idx < 0 {
		idx = 0
	}
	if round < 0 {
		round = 0
	}
	if idx >= len(objects) {
		idx, round = 0, round+1
	}
	if round >= amount {
		return
	}

	if done {
		if idx < len(objects) {
			applyTimeTravel(h, objects[idx], choice)
		}
		idx++
		if idx >= len(objects) {
			idx = 0
			round++
		}
		if round >= amount {
			return
		}
	}

	for idx < len(objects) {
		id := objects[idx]
		if !timeTravelEligible(h.Game(), id, c.Controller) {
			idx++
			continue
		}
		d := &decision.Decision{Player: c.Controller, Kind: decision.KChoose,
			Min: 1, Max: 1, Source: c.Source,
			ResumeKind: "time_travel", ResumeSA: sa,
			// The low 32 bits carry the snapshot index; the repetition is in
			// the high bits. The rules resume frame stores the offered object
			// list separately, so removals cannot shift this cursor.
			ResumeTarget: (round << 32) | idx,
			Prompt:       "Time travel: add or remove a time counter?"}
		d.Options = append(d.Options,
			decision.Option{Index: 0, Kind: "time_travel_skip", Label: "Skip", Obj: id},
			decision.Option{Index: 1, Kind: "time_travel_add", Label: "Add a time counter", Obj: id},
			decision.Option{Index: 2, Kind: "time_travel_remove", Label: "Remove a time counter", Obj: id})
		if Ask(h, d) == AskAsked {
			return
		}
		// R-9/no-host fallback: decline the optional election. This still
		// traverses every affected object and never emits an unimplemented note.
		applyTimeTravel(h, id, "time_travel_skip")
		idx++
	}
	// A no-host pass may have completed this repetition; repeat Amount times.
	if round+1 < amount {
		for round++; round < amount; round++ {
			for _, id := range objects {
				if timeTravelEligible(h.Game(), id, c.Controller) {
					applyTimeTravel(h, id, "time_travel_skip")
				}
			}
		}
	}
}

func timeTravelObjects(g *state.Game, controller state.PlayerID) []state.ObjID {
	out := make([]state.ObjID, 0)
	// Exiled suspended cards are ordered by the owner's exile zone order. The
	// provenance flag prevents ordinary exiled TIME-counter cards from being
	// treated as suspended.
	for _, id := range g.Zone(state.ZExile, controller) {
		o := g.Obj(id)
		if o != nil && o.Owner == controller && o.CastFlags&state.FlagSuspend != 0 {
			out = append(out, id)
		}
	}
	for _, id := range g.Zone(state.ZBattlefield, controller) {
		o := g.Obj(id)
		if o != nil && o.Controller == controller && o.Counter("TIME") > 0 {
			out = append(out, id)
		}
	}
	return out
}

func timeTravelEligible(g *state.Game, id state.ObjID, controller state.PlayerID) bool {
	o := g.Obj(id)
	if o == nil {
		return false
	}
	if o.Zone == state.ZExile {
		return o.Owner == controller && o.CastFlags&state.FlagSuspend != 0
	}
	return o.Zone == state.ZBattlefield && o.Controller == controller && o.Counter("TIME") > 0
}

func applyTimeTravel(h Host, id state.ObjID, choice string) {
	o := h.Game().Obj(id)
	if o == nil {
		return
	}
	// A suspended card may receive its first TIME counter. Battlefield
	// permanents, by contrast, are eligible only while they already have one.
	if o.Zone == state.ZBattlefield && o.Counter("TIME") <= 0 {
		return
	}
	if o.Zone != state.ZBattlefield && (o.Zone != state.ZExile || o.CastFlags&state.FlagSuspend == 0) {
		return
	}
	switch strings.TrimSpace(choice) {
	case "time_travel_add":
		h.Emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "TIME", Amount: 1})
	case "time_travel_remove":
		h.Emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "TIME", Amount: -1})
	case "time_travel_skip", "":
	default:
		h.Emit(events.Event{Kind: events.Note, Obj: id,
			Text: fmt.Sprintf("TimeTravel: unknown choice %q; skipped", choice)})
	}
}
