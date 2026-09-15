// MayPlay statics: an S:Mode$ Continuous ... MayPlay$ True line grants the
// affected cards a CR 118.3a permission to be PLAYED from a zone other than
// the hand ("You may play lands from your graveyard", Conduit of Worlds;
// "You may cast CARDNAME from your graveyard", a Card.Self self-grant).
// Playing such a card is the ordinary play action -- a land drop for a land,
// a cast paying the printed mana cost for a spell -- so the grant only opens
// the offer and the cost; every gate the ordinary hand walk applies
// (timing, restrictions, target availability, castable cost) applies here
// too. The grant lives in rules (not effects) because it is consumed by
// legalActions/beginCast, the same places the hand walk and its cast live.
package rules

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// mayPlayGrant reports whether the card id -- sitting in the acting player
// p's graveyard or exile -- is granted a "may play this card from that zone"
// permission, and whether the grant makes the play free
// (MayPlayWithoutManaCost$ True). Two shapes grant it:
//
//   - a self-grant: the card's own S: static, evaluated while the card sits
//     in the affected zone ("You may cast CARDNAME from your graveyard",
//     Affected$ Card.Self + AffectedZone$ Graveyard). The static's source is
//     the card itself and its controller is the card's controller.
//   - a battlefield grant: a continuous static on a permanent another card
//     controls (Conduit of Worlds' Affected$ Land.YouOwn +
//     AffectedZone$ Graveyard). Only the static's controller benefits: the
//     Affected$ qualifiers (YouOwn, YouCtrl) resolve against that
//     controller, and a static worded for its controller must never hand the
//     permission to an opponent walking their own graveyard.
//
// Gates the build cannot evaluate fail CLOSED (no grant) -- the conservative
// direction for a "may play" permission, the same discipline the filter
// matcher and MatchesPlayerSpec practise: an unimplemented condition must
// withhold the offer, not widen it.
func (e *Engine) mayPlayGrant(p state.PlayerID, id state.ObjID) (free bool, ok bool) {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return false, false
	}
	// (a) Self-grants: the card's own statics, read off its face wherever
	// the card currently sits (an S: line is part of the face, so a card in
	// the graveyard carries it exactly as it would on the battlefield).
	for _, st := range o.Face().Statics {
		if st.Mode != "Continuous" || st.Params["MayPlay"] != "True" {
			continue
		}
		if free, ok := e.mayPlayStatic(st.Params, id, o.Controller, id); ok {
			return free, true
		}
	}
	// (b) Battlefield grants, in activeStatics' deterministic APNAP order.
	for _, sv := range e.activeStatics("Continuous") {
		if sv.Params["MayPlay"] != "True" || sv.Controller != p {
			continue
		}
		if free, ok := e.mayPlayStatic(sv.Params, id, sv.Controller, sv.Source); ok {
			return free, true
		}
	}
	return false, false
}

// mayPlayStatic evaluates one MayPlay$ static's parameters against the card
// id (in its current zone), with `you` the player whose YouOwn/YouCtrl the
// spec's qualifiers resolve against and `source` the static's source object.
// free reports MayPlayWithoutManaCost$ True; ok reports whether the grant
// applies at all.
func (e *Engine) mayPlayStatic(params map[string]string, id state.ObjID, you state.PlayerID, source state.ObjID) (free, ok bool) {
	// Fail closed on gates this build cannot evaluate: CheckSVar$ (a
	// condition the grant is gated on, e.g. Windbrisk Heights' attacker
	// count on its own AB -- a battlefield static carrying one is 11 corpus
	// lines) and MayPlayPlayer$ (a beneficiary other than the static's
	// controller: ActivePlayer, CardOwner, Exiler, Player). Withholding the
	// offer is the conservative direction; both families are named in the
	// AGENTS.md approximations audit.
	if strings.TrimSpace(params["CheckSVar"]) != "" || strings.TrimSpace(params["MayPlayPlayer"]) != "" {
		return false, false
	}
	spec := strings.TrimSpace(params["Affected"])
	if spec == "" {
		// A MayPlay$ static with no Affected$ grants nothing here: Forge's
		// default would be "everything", and granting everything is the
		// widening direction this file refuses.
		return false, false
	}
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return false, false
	}
	// AffectedZone$ (comma list of zone names): where the affected card must
	// sit. Absent grants every zone -- the corpus always spells the zone
	// out (181 S: MayPlay lines all carry it or default sensibly), so the
	// default only serves a script that omits it.
	if az := strings.TrimSpace(params["AffectedZone"]); az != "" {
		inZone := false
		for _, part := range strings.Split(az, ",") {
			if z, known := effects.ZoneFromString(strings.TrimSpace(part)); known && z == o.Zone {
				inZone = true
				break
			}
		}
		if !inZone {
			return false, false
		}
	}
	if !effects.MatchesSpecFrom(e.G, spec, id, you, source) {
		return false, false
	}
	// MayPlayLimit$ (always the literal 1 in the corpus, 45 S: lines): the
	// granted play is once per turn. An unresolvable value stays unenforced,
	// the same convention activationLimitReached applies to a non-literal
	// ActivationLimit$.
	if raw := strings.TrimSpace(params["MayPlayLimit"]); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && e.mayPlayLimitReached(id, n) {
			return false, false
		}
	}
	return strings.EqualFold(params["MayPlayWithoutManaCost"], "True"), true
}

