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
	// kwStamp is the timestamp of the AddKWs$ keyword grant registered with
	// this control effect (0 for none); it is removed when the grant ends.
	kwStamp uint32
	// static marks a grant the static-control reconcile (rules/
	// control_static.go) registered from an S:Mode$ Continuous GainControl$
	// static (Mind Control's "You control enchanted creature"). Its Duration
	// is the zero (Permanent) value: the grant ends not on a duration but
	// when its static stops being live -- staticGrantLive re-derives the
	// scan's wanted set (the source left the battlefield, the "as long as"
	// gate flipped, the Aura moved bearers, the named player -- e.g. the
	// monarch -- changed) and grantEnded reads it, so the bearer returns
	// through expireControl's ordinary Previous chain exactly like an API
	// grant ending.
	static bool
}

// controlMoment says which fixed points of the turn are being passed when
// expireControl runs; the state-based terms are checked at every moment.
type controlMoment uint8

const (
	controlOnEvent controlMoment = iota
	controlAtEndOfCombat
	controlAtCleanup
)

// RegisterControl implements effects.Host: every control change is recorded
// in timestamp order until expireControl ends it or its object leaves the
// battlefield.
func (e *Engine) RegisterControl(gr effects.ControlGrant) {
	o := e.G.Obj(gr.Obj)
	if o == nil || o.Zone != state.ZBattlefield {
		return
	}
	// AddKWs$: the keywords are a layer-6 grant on the object itself, so the
	// ordinary source-leaves rule already ends them when it leaves the
	// battlefield; a grant with a shorter duration removes them early.
	var kwStamp uint32
	if len(gr.AddKeywords) > 0 {
		e.AddContinuous(ContinuousEffect{Source: gr.Obj, Affects: "Card.Self", Controller: gr.Controller,
			Layer: state.LAbilities, AddKeywords: append([]string(nil), gr.AddKeywords...)})
		kwStamp = e.G.Clock
	}
	// A permanent change is recorded too: as the latest grant it keeps every
	// earlier grant's expiry from moving control (CR 613.7), it ends if its
	// controller leaves the game (CR 800.4a), and its record drops -- with its
	// keywords -- when the object leaves the battlefield.
	cg := controlGrant{ControlGrant: gr, kwStamp: kwStamp}
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
	// A static-derived grant's lifetime is its static's liveness, not a
	// duration: the zero Duration above never ends it, so this is the one
	// check that can. staticControlWants re-derives the live scan's wanted
	// set (cheap when no GainControl static is in play: the memo epoch
	// guard plus a slice walk); a grant is live only while the SAME source
	// object still carries the live static over the SAME bearer and the
	// resolved controller has not moved (the monarch changed). The CR
	// 800.4a check below still applies on top.
	if g.static && !e.staticGrantLive(g) {
		return true
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
	// Fast path, run on every state-changing emit while any grant exists:
	// nothing ends and nothing left the battlefield, so no allocation.
	changing := false
	for _, g := range e.controlGrants {
		if o := e.G.Obj(g.Obj); o == nil || o.Zone != state.ZBattlefield || o.Timestamp != g.ObjStamp || e.grantEnded(g, m) {
			changing = true
			break
		}
	}
	if !changing {
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
			// A record dropped because its object left the battlefield takes
			// its keyword grant with it: the returning permanent is a new
			// object (CR 400.7) that must not keep a stolen creature's haste.
			e.dropControlKeywords(old, ended)
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
		e.dropControlKeywords(old, ended)
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

// dropControlKeywords removes the AddKWs$ keyword grants of the ended grants.
// It rewrites e.continuous in place and bumps continuousVersion, the same
// discipline EndOfTurnCleanup keeps for its own in-place removals.
func (e *Engine) dropControlKeywords(grants []controlGrant, ended []bool) {
	removed := false
	for i, g := range grants {
		if !ended[i] || g.kwStamp == 0 {
			continue
		}
		kept := e.continuous[:0]
		for _, ce := range e.continuous {
			if ce.Timestamp == g.kwStamp && ce.Source == g.Obj && len(ce.AddKeywords) > 0 {
				removed = true
				continue
			}
			kept = append(kept, ce)
		}
		e.continuous = kept
	}
	if removed {
		e.continuousVersion++
	}
}
