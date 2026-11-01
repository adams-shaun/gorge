package rules

import (
	"fmt"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// station.go implements the two bespoke activated actions this
// keywords need, each with its own offer in legalActions, its own KChoose
// answer flow (handleChoose routes on the chooseFor below), and its own
// effects:
//
//   - kw:Station (CR 702.184a, "Tap another untapped creature you control:
//     Put a number of charge counters on this permanent equal to the tapped
//     creature's power. Activate only as a sorcery."): the offer is gated on
//     the sorcery window (sorcerySpeed) and on at least one legal candidate
//     existing; the KChoose picks the creature to tap; the answer emits the
//     Tap (the cost) and puts the Station ABILITY on the stack (it is an
//     activated ability, not a special action), whose resolution places the
//     CHARGE counters (the amount read from the tapped creature's tap-power
//     value at resolution, stationAmount).
//
//   - Room unlock (CR 309.5, engine/rules/rooms.go): the offer is gated on the
//     sorcery window and on the locked half's mana cost being payable; the
//     answer pays that cost and emits the DoorUnlock event.
//
// Both asks follow the cleanup-discard precedent: the decision suspends
// nothing mid-resolution (no resolution is running), and the answer handler
// is the ask's whole continuation (for Station, up to the stack push).

// chooseStation is the chooseFor for the Station tap pick. iota+13 is
// pairwise distinct from the shared package set as of the mass-turn/
// new-mechanics merge: cast=1/etb=2/miracle=3 (cast.go), cleanup=4/
// damageDivision=5 (combat.go), mana=6/manaColor=7/manaDiscard=8/
// manaExile=9 (mana_activation.go), opening=10 (opening_hand.go),
// suspendCast=11 (turn.go) -- the exact numbers only need to differ.
const chooseStation chooseFor = iota + 13

// chooseUnlock is a marker for the room-unlock flow (no second decision is
// ever asked -- the unlock pays its cost directly on the option answer) -- but
// keeping the name reserved here documents that handlePriority's "unlock"
// case never routes through handleChoose. The zero-value flow that reads it
// is the cast flow's; no handler exists, by design.
const chooseUnlock chooseFor = iota + 14

// stationSAKind is the TapPowerValue ValidSA$ action token a Station static
// scopes to (Tapestry Warden's `ValidSA$ Activated.Station`). It is the
// saKind every Station power read passes to tapPowerValue, so the offer
// label and the counter amount agree.
const stationSAKind = "Station"

func hasCreatureType(types []string) bool {
	for _, typ := range types {
		if typ == "Creature" {
			return true
		}
	}
	return false
}

// stationCandidates lists the creatures p controls that may be tapped for
// Station: on the battlefield and untapped, not the spacecraft itself
// ("Tap ANOTHER creature you control"). Summoning sickness does NOT
// disqualify a creature: sickness (CR 302.6) bars a creature from ATTACKING
// and from activating its OWN tap-cost abilities -- tapping it to pay
// another permanent's activation cost is neither, exactly as crewing a
// Vehicle (CR 702.121b) is. The CR-correct shape is also the simpler one.
func (e *Engine) stationCandidates(p state.PlayerID, station state.ObjID) []state.ObjID {
	var out []state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		if id == station {
			continue
		}
		o := e.G.Obj(id)
		if o == nil || o.Tapped || !hasCreatureType(e.derivedTypesOf(id)) {
			continue
		}
		out = append(out, id)
	}
	return out
}