// mayPlayLimitReached reports whether the card has already been played
// `limit` times this turn -- a spell cast (PutOnStack) or moved to the
// battlefield from a non-hand, non-stack zone (a land played from the
// graveyard or exile). The scan mirrors activationLimitReached's: walk the
// log backwards to the last TurnChange. A land returned to the battlefield
// by a non-play effect (a reanimate) also matches the MoveZone shape, so the
// count can overcount for a card that is both reanimated and re-played --
// narrower, never wider, and noted in the AGENTS.md audit.
func (e *Engine) mayPlayLimitReached(id state.ObjID, limit int) bool {
	used := 0
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.TurnChange {
			break
		}
		if ev.Obj != id {
			continue
		}
		if ev.Kind == events.PutOnStack ||
			(ev.Kind == events.MoveZone && ev.To == state.ZBattlefield && ev.From != state.ZHand && ev.From != state.ZStack) {
			used++
		}
	}
	return used >= limit
}

// mayPlayGrantOffers walks the acting player's graveyard and exile and adds
// a play/cast option for every card a MayPlay$ static grants. The returned
// options carry Index 0 -- the caller appends them to its list and reindexes
// (the same discipline the hand walk's add closure practises); the walk order
// is the zone list then each zone's card order, both deterministic slices.
func (e *Engine) mayPlayGrantOffers(p state.PlayerID, sorcery bool) []decision.Option {
	var out []decision.Option
	for _, zn := range []state.Zone{state.ZGraveyard, state.ZExile} {
		for _, id := range e.G.Zone(zn, p) {
			o := e.G.Obj(id)
			f := o.Face()
			if f == nil {
				continue
			}
			free, ok := e.mayPlayGrant(p, id)
			if !ok {
				continue
			}
			if f.IsLand() {
				// A granted land play consumes the ordinary land drop
				// (CR 118.3a: the permission is not an additional drop)
				// and the ordinary sorcery timing. handlePriority's
				// play_land path commits it from the card's current zone.
				if !sorcery || e.G.Players[p].LandsPlayed >= 1 {
					continue
				}
				out = append(out, decision.Option{Index: 0, Kind: "play_land",
					Label: "Play " + f.Name, Obj: id})
				continue
			}
			if e.castRestricted(p, id) || e.castSuppressed(p, id) {
				continue
			}
			instantSpeed := f.IsInstant() || e.HasKeyword(id, "Flash")
			if !instantSpeed && !sorcery {
				continue
			}
			if !e.castTargetsAvailable(p, id, f.SpellAbility()) {
				continue
			}
			cost := Cost{}
			if !free {
				cost = e.rawBaseCost(p, id)
			}
			if e.castable(p, id, e.offerCostFor(p, id, cost, false), false) {
				out = append(out, decision.Option{Index: 0, Kind: "cast",
					Label: "Cast " + f.Name, Obj: id, Mode: "mayplay"})
			}
		}
	}
	return out
}
