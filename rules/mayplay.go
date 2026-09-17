// MayPlay statics: an S:Mode$ Continuous ... MayPlay$ True line grants the
// affected cards a CR 118.3a permission to be PLAYED from a zone other than
// the hand ("You may play lands from your graveyard", Conduit of Worlds;
// "You may cast CARDNAME from your graveyard", a Card.Self self-grant;
// "You may play lands from the top of your library", Ka-Zar of the Savage
// Land). Playing such a card is the ordinary play action -- a land drop for
// a land, a cast paying the printed mana cost for a spell -- so the grant
// only opens the offer and the cost; every gate the ordinary hand walk
// applies (timing, restrictions, target availability, castable cost) applies
// here too. The grant lives in rules (not effects) because it is consumed by
// legalActions/beginCast, the same places the hand walk and its cast live.
package rules

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// mayPlayGrant reports whether the card id -- sitting in the acting player
// p's graveyard, exile or library -- is granted a "may play this card from
// that zone" permission, and whether the grant makes the play free
// (MayPlayWithoutManaCost$ True). Statics of two shapes grant it:
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
//     permission to an opponent walking their own zones.
//
// ok and free are collected independently over the WHOLE scan, not from the
// first hit, because Forge splits the two roles across statics:
// MayPlayDontGrantZonePermissions$ True marks a static that only exempts the
// mana cost and does NOT open the zone permission -- so a granting static and
// a free-casting static must be allowed to cooperate (the permission from
// one, the exemption from the other), and a lone DontGrant static grants
// nothing.
//
// Gates the build cannot evaluate fail CLOSED (no grant) -- the conservative
// direction for a "may play" permission, the same discipline the filter
// matcher and MatchesPlayerSpec practise: an unimplemented condition must
// withhold the offer, not widen it. The full fail-closed list is in
// mayPlayStatic.
func (e *Engine) mayPlayGrant(p state.PlayerID, id state.ObjID) (free, ok bool) {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return false, false
	}
	// (a) Self-grants: the card's own statics, read off its face wherever
	// the card currently sits (an S: line is part of the face, so a card in
	// the graveyard carries it exactly as it would on the battlefield).
	for _, st := range o.Face().Statics {
		if st.Mode != "Continuous" {
			continue
		}
		_, grants, staticFree := e.mayPlayStatic(st.Params, id, o.Controller, id)
		if staticFree {
			free = true
		}
		if grants {
			ok = true
		}
	}
	// (b) Battlefield grants, in activeStatics' deterministic APNAP order.
	for _, sv := range e.activeStatics("Continuous") {
		if sv.Controller != p {
			continue
		}
		_, grants, staticFree := e.mayPlayStatic(sv.Params, id, sv.Controller, sv.Source)
		if staticFree {
			free = true
		}
		if grants {
			ok = true
		}
	}
	return free, ok
}

// mayPlayUnreadGates are the gating parameters a MayPlay$ static can carry
// that this build neither implements nor can safely ignore. Each one either
// further conditions the permission (ValidAfterStack$, the SVar condition
// family, ValidSA$, ActivationZone$) or changes what playing the card costs
// (RaiseCost$) -- and an uncharged surcharge is exactly
// the widening this file refuses: an offer the engine prices incorrectly must
// not exist at all. Any of these present fails the static closed, so the
// card is simply not offered. MayPlayAltManaCost$ is NOT in this list: it is
// genuinely consumed, but on a different path -- mayPlayAltCosts delivers it
// for the ordinary cast walk (alternativeCosts), while the ZONE-permission
// grant below still refuses a cost-carrying static that would have the
// may-play cast pay the printed or free cost (the altCostWithholding check
// in mayPlayStatic).
var mayPlayUnreadGates = [...]string{
	"ValidAfterStack", "SVarCompare", "CheckSecondSVar", "CheckThirdSVar",
	"PresentCompare", "ValidSA", "ActivationZone", "CharacteristicDefining",
	"RaiseCost",
}

