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

	"github.com/adams-shaun/gorge/decision"
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
// (RaiseCost$, MayPlayAltManaCost$) -- and an uncharged surcharge is exactly
// the widening this file refuses: an offer the engine prices incorrectly must
// not exist at all. Any of these present fails the static closed, so the
// card is simply not offered.
var mayPlayUnreadGates = [...]string{
	"ValidAfterStack", "SVarCompare", "CheckSecondSVar", "CheckThirdSVar",
	"PresentCompare", "ValidSA", "ActivationZone", "CharacteristicDefining",
	"RaiseCost", "MayPlayAltManaCost",
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
		strings.TrimSpace(params["RaiseCost"]) != "" ||
		strings.TrimSpace(params["MayPlayAltManaCost"]) != "" {
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
	if ip := strings.TrimSpace(params["IsPresent"]); ip != "" && !e.isPresent(ip, you, source) {
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
func (e *Engine) isPresent(spec string, you state.PlayerID, source state.ObjID) bool {
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

// mayPlayGrantOffers walks the acting player's graveyard, exile and library
// and adds a play/cast option for every card a MayPlay$ static grants. The
// library walk is the TOP CARD ONLY: a card deeper in the library can never
// be played, every library grant in the corpus scopes its Affected$ with the
// TopLibrary predicate (Ka-Zar of the Savage Land, Korlessa Scale Singer,
// Oracle of Mul Daya, and the Card.Self+TopLibrary self-grants), and walking
// only the top card keeps a hidden library from leaking deeper card
// identities into the chooser's own option list -- a MayPlay grant exposes
// exactly the one card it lets you play, which playing it reveals anyway.
// The graveyard/exile walk order and the zone order are deterministic
// slices. The returned options carry Index 0 -- the caller appends them to
// its list and reindexes (the same discipline the hand walk's add closure
// practises).
func (e *Engine) mayPlayGrantOffers(p state.PlayerID, sorcery bool) []decision.Option {
	var out []decision.Option
	offer := func(id state.ObjID) {
		o := e.G.Obj(id)
		if o == nil {
			return
		}
		f := o.Face()
		if f == nil {
			return
		}
		free, ok := e.mayPlayGrant(p, id)
		if !ok {
			return
		}
		if f.IsLand() {
			// A granted land play consumes the ordinary land drop
			// (CR 118.3a: the permission is not an additional drop)
			// and the ordinary sorcery timing. handlePriority's
			// play_land path commits it from the card's current zone.
			if !sorcery || e.G.Players[p].LandsPlayed >= 1 {
				return
			}
			out = append(out, decision.Option{Index: 0, Kind: "play_land",
				Label: "Play " + f.Name, Obj: id})
			return
		}
		if e.castRestricted(p, id) || e.castSuppressed(p, id) {
			return
		}
		instantSpeed := f.IsInstant() || e.HasKeyword(id, "Flash")
		if !instantSpeed && !sorcery {
			return
		}
		if !e.castTargetsAvailable(p, id, f.SpellAbility()) {
			return
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
	for _, zn := range []state.Zone{state.ZGraveyard, state.ZExile} {
		for _, id := range e.G.Zone(zn, p) {
			offer(id)
		}
	}
	if lib := e.G.Zone(state.ZLibrary, p); len(lib) > 0 {
		offer(lib[0])
	}
	return out
}
