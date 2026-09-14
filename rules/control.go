package rules

import (
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// controlGrant is one GainControl effect that can still end, in the order
// the effects began (CR 613.7 timestamp order within layer 2).
type controlGrant struct {
	effects.ControlGrant
	// untilTurn is the turn at whose cleanup an UntilTheEndOfYourNextTurn
	// grant ends, fixed when the effect begins.
	untilTurn int32
}

// controlMoment says which fixed points of the turn are being passed when
// expireControl runs; the state-based terms are checked at every moment.
type controlMoment uint8

const (
	controlOnEvent controlMoment = iota
	controlAtEndOfCombat
	controlAtCleanup
)

// RegisterControl implements effects.Host. A permanent control change
// supersedes every earlier control effect on that object: nothing that
// expires later can hand it back (CR 613.7), so their records are dropped.
// A grant with a duration is kept until expireControl ends it.
func (e *Engine) RegisterControl(gr effects.ControlGrant) {
	o := e.G.Obj(gr.Obj)
	if o == nil || o.Zone != state.ZBattlefield {
		return
	}
	if gr.Duration.Permanent() {
		kept := e.controlGrants[:0]
		for _, g := range e.controlGrants {
			if g.Obj != gr.Obj {
				kept = append(kept, g)
			}
		}
		e.controlGrants = kept
		return
	}
	cg := controlGrant{ControlGrant: gr}
	if gr.Duration.NextTurn {
		cg.untilTurn = e.nextTurnFor(gr.You)
		if cg.untilTurn == 0 {
			// The effect's controller has no next turn (left the game):
			// CR 800.4a ends the effect at this turn's cleanup at the latest.
			cg.untilTurn = e.G.Turn
		}
	}
	e.controlGrants = append(e.controlGrants, cg)
}

// grantEnded reports whether g's duration is over at moment m.
func (e *Engine) grantEnded(g controlGrant, m controlMoment) bool {
	d := g.Duration
	switch m {
	case controlAtCleanup:
		// CR 514.2. An end-of-combat grant made outside combat has no later
		// combat to wait for, so it also ends here rather than lingering.
		if d.EOT || d.EndOfCombat || (d.NextTurn && g.untilTurn <= e.G.Turn) {
			return true
		}
	case controlAtEndOfCombat:
		if d.EndOfCombat {
			return true
		}
	}
	// CR 800.4a: effects that give a player who left the game control end.
	for _, p := range []state.PlayerID{g.Controller, g.You} {
		if int(p) >= len(e.G.Players) || e.G.Players[p].Lost {
			return true
		}
	}
	return effects.ControlGrantEnded(e, g.ControlGrant)
}

// expireControl ends every tracked control effect whose duration is over and
// hands each affected permanent to the controller the remaining effects give
// it: the latest surviving grant's controller, or the controller the object
// had before the earliest grant. Records of an object that left the
// battlefield are dropped without an event -- that object is new and already
// under its owner's control (CR 400.7). It loops until nothing more ends,
// because a returned permanent can itself be the source of another grant
// (Kellogg changing hands ends Kellogg's own steal).
func (e *Engine) expireControl(m controlMoment) {
	if len(e.controlGrants) == 0 || e.expiringControl {
		return
	}
	e.expiringControl = true
	defer func() { e.expiringControl = false }()
	for pass := 0; pass <= len(e.controlGrants)+1 && len(e.controlGrants) > 0; pass++ {
		old := e.controlGrants
		ended := make([]bool, len(old))
		anyEnded := false
		var live []controlGrant
		for i, g := range old {
			o := e.G.Obj(g.Obj)
			if o == nil || o.Zone != state.ZBattlefield || o.Timestamp != g.ObjStamp {
				ended[i] = true // a new object: its record simply goes
				continue
			}
			if e.grantEnded(g, m) {
				ended[i], anyEnded = true, true
			}
		}
		if !anyEnded {
			for i, g := range old {
				if !ended[i] {
					live = append(live, g)
				}
			}
			e.controlGrants = live
			return
		}
		type change struct {
			obj state.ObjID
			to  state.PlayerID
		}
		var changes []change
		type objKey struct {
			obj   state.ObjID
			stamp uint32
		}
		done := map[objKey]bool{}
		for i, g := range old {
			key := objKey{g.Obj, g.ObjStamp}
			if done[key] {
				continue
			}
			done[key] = true
			o := e.G.Obj(g.Obj)
			if o == nil || o.Zone != state.ZBattlefield || o.Timestamp != g.ObjStamp {
				continue
			}
			// Every record of this object, in timestamp order.
			var all []int
			for j := i; j < len(old); j++ {
				if old[j].Obj == g.Obj && old[j].ObjStamp == g.ObjStamp {
					all = append(all, j)
				}
			}
			base := old[all[0]].Previous
			var remaining []int
			for _, j := range all {
				if !ended[j] {
					remaining = append(remaining, j)
				}
			}
			last := old[all[len(all)-1]]
			want := base
			if len(remaining) > 0 {
				want = old[remaining[len(remaining)-1]].Controller
				// The earliest survivor now carries the base controller.
				first := old[remaining[0]]
				first.Previous = base
				old[remaining[0]] = first
			}
			if int(want) >= len(e.G.Players) || e.G.Players[want].Lost {
				want = o.Owner
			}
			// Only the visible effect changing moves control, and never
			// over a change this tracker did not make.
			if ended[all[len(all)-1]] && o.Controller == last.Controller && want != o.Controller {
				changes = append(changes, change{obj: g.Obj, to: want})
			}
		}
		for i, g := range old {
			if !ended[i] {
				live = append(live, g)
			}
		}
		e.controlGrants = live
		for _, ch := range changes {
			e.emit(events.Event{Kind: events.ControlChange, Obj: ch.obj, Player: ch.to})
		}
		if m != controlOnEvent {
			// The fixed point has been passed; what the returns changed is
			// checked as ordinary state from here on.
			m = controlOnEvent
		}
	}
}