// mayPlayGateRejected reports whether a MayPlay$ static carries one of the
// gates this build cannot evaluate -- the mayPlayUnreadGates family plus
// CheckSVar$ (a condition the grant is gated on, e.g. Windbrisk Heights'
// attacker count on its own AB) and MayPlayPlayer$ (a beneficiary other than
// the static's controller: ActivePlayer, CardOwner, Exiler, Player). The
// static is withheld whole -- withholding the offer is the conservative
// direction; both families are named in the AGENTS.md audit.
//
// The literal per-key reads keep the parameter scan classifiable, but they
// are a RECOGNITION, not a consumption: the static never grants, so the key
// stays unread in the parameter census's sense. The census scopes these
// reads away from every card-side label (apiSpecificRulesStat in
// paramcensus_test.go names the functions it excludes), so a MayPlay static
// carrying one of these keys keeps its unread-param label instead of the
// recognition masking it -- Evendo Brushrazer's CheckSVar$, for example, is
// still reported unread because its may-play is withheld entirely, while its
// Condition$ PlayerTurn (which mayPlayStatic genuinely evaluates on the
// family) is not.
func mayPlayGateRejected(params map[string]string) bool {
	if strings.TrimSpace(params["ValidAfterStack"]) != "" ||
		strings.TrimSpace(params["SVarCompare"]) != "" ||
		strings.TrimSpace(params["CheckSecondSVar"]) != "" ||
		strings.TrimSpace(params["CheckThirdSVar"]) != "" ||
		strings.TrimSpace(params["PresentCompare"]) != "" ||
		strings.TrimSpace(params["ValidSA"]) != "" ||
		strings.TrimSpace(params["ActivationZone"]) != "" ||
		strings.TrimSpace(params["CharacteristicDefining"]) != "" ||
		strings.TrimSpace(params["RaiseCost"]) != "" {
		return true
	}
	return strings.TrimSpace(params["CheckSVar"]) != "" || strings.TrimSpace(params["MayPlayPlayer"]) != ""
}

// mayPlayStatic evaluates one MayPlay$ static's parameters against the card
// id (in its current zone), with `you` the player whose YouOwn/YouCtrl the
// spec's qualifiers resolve against and `source` the static's source object.
// Three booleans, reported independently:
//
//   - applies: the static covers this card at all (its gates pass, its
//     Affected$/AffectedZone$ match). A DontGrant static that applies is
//     still only a cost exemption candidate.
//   - grants: applies AND the static opens the zone permission itself --
//     MayPlay$ True and not MayPlayDontGrantZonePermissions$ True.
//   - free: applies AND MayPlayWithoutManaCost$ True.
func (e *Engine) mayPlayStatic(params map[string]string, id state.ObjID, you state.PlayerID, source state.ObjID) (applies, grants, free bool) {
	if strings.TrimSpace(params["MayPlay"]) != "True" {
		// "MayPlay$ You" (1 corpus line) and any other value: this build
		// implements the plain permission, nothing else.
		return false, false, false
	}
	// Fail closed on gates this build cannot evaluate: the
	// mayPlayUnreadGates family plus CheckSVar$/MayPlayPlayer$ (see
	// mayPlayGateRejected's doc -- a recognition, not a consumption).
	if mayPlayGateRejected(params) {
		return false, false, false
	}
	// altCostWithholding: a static that prices the play itself
	// (MayPlayAltManaCost$) and still OPENS the zone permission would have
	// the may-play cast pay the printed or free cost -- the widening this
	// file refuses, because the may-play cast path cannot charge the
	// alternative. The cost's real delivery is mayPlayAltCosts, which
	// serves the ordinary cast walk (a hand cast pays the alt cost as its
	// own cast option); the zone permission stays closed until that cast
	// path can price it. A MayPlayDontGrantZonePermissions$ static (the
	// corpus's dominant carrier, Darksteel Monolith) never granted here
	// anyway, so this only formalises the boundary for a future
	// grant-and-reprice static.
	if _, alt := params["MayPlayAltManaCost"]; alt &&
		!strings.EqualFold(params["MayPlayDontGrantZonePermissions"], "True") {
		return false, false, false
	}
	// Condition$ PlayerTurn ("during each of your turns", Kess, Dissident
	// Mage): the static's controller's turn. Any other Condition$ value is
	// an unimplemented gate and fails closed.
	switch cond := strings.TrimSpace(params["Condition"]); cond {
	case "":
	case "PlayerTurn":
		if e.G.Active != you {
			return false, false, false
		}
	default:
		return false, false, false
	}
	// IsPresent$ ("as long as a Zombie is on the battlefield", Gravecrawler):
	// the static applies only while at least one object on the battlefield
	// matches the spec. A spec nothing on the battlefield satisfies --
	// including one whose predicate this build cannot evaluate (unknown
	// predicates fail closed) -- withholds the static, never widens it.
	if ip := strings.TrimSpace(params["IsPresent"]); ip != "" && !e.mayPlayIsPresent(ip, you, source) {
		return false, false, false
	}
	spec := strings.TrimSpace(params["Affected"])
	if spec == "" {
		// A MayPlay$ static with no Affected$ grants nothing here: Forge's
		// default would be "everything", and granting everything is the
		// widening direction this file refuses.
		return false, false, false
	}
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return false, false, false
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
			return false, false, false
		}
	}
	if !effects.MatchesSpecFrom(e.G, spec, id, you, source) {
		return false, false, false
	}
	// MayPlayLimit$ (always the literal 1 in the corpus, 45 S: lines): the
	// granted play is once per turn. An unresolvable value stays unenforced,
	// the same convention activationLimitReached applies to a non-literal
	// ActivationLimit$.
	if raw := strings.TrimSpace(params["MayPlayLimit"]); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && e.mayPlayLimitReached(id, n) {
			return false, false, false
		}
	}
	grants = !strings.EqualFold(params["MayPlayDontGrantZonePermissions"], "True")
	free = strings.EqualFold(params["MayPlayWithoutManaCost"], "True")
	return true, grants, free
}

