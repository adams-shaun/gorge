package rules

import (
	"slices"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Effect-delivered may-play grant's two riders the printed route already
// reads: MayPlayAltManaCost$ (Inside Information, Hama, the Bloodbender --
// state.ContinuousEffect.MayPlayAltCost) and a grant over the top card of the
// library (The Belligerent, Gwenom, Remorseless -- AffectedZone$ Library on a
// TopLibrary Affected$). Forge reads both off any Mode$ Continuous static
// whatever its host (StaticAbilityContinuous), so an Effect's static behaves
// as a printed one.

// mayPlayEffectReaches reports whether an Effect grant can name card o for p
// at all: a graveyard or exile card, or p's own library TOP card -- never a
// deeper library card, so the hidden library leaks no identity into an
// option list, and never the hand (the ordinary walk's).
func (e *Engine) mayPlayEffectReaches(p state.PlayerID, o *state.Object) bool {
	switch o.Zone {
	case state.ZGraveyard, state.ZExile:
		return true
	case state.ZLibrary:
		lib := e.G.Zone(state.ZLibrary, p)
		return len(lib) > 0 && lib[0] == o.ID && o.Controller == p
	}
	return false
}

// mayPlayEffectCoversCard is the per-grant gate chain every Effect arm
// shares once the card's zone is reachable: controller (or CardOwner), the
// Condition$ PlayerTurn rider, the MayPlayLimit$ cap, the parsed
// AffectedZone$ and the Affects spec with the delivering effect's Remembered
// set loaded. A library card is covered only by a grant that NAMES the
// library; the no-zone fallback means graveyard and exile only.
func (e *Engine) mayPlayEffectCoversCard(ce *state.ContinuousEffect, p state.PlayerID, o *state.Object, limited *lazyMayPlays) bool {
	if !ce.MayPlay || !mayPlayEffectBenefits(ce, o, p) {
		return false
	}
	if ce.MayPlayPlayerTurn && e.G.Active != p {
		return false
	}
	if !e.mayPlayCapOpen(ce, limited) {
		return false
	}
	zones, all, ok := effects.ParseZones(ce.AffectedZone)
	if !ok && !all {
		return false
	}
	if o.Zone == state.ZLibrary {
		if !slices.Contains(zones, state.ZLibrary) {
			return false
		}
	} else if !all && !slices.Contains(zones, o.Zone) {
		return false
	}
	return e.effectGrantMatches(ce, o.ID)
}

// mayPlayEffectLibraryTop returns p's library top card when the Effect grant
// ce offers it and the walk's card test keep accepts it (the lands walk keeps
// lands, the spells walk non-lands). The caller has already run the grant's
// seat / PlayerTurn / limit gates.
func (e *Engine) mayPlayEffectLibraryTop(ce *state.ContinuousEffect, p state.PlayerID, keep func(*state.Object) bool) (state.ObjID, bool) {
	zones, _, _ := effects.ParseZones(ce.AffectedZone)
	if !slices.Contains(zones, state.ZLibrary) {
		return 0, false
	}
	lib := e.G.Zone(state.ZLibrary, p)
	if len(lib) == 0 {
		return 0, false
	}
	o := e.G.Obj(lib[0])
	if o == nil || o.Controller != p || !keep(o) || !e.effectGrantMatches(ce, o.ID) {
		return 0, false
	}
	return o.ID, true
}

// mayPlayEffectAltCosts lists the alternative costs the active Effect-
// delivered grants covering card id name (MayPlayAltManaCost$), in e.active()
// order, priced by altCostParse -- the printed route's own pricing, so an
// unpriceable token is withheld rather than offered at a wrong price. Each
// opens its own permission (grants), exactly like a printed Valgavoth-shape
// static; mayPlayAltCosts appends them after the static entries, and the
// offer walk and beginCast both read the list through mayPlayCastAlts, so
// the AltCostIndex cannot drift between offer and charge.
func (e *Engine) mayPlayEffectAltCosts(p state.PlayerID, id state.ObjID) []mayPlayAltView {
	o := e.G.Obj(id)
	if o == nil || !e.mayPlayEffectReaches(p, o) {
		return nil
	}
	var out []mayPlayAltView
	limited := lazyMayPlays{e: e, p: p}
	ces := e.active()
	for i := range ces {
		ce := &ces[i]
		if ce.MayPlayAltCost == "" || !e.mayPlayEffectCoversCard(ce, p, o, &limited) {
			continue
		}
		if alt, ok := e.altCostParse(id, ce.MayPlayAltCost); ok {
			out = append(out, mayPlayAltView{cost: alt, grants: true})
		}
	}
	return out
}
