package kgen

import (
	"fmt"
	"io"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/effects"
)

// gainsAbilitiesOf lowers a Continuous static's GainsAbilitiesOf$ (CR
// 613.1f, "has all activated abilities of ..."): the affected object, the
// static's own source (AffectedDefined$ Self arrives as Card.Self), may
// activate each activated ability of every battlefield permanent the filter
// matches at that moment (gorge layers.go gainedFacesForSpec,
// legal_granted.go grantedAbilitiesLimited). The filter is read relative to
// the source (Other excludes it) and lands in kop.Static.GainsOf; the zone
// rides kop.Static.Zone (ZoneNone: the battlefield, Forge's default;
// ZoneGraveyard: GainsAbilitiesOfZones$ Graveyard, every seat's graveyard). The
// kernel's offer walk is dirac/static_gainsof.go. Other zones, another
// recipient, and artifact or land sources (whose mana abilities the walk
// does not model) are refused by name.
func (lc *lowerCtx) gainsAbilitiesOf(sp *params, st *lStatic, aff string) error {
	spec, ok := sp.get("GainsAbilitiesOf")
	if !ok {
		return nil
	}
	z, hasZone := sp.get("GainsAbilitiesOfZones")
	grave := hasZone && z == "Graveyard"
	offField := hasZone && z != "Battlefield" && !grave
	f, err := lc.filter(spec)
	if err != nil {
		// A zone this lowering does not walk outranks what its filter reads:
		// Myr Welder's Card.IsImprinted is an Exile filter, so the card is
		// held back by its zone, not by the Imprinted reader.
		if offField {
			return refuse("GainsAbilitiesOfZones$ %q", z)
		}
		return err
	}
	if f.Players != 0 || len(f.Clauses) == 0 {
		return refuse("GainsAbilitiesOf$ %q: artifact or land sources", spec)
	}
	for _, c := range f.Clauses {
		// a clause with no card type must name a creature type (Elf): a
		// subtype-only "Elf" cannot match an artifact or a land, and a
		// creature's mana ability stalls the walk (StallGainsOf) anyway
		creatSub := c.Types == 0 && c.Subtype != "" && effects.CreatureTypeWords(c.Subtype)
		if c.Types&(tArtifact|tLand) != 0 || c.Types == 0 && !creatSub {
			return refuse("GainsAbilitiesOf$ %q: artifact or land sources", spec)
		}
	}
	if offField {
		return refuse("GainsAbilitiesOfZones$ %q", z)
	}
	if aff != "Card.Self" {
		return refuse("GainsAbilitiesOf Affected %q", aff)
	}
	st.GainsOf = f
	if grave {
		st.Zone = zGraveyard
	}
	return nil
}

// continuousFilterPair builds a Continuous static's Landwalk (battlefield)
// and GainsOf filter fields (cardBuilder.static's filter closure). GainsOf is
// read in the zone GainsAbilitiesOfZones$ names, which also lands in
// kop.Static.Zone (ZoneNone, the battlefield, unless the graveyard).
func continuousFilterPair(filter func(*lFilter, kop.Zone) (kop.FilterID, error), s *lStatic, st *kop.Static) (kop.FilterID, kop.FilterID, error) {
	lw, err := filter(s.Landwalk, kop.ZoneBattlefield)
	if err != nil {
		return 0, 0, err
	}
	gz := kop.ZoneBattlefield
	if s.GainsOf != nil && s.Zone == zGraveyard {
		gz, st.Zone = kop.ZoneGraveyard, kop.ZoneGraveyard
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