// isPresent scans the battlefield (Forge's IsPresent$ default zone) for at
// least one object matching spec -- Gravecrawler's `IsPresent$ Zombie.YouCtrl`.
// The spec's controller-relative qualifiers resolve against `you`, Self and
// CARDNAME against `source`, exactly like the Affected$ spec. AliveFrom(0)
// keeps the scan deterministic and empty-safe.
func (e *Engine) mayPlayIsPresent(spec string, you state.PlayerID, source state.ObjID) bool {
	for _, p := range e.G.AliveFrom(0) {
		for _, oid := range e.G.Zone(state.ZBattlefield, p) {
			if effects.MatchesSpecFrom(e.G, spec, oid, you, source) {
				return true
			}
		}
	}
	return false
}

// mayPlayLimitReached reports whether the card has already been played
// `limit` times this turn -- a spell cast (PutOnStack) or moved to the
// battlefield from a non-hand, non-stack zone (a land played from the
// graveyard, exile or the top of the library). The scan mirrors
// activationLimitReached's: walk the log backwards to the last TurnChange.
// A land returned to the battlefield by a non-play effect (a reanimate) also
// matches the MoveZone shape, so the count can overcount for a card that is
// both reanimated and re-played -- narrower, never wider, and noted in the
// AGENTS.md audit.
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

