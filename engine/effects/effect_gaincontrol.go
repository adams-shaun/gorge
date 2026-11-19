package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// effect_gaincontrol.go is the api:Effect-delivered control static
// (StaticAbilities$ naming a Mode$ Continuous body that carries GainControl$):
// Garland, Royal Kidnapper's "gain control of target creature that player
// controls for as long as they're the monarch" and Shield Broker's
// shield-counter steal. The printed S: route's twin is realized entirely in
// rules (engine/rules/control_static.go's staticControlWants over the static
// scan); an Effect-delivered one is a REGISTRATION first -- this file builds
// the ContinuousEffect that registration writes, and rules' reconcile derives
// its wanted transfer from the live registration, re-evaluating the body's
// CheckSVar$/SVarCompare$ intervening-if on every derivation so "for as long
// as ..." holds exactly while the condition does.

// gainControlStaticKeysReadable is the delivered line's whitelist: Mode$
// Continuous with a GainControl$ value, and NO parameter outside the keys
// this delivery reads (Affected$ plus the gate pair and the display
// Description$). A body carrying any other key is a grant shape this build
// does not evaluate and fails closed to the caller's unimplemented Note --
// the shipped-statics convention the EffEffect whitelist arms keep. The
// gate body itself is not validated here: rules' live evaluation fails
// closed on an unreadable count head, withholding the transfer rather than
// silently always-applying it.
func gainControlStaticKeysReadable(params map[string]string) bool {
	if !staticKeysReadable(params,
		"Mode", "Affected", "GainControl", "CheckSVar", "SVarCompare", "Description") {
		return false
	}
	return strings.TrimSpace(params["Mode"]) != "" &&
		strings.EqualFold(strings.TrimSpace(params["Mode"]), "Continuous") &&
		strings.TrimSpace(params["GainControl"]) != ""
}

// effectImprintedObjects resolves the api:Effect's ImprintCards$ into the
// object ids the grant remembers as its own capture (the Affected$
// Card.IsImprinted spec names them; this build's IsImprinted predicate is
// deliberately exile-scoped and reads a SOURCE object's association, and the
// Effect has no object of its own, so the delivered grant registers the set
// directly and its Affected$ clause reads as IsRemembered -- the same
// normalization the Effect's permanent-comeback trigger registration makes
// for the predicate's exile rule). The member vocabulary is
// effectRemembered's: Targeted (the resolution's chosen targets, exactly the
// read RememberObjects$ Targeted makes) and Self (the Effect's source). An
// unrecognised member is an unreadable capture and fails closed: the caller
// registers nothing and keeps its Note.
func effectImprintedObjects(h Host, c *Ctx, sa *cards.SA, ep *EffectParams) ([]state.ObjID, bool) {
	if ep.ImprintCards == "" {
		return nil, true
	}
	var out []state.ObjID
	for _, member := range strings.Split(ep.ImprintCards, "&") {
		switch effectRememberedCodes.Code(strings.TrimSpace(member)) {
		case effectRememberedTargeted:
			targets := c.Targets
			if c.PickedTargets != nil {
				targets = c.PickedTargets
			} else if len(targets) == 0 || !TargetsOf(sa).Targeted() {
				// Forge's Targeted is the union of every target choice down
				// the chain, the same read effectRemembered's Targeted member
				// makes (the Effect link may have inherited its targets from
				// an earlier link in the chain).
				targets = refTargetUnion(c)
			}
			out = appendEffectRememberedObjects(h, out, targetsStillChosen(h.Game(), c, targets))
		case effectRememberedYou:
			out = append(out, c.Source)
		default:
			return nil, false
		}
	}
	return out, true
}