// askStation poses the Station tap pick for the spacecraft the "station"
// option named. A stale option (no spacecraft, it left play, no candidate
// anymore) degrades to a no-op: nothing is emitted, priority resumes
// untouched.
func (e *Engine) askStation(p state.PlayerID, opt decision.Option) {
	e.stationing = opt.Obj
	o := e.G.Obj(opt.Obj)
	if o == nil || o.Zone != state.ZBattlefield || !e.hasKeywordH(opt.Obj, kwhStation) {
		return
	}
	cands := e.stationCandidates(p, opt.Obj)
	if len(cands) == 0 {
		return
	}
	opts := make([]decision.Option, 0, len(cands))
	for _, id := range cands {
		name := "a creature"
		if co := e.G.Obj(id); co != nil && co.Face() != nil {
			name = co.Face().Name
		}
		opts = append(opts, decision.Option{Index: len(opts), Kind: "station",
			Label: fmt.Sprintf("Tap %s (power %d) to station", name, e.tapPowerValue(id, stationSAKind)),
			Obj:   id, Player: p})
	}
	e.choosing = chooseStation
	e.ask(&decision.Decision{Player: p, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt:  "Station " + o.Face().Name + " — tap another creature you control",
		Options: opts})
}

// handleStation applies the answered Station pick. Station is an activated
// ability (CR 702.184a: "Tap another untapped creature you control: Put a
// number of charge counters on this permanent equal to the tapped creature's
// power. Activate only as a sorcery."), so it uses the stack: the Tap is the
// cost (one event), then events.KeywordAbilityPush mints the ability object
// from the Spacecraft's K:Station line (cards.KeywordPushAbility), with the
// tapped creature in its IDs so the resolution reads that creature's
// tap-power value (stationAmount). The CHARGE counters are the RESOLVING
// ability's effect, placed when every player has passed -- never here. A
// stale answer (the creature left play, or is already tapped) degrades to a
// no-op rather than wedging.
func (e *Engine) handleStation(spacecraft state.ObjID, chosen []decision.Option) {
	e.stationing = 0
	if len(chosen) == 0 {
		return
	}
	id := chosen[0].Obj
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || o.Tapped {
		return
	}
	line := e.stationKeywordLine(spacecraft)
	if line == "" {
		return
	}
	e.emit(events.Event{Kind: events.Tap, Obj: id})
	e.emit(events.Event{Kind: events.KeywordAbilityPush, Player: chosen[0].Player,
		Obj: spacecraft, Counter: line, IDs: []state.ObjID{id}})
}

// stationKeywordLine is the Spacecraft's printed K:Station line ("Station:8"),
// the Counter a KeywordAbilityPush carries so a replay re-synthesizes the
// identical body. A Station keyword with no printed line (a grant) is the
// bare head. "" when the object is gone.
func (e *Engine) stationKeywordLine(spacecraft state.ObjID) string {
	o := e.G.Obj(spacecraft)
	if o == nil || o.Face() == nil {
		return ""
	}
	if param, ok := o.Face().KeywordParam(kwhStation.S); ok && param != "" {
		return kwhStation.S + ":" + param
	}
	return kwhStation.S
}

// isStationAbility reports whether ab is the Station ability's resolving
// body (cards.KeywordPushAbility's Station arm, tagged Keyword$ Station).
func isStationAbility(ab *cards.SA) bool {
	return ab != nil && ab.API == "PutCounter" && pay.SaHasKeyword(ab, stationSAKind)
}

// stationAmount is the number of CHARGE counters a resolving Station ability
// puts: the tapped creature's power (CR 702.184a), read through
// tapPowerValue so a Station-scoped stat:TapPowerValue static (Tapestry
// Warden's toughness) substitutes, at RESOLUTION -- the ability's effect
// reads the creature's power as it resolves. The creature is the first
// object the activation's push remembered. ok is false for any other ability.
// A tapped creature that has since left the battlefield is read in its new
// zone (its printed power there): an approximation of CR 608.2h's last known
// information, which this build does not snapshot for a tap cost.
func (e *Engine) stationAmount(o *state.Object) (int32, bool) {
	if o == nil || !isStationAbility(o.Ability) {
		return 0, false
	}
	for _, rem := range o.Remembered {
		if rem.IsPlayer || rem.Obj == 0 {
			continue
		}
		if e.G.Obj(rem.Obj) == nil {
			return 0, true
		}
		n := e.tapPowerValue(rem.Obj, stationSAKind)
		if n < 0 {
			n = 0
		}
		return n, true
	}
	return 0, true
}