// mayPlayAltCosts delivers a MayPlay static's MayPlayAltManaCost$ as an
// alternative cost for the ORDINARY cast walk (rules/statics.go's
// alternativeCosts appends these after its AlternativeCost statics): "Once
// each turn, you may pay {0} rather than pay the mana cost for a colorless
// spell that you cast from your hand" (Darksteel Monolith's
// MayPlayAltManaCost$ 0). The static's own gate chain decides whether the
// affected card is offered the alternative at all, exactly like the zone
// permission's gates decide whether the card may play from elsewhere --
// same gates, different delivery.
//
// Sources are the same two the permission grant walks: battlefield
// continuous statics (activeStatics, APNAP order) and the card's own face
// statics (a self-referential S: line -- activeStatics alone never sees a
// static on a card still in hand). Order is deterministic and STABLE
// between the offer walk (legal.go's alternativeCosts loop) and beginCast's
// re-resolution of the chosen AltCostIndex, because both call this function
// over the same board.
//
// Fail-closed discipline, the family's own:
//
//   - MayPlay$ True is required (the corpus pairs every MayPlayAltManaCost$
//     with MayPlay$ True, 27 of 27 carrier files at the corpus pin).
//   - the remaining mayPlayUnreadGates plus CheckSVar$/MayPlayPlayer$
//     withhold the static whole (mayPlayGateRejected).
//   - Condition$, IsPresent$, Affected$, AffectedZone$ and MayPlayLimit$
//     are evaluated exactly as mayPlayStatic evaluates them.
//   - the value is priced through ParseCost; a token this build cannot
//     model (Valgavoth, Terror Eater's dynamic PayLife<ConvertedManaCost>)
//     leaves Cost.Unknown non-empty and the alternative is NOT offered --
//     an unpriceable cost must never exist as an option.
//
// MayPlayLimit$ is enforced per AFFECTED CARD (mayPlayLimitReached's
// per-card log walk), the same reading the zone-permission family applies:
// a colorless spell already played this turn cannot use the discount again.
// The Monolith's printed "Once each turn" is a per-STATIC limit, which this
// per-card tracking under-enforces (two different colorless spells can each
// use it once in one turn) -- the same under-enforcement Forge's per-card
// MayPlayTurn carries; named in the deck import report's Issues.
func (e *Engine) mayPlayAltCosts(p state.PlayerID, id state.ObjID) []Cost {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return nil
	}
	var out []Cost
	// The two sources the permission grant walks, merged into one
	// deterministic scan (battlefield statics first, then the card's own
	// face statics): the copy keeps activeStatics' slice from being
	// appended to in place.
	statics := append([]staticView(nil), e.activeStatics("Continuous")...)
	for _, st := range o.Face().Statics {
		if st.Mode == "Continuous" {
			statics = append(statics, staticView{Params: st.Params, Source: id, Controller: o.Controller})
		}
	}
	for _, sv := range statics {
		raw := strings.TrimSpace(sv.Params["MayPlayAltManaCost"])
		if raw == "" || strings.TrimSpace(sv.Params["MayPlay"]) != "True" || mayPlayGateRejected(sv.Params) {
			continue
		}
		// Condition$ PlayerTurn ("during each of your turns"): the static's
		// controller's turn, the same switch mayPlayStatic runs. Any other
		// value is an unimplemented gate and fails closed.
		switch cond := strings.TrimSpace(sv.Params["Condition"]); cond {
		case "":
		case "PlayerTurn":
			if e.G.Active != sv.Controller {
				continue
			}
		default:
			continue
		}
		if ip := strings.TrimSpace(sv.Params["IsPresent"]); ip != "" && !e.mayPlayIsPresent(ip, sv.Controller, sv.Source) {
			continue
		}
		spec := strings.TrimSpace(sv.Params["Affected"])
		if spec == "" {
			continue
		}
		if az := strings.TrimSpace(sv.Params["AffectedZone"]); az != "" {
			inZone := false
			for _, part := range strings.Split(az, ",") {
				if z, known := effects.ZoneFromString(strings.TrimSpace(part)); known && z == o.Zone {
					inZone = true
					break
				}
			}
			if !inZone {
				continue
			}
		}
		if !effects.MatchesSpecFrom(e.G, spec, id, sv.Controller, sv.Source) {
			continue
		}
		if rawLimit := strings.TrimSpace(sv.Params["MayPlayLimit"]); rawLimit != "" {
			if n, err := strconv.Atoi(rawLimit); err == nil && n > 0 && e.mayPlayLimitReached(id, n) {
				continue
			}
		}
		alt := ParseCost(raw)
		if len(alt.Unknown) > 0 {
			// An unpriceable alternative (dynamic PayLife<ConvertedManaCost>,
			// Waterbend<ConvertedManaCost>) is withheld, never offered at a
			// wrong price.
			continue
		}
		out = append(out, alt)
	}
	return out
}
