package kgen

import (
	"fmt"
	"io"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// gainsAbilitiesOf lowers a Continuous static's GainsAbilitiesOf$ (CR
// 613.1f, "has all activated abilities of ..."): the affected object, the
// static's own source (AffectedDefined$ Self arrives as Card.Self) or each
// permanent its Affected$ filter matches (Hazel's Brewmaster's Foods; the
// filter is the static's Filter), may activate each activated ability of every battlefield permanent the filter
// matches at that moment (gorge layers.go gainedFacesForSpec,
// legal_granted.go grantedAbilitiesLimited). The filter is read relative to
// the source (Other excludes it) and lands in kop.Static.GainsOf; the zone
// rides kop.Static.Zone (ZoneNone: the battlefield, Forge's default;
// ZoneGraveyard: GainsAbilitiesOfZones$ Graveyard, every seat's graveyard;
// ZoneExile: GainsAbilitiesOfZones$ Exile, every seat's exile zone). The
// kernel's offer walk is dirac/static_gainsof.go. Other zones are refused by
// name. An untyped source (Territory Forge's Card.ExiledWithSource) lowers:
// the walk matches any zone card and prices both its non-mana abilities
// (static_gainsof.go gainedOffer) and its mana abilities
// (mana_gainedgrants.go), and prep's StallGainsOf pair refuses the matchup
// when a lender actually carries an ability the walk does not model. An
// artifact or land source lowers like any other (Mirran Safehouse's
// graveyard lands, Trazyn's artifacts; the animategainsof lockstep pool
// plays Safehouse's lent mana abilities beside real lands).
func (lc *lowerCtx) gainsAbilitiesOf(sp *params, st *lStatic, _ string) error {
	spec, ok := sp.get("GainsAbilitiesOf")
	if !ok {
		return nil
	}
	z, hasZone := sp.get("GainsAbilitiesOfZones")
	grave := hasZone && z == "Graveyard"
	exile := hasZone && z == "Exile"
	offField := hasZone && z != "Battlefield" && !grave && !exile
	f, err := lc.filter(spec)
	if err != nil {
		// A zone this lowering does not walk (neither the battlefield, the
		// graveyards nor exile) outranks what its filter reads.
		if offField {
			return refuse("GainsAbilitiesOfZones$ %q", z)
		}
		return err
	}
	if f.Players != 0 || len(f.Clauses) == 0 {
		return refuse("GainsAbilitiesOf$ %q: a player alternative", spec)
	}
	if offField {
		return refuse("GainsAbilitiesOfZones$ %q", z)
	}
	st.GainsOf = f
	switch {
	case grave:
		st.Zone = zGraveyard
	case exile:
		st.Zone = zExile
	}
	return nil
}

// continuousFilterPair builds a Continuous static's Landwalk (battlefield)
// and GainsOf filter fields (cardBuilder.static's filter closure). GainsOf is
// read in the zone GainsAbilitiesOfZones$ names, which also lands in
// kop.Static.Zone (ZoneNone, the battlefield, unless the graveyard or exile).
func continuousFilterPair(filter func(*lFilter, kop.Zone) (kop.FilterID, error), s *lStatic, st *kop.Static) (kop.FilterID, kop.FilterID, error) {
	lw, err := filter(s.Landwalk, kop.ZoneBattlefield)
	if err != nil {
		return 0, 0, err
	}
	gz := kop.ZoneBattlefield
	if s.GainsOf != nil && s.Zone == zGraveyard {
		gz, st.Zone = kop.ZoneGraveyard, kop.ZoneGraveyard
	}
	if s.GainsOf != nil && s.Zone == zExile {
		gz, st.Zone = kop.ZoneExile, kop.ZoneExile
	}
	g, err := filter(s.GainsOf, gz)
	return lw, g, err
}

// dumpStaticFilters prints a static's Landwalk and GainsOf filters in the IR
// dump (dumpLIR).
func dumpStaticFilters(w io.Writer, s *lStatic) {
	if s.Landwalk != nil {
		fmt.Fprintf(w, " landwalk=%s", s.Landwalk)
	}
	if s.GainsOf != nil {
		fmt.Fprintf(w, " gainsof=%s", s.GainsOf)
	}
}

// staticFilterLabels are the OPS.md labels of a table static's filter fields
// (opsUsage).
func staticFilterLabels(st *kop.Static) []string {
	var out []string
	for _, f := range []struct {
		id    kop.FilterID
		label string
	}{{st.ProtFrom, "st:ProtFrom"}, {st.ProtExc, "st:ProtExc"}, {st.Landwalk, "st:Landwalk"}, {st.GainsOf, "st:GainsOf"}} {
		if f.id != 0 {
			out = append(out, f.label)
		}
	}
	return out
}
