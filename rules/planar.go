package rules

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Planar-die face constants (CR 901.3a): the planar die is a six-sided die
// with four blank faces, one planeswalk face and one chaos face. Forge rolls
// it as an ordinary d6 with the 5/6 split; rules/replacement.go's
// planarDieFaceName uses the same split for the per-die Notes.
const (
	planarDiePlaneswalk int32 = 5
	planarDieChaos      int32 = 6
)

// currentPlane returns the CURRENT plane of one seat's planar deck (CR 901):
// the face-up top card of that seat's ZPlanarDeck zone. It returns nil when
// the seat has no planar deck, no revealed plane, or the zone and the object
// disagree — a nil return means "no plane", so every caller treats it as the
// plain no-plane-deck degrade.
func (e *Engine) currentPlane(p state.PlayerID) *state.Object {
	if int(p) >= len(e.G.Players) {
		return nil
	}
	ids := e.G.Zone(state.ZPlanarDeck, p)
	if len(ids) == 0 {
		return nil
	}
	o := e.G.Obj(ids[0])
	if o == nil || o.FaceDown || o.Zone != state.ZPlanarDeck {
		return nil
	}
	return o
}

// planarRollConsequences applies the consequences a completed planar-dice
// roll's kept results have under CR 901.4: a kept result on the planeswalk
// face moves the roller to the next plane of their planar deck (the same
// PlanarWalk event events.Apply folds, so a PlaneswalkedTo plane change is
// replay-derived), and a kept result on the chaos face makes chaos ensue on
// the roller's current plane. The consequences run after the roll record is
// logged, from rules/engine.go's single emit tail, so the log order is always
// die Notes -> roll record -> PlanarWalk/ChaosEnsues.
//
// A game with no planar deck has no plane to walk away from and no plane to
// erupt in chaos, so a kept planeswalk/chaos result is recorded and inert —
// the same documented degrade the roll itself has always had. Only KEPT
// results (ev.IDs) carry consequences: an ignored result (the Ichor Elixir
// class) never happened for CR 901.4's purposes.
func (e *Engine) planarRollConsequences(ev events.Event) {
	for _, r := range ev.IDs {
		switch int32(r) {
		case planarDiePlaneswalk:
			// CR 901.4: the player who rolled the planeswalk face
			// planeswalks. e.Planeswalk is a no-op when the roller has no
			// planar deck, so the result is recorded and nothing moves.
			e.Planeswalk(ev.Player)
		case planarDieChaos:
			e.chaosEnsues(ev.Player)
		}
	}
}

// chaosEnsues emits the ChaosEnsues marker for one seat's current plane (CR
// 901.9). The plane's own chaos ability is an ordinary triggered ability
// (Mode$ ChaosEnsues) the trigger walk queues when the marker is checked; a
// game with no current plane records nothing here — the verb degrade lives
// in effects/planechase.go, not in the roll dispatch.
func (e *Engine) chaosEnsues(p state.PlayerID) {
	plane := e.currentPlane(p)
	if plane == nil {
		return
	}
	e.emit(events.Event{Kind: events.ChaosEnsues, Player: p, Obj: plane.ID})
}

// chaosEnsuesMatches implements Mode$ ChaosEnsues (CR 901.9): the current
// plane's chaos ability fires on the events.ChaosEnsues marker the roll
// dispatch and the DB$ ChaosEnsues verb emit. The marker already names the
// plane it erupts on (ev.Obj) and the seat whose planar deck owns it
// (ev.Player), and checkChaosEnsuesTriggers has already narrowed the walk to
// that plane, so the matcher only has to confirm the marker is about THIS
// source. A bare marker with no plane (a Describe-coverage fuzz event)
// matches nothing.
func (e *Engine) chaosEnsuesMatches(t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
	if ev.Kind != events.ChaosEnsues || ev.Obj == 0 {
		return false
	}
	return ev.Obj == source
}

// checkChaosEnsuesTriggers queues the current plane's Mode$ ChaosEnsues
// ability when the chaos-ensues marker is checked (CR 901.9). The plane
// lives in the private ZPlanarDeck zone, which forEachObject's
// ZLibrary..ZStack walk never visits, so the ordinary per-object trigger
// walk cannot reach it: this is the per-event synthetic scan site (the
// checkChapterTriggers / checkRingEmblemTriggers precedent). The plane is
// the marker's own Obj -- the roll path and the DB$ ChaosEnsues verb both
// name the plane they erupt on -- and its face is face-up (genesis
// reveals the top plane), so its printed triggers and Execute$ body resolve
// exactly as any other face's do.
//
// TriggerZones$ Command (every corpus ChaosEnsues carrier spells it) is
// not run through zoneGate: a gorge plane is in ZPlanarDeck, not
// ZCommand, so the ordinary Command gate would reject the plane's own
// zone. This scan is already scoped to the one plane the marker names and
// to the one mode, so the zone gate carries no information the marker did
// not.
func (e *Engine) checkChaosEnsuesTriggers(ev events.Event) {
	if ev.Kind != events.ChaosEnsues || ev.Obj == 0 {
		return
	}
	o := e.G.Obj(ev.Obj)
	if o == nil || o.Zone != state.ZPlanarDeck || o.FaceDown {
		return
	}
	f := o.Face()
	if f == nil {
		return
	}
	// The plane's chaos ability is controlled by the plane's controller (CR
	// 901.9), which the planar-deck genesis set to the deck's owner -- the
	// same seat the marker's Player names. Read the controller off the
	// object so a marker with a zero or stale Player still queues against
	// the plane's real owner.
	controller := o.Controller
	key := triggerKey{Source: ev.Obj, Idx: -1}
	for ti, t := range f.Triggers {
		if t.Mode != "ChaosEnsues" || t.Effect == nil {
			continue
		}
		key.Idx = ti
		if e.triggerFireCount == nil {
			e.triggerFireCount = map[triggerKey]int32{}
		}
		if e.triggerFireCount[key] >= maxTriggerFires {
			continue
		}
		e.triggerFireCount[key]++
		e.pendingTriggers = append(e.pendingTriggers, pendingTrigger{
			Source:     ev.Obj,
			Controller: controller,
			Idx:        ti,
			SA:         t.Effect,
			Ctx: effects.Ctx{
				Source:         ev.Obj,
				Controller:     controller,
				TriggerContext: effects.TriggerContext{TriggerCard: ev.Obj},
			},
		})
	}
}