// effectGainControlDelivered registers one Effect-delivered control static
// (Garland, Royal Kidnapper's "gain control of target creature that player
// controls for as long as they're the monarch", Shield Broker's
// shield-counter steal): the StaticAbilities$ body carries GainControl$, a
// rules-mod like MayPlay that changes no characteristic, so it is registered
// into the continuous registry as a GainControl-carrying grant the same
// shape the printed S: scan emits (engine/rules/control_static.go's
// staticControlWants) -- but read from the live REGISTRATION, with the
// body's CheckSVar$/SVarCompare$ intervening-if re-evaluated on EVERY
// derivation against the grant's own SVar table and remembered players,
// because a delivered static has no printed scan to re-run the gate for it.
// The grant's memory is the union of the Effect's own capture (Shield
// Broker's RememberObjects$ Targeted) and its ImprintCards$ set (Garland's
// Targeted), so the Affected$ IsImprinted clause (normalized to IsRemembered:
// the predicate reads a source OBJECT's exile-scoped association and the
// Effect has no object of its own) matches exactly what the resolution
// captured. The line must be entirely readable
// (gainControlStaticKeysReadable) or nothing registers and effEffect keeps
// its honest unimplemented Note rather than a half-read transfer. Lifetime
// is the Effect's own, the same shape every registration in effEffect
// writes: Duration$ Permanent with FromEffect keeps the grant alive past its
// source (AddContinuous's CR 611.2a rule) and the ForgetOnMoved$/
// ForgetCounter$ ends are its own expiry. effEffect calls this whole arm
// (the registration included) because effEffect is at its codeshape ceiling.
func effectGainControlDelivered(h Host, c *Ctx, sa *cards.SA, ep *EffectParams,
	params map[string]string, remembered []state.ObjID, effectName, rawDur, dur,
	forgetOn, exileOn, forgetCounter string, imprintOnHost bool) bool {
	if !gainControlStaticKeysReadable(params) {
		return false
	}
	ce, ok := effectGainControlGrant(h, c, sa, ep, params, remembered)
	if !ok {
		return false
	}
	ce.UntilEOT = effectUntilEOT(h, c.Source, rawDur)
	ce.Duration = dur
	ce.Permanent = strings.EqualFold(strings.TrimSpace(rawDur), "Permanent")
	ce.ForgetOnMoved = forgetOn
	ce.ExileOnMoved = exileOn
	ce.ForgetCounter = forgetCounter
	ce.ImprintOnHost = imprintOnHost
	ce.Name = effectName
	effectContinuous(h, ce)
	return true
}

// effectGainControlGrant builds the ContinuousEffect one Effect-delivered
// control static registers (mode/params as parseStaticLine split them, the
// api:Effect's compiled parameters, the capture effectRemembered already
// resolved). The remembered PLAYERS ride alongside (Garland's
// RememberObjects$ TriggeredPlayer: the monarch the gate's
// PlayerCountRememberedPlayer body counts); SVars is the RESOLVING face's
// table (the gate's named SVar resolves against the face that wrote it,
// even after the source moved), and the gate text is carried for rules'
// live evaluation. ok=false is the fail-closed grant: nothing registers and
// the caller keeps its Note. Lifetime fields are the caller's.
func effectGainControlGrant(h Host, c *Ctx, sa *cards.SA, ep *EffectParams,
	params map[string]string, remembered []state.ObjID) (state.ContinuousEffect, bool) {
	imprinted, ok := effectImprintedObjects(h, c, sa, ep)
	if !ok {
		return state.ContinuousEffect{}, false
	}
	affects := strings.TrimSpace(params["Affected"])
	// Forge omits Affected$ on a self-only static; its default is the host
	// card, the printed scan's default.
	if affects == "" {
		affects = "Card.Self"
	}
	// The delivered grant names its capture through IsRemembered (see
	// effectImprintedObjects): translate the IsImprinted clause, leave every
	// other qualifier verbatim.
	affects = strings.ReplaceAll(affects, "IsImprinted", "IsRemembered")
	remembered = append([]state.ObjID(nil), remembered...)
	remembered = append(remembered, imprinted...)
	ce := state.ContinuousEffect{
		Source:                 c.Source,
		Controller:             c.Controller,
		Affects:                affects,
		GainControl:            strings.TrimSpace(params["GainControl"]),
		GainControlCheckSVar:   strings.TrimSpace(params["CheckSVar"]),
		GainControlSVarCompare: strings.TrimSpace(params["SVarCompare"]),
		SVars:                  c.SVars,
		Remembered:             remembered,
		RememberedPlayers:      effectRememberedPlayers(h, c, sa),
	}
	return ce, true
}
