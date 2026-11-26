package kgen

import (
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A Continuous static whose layer-4 part is an override (AddAllCreatureTypes$,
// or AddType$ naming a supertype) and whose AffectedZone$ reaches cards off the
// battlefield: Maskwood Nexus (All) and Rootpath Purifier (Battlefield,Library).
// Gorge's layer walk applies the grant in every zone the static names, but its
// type and subtype filters read the layer-4 table, built from battlefield
// objects only (engine/rules/layer4types.go buildDerivedTypesWalk): a search, a
// cost reduction or a trigger reads a card in any other zone as printed.
//
// The static lowers as two: the plain battlefield StatOverride (allZoneTwin),
// and a StatOffZoneGrant carrying the same Override record (Static.Override),
// which only the off-battlefield projection reads (static_offzonechars.go).

// offZoneWithBFFlag marks a multi-zone grant whose battlefield half rides the
// plain static (allZoneTwin); it never reaches the build.
const offZoneWithBFFlag = "OffZoneWithBF"

// Refusal texts of the AffectedZone$ override shapes this lowering names and
// keeps refused.
const (
	refuseOffZoneChosenRemove = "off-zone AddType ChosenType with RemoveCreatureTypes"
	refuseOffZoneColour       = "off-zone AddColor (no off-battlefield colour reader)"
	refuseOffZoneStackType    = "off-zone AddType on the stack"
)

// offZoneOverrideShape accepts exactly the two off-zone override shapes: az All
// (zone ZoneAny; the battlefield half is the twin) or Battlefield with Library
// (zone ZoneLibrary, withBF), over the keys Mode, Affected, AffectedZone,
// AddAllCreatureTypes (True), AddType (supertype words only) and Description.
func offZoneOverrideShape(sp *params, az string) (z kop.Zone, withBF, ok bool) {
	if sp.is("MayPlay", "True") {
		return 0, false, false
	}
	switch strings.Join(strings.Fields(strings.ReplaceAll(az, ",", " ")), ",") {
	case "All":
		z = kop.ZoneAny
	case "Battlefield,Library", "Library,Battlefield":
		z, withBF = kop.ZoneLibrary, true
	default:
		return 0, false, false
	}
	_, allTypes := sp.get("AddAllCreatureTypes")
	t, addType := sp.get("AddType")
	if !allTypes && !addType {
		return 0, false, false
	}
	for k := range sp.m {
		switch k {
		case "Mode", "Affected", "AffectedZone", "Description":
		case "AddAllCreatureTypes":
			if !sp.is(k, "True") {
				return 0, false, false
			}
		case "AddType":
			for _, w := range strings.Split(t, "&") {
				if b, isType := typeAddBit(strings.TrimSpace(w)); !isType || b&tSupertype == 0 || b&tAnyCard != 0 {
					return 0, false, false
				}
			}
		default:
			return 0, false, false
		}
	}
	return z, withBF, true
}

// offZoneOverrideRefusal names the refusal of an off-zone override shape this
// slice leaves unlowered ("" for any other static).
func offZoneOverrideRefusal(sp *params, az string) string {
	if sp.is("MayPlay", "True") {
		return ""
	}
	switch {
	case az == "All" && sp.is("AddType", "ChosenType") && sp.is("RemoveCreatureTypes", "True"):
		return refuseOffZoneChosenRemove
	case az == "All" && sp.str("AddColor") != "":
		return refuseOffZoneColour
	case strings.Contains(az, "Stack") && strings.Contains(az, "Battlefield") && sp.str("AddType") != "":
		return refuseOffZoneStackType
	}
	return ""
}

// offZoneOverride is affectedZone's arm for the shapes above. took reports the
// static was claimed: it carries the zone and the off-zone flag and continuous'
// tail lowers it as an override.
func offZoneOverride(sp *params, st *lStatic, az string) (took bool, err error) {
	if r := offZoneOverrideRefusal(sp, az); r != "" {
		return true, refuse("%s", r)
	}
	z, withBF, ok := offZoneOverrideShape(sp, az)
	if !ok {
		return false, nil
	}
	st.Zone = int(z)
	st.Flags = append(st.Flags, offZoneFlag)
	if withBF {
		st.Flags = append(st.Flags, offZoneWithBFFlag)
	}
	sp.consume("AffectedZone")
	return true, nil
}

// splitOffZone is the battlefield and the off-zone halves of a static flagged
// offZoneFlag.
func splitOffZone(s lStatic) (bf, off lStatic) {
	drop := func(f string) bool { return f == offZoneFlag || f == offZoneWithBFFlag }
	bf = s
	bf.Zone = 0
	bf.Flags = slices.DeleteFunc(slices.Clone(s.Flags), drop)
	off = s
	off.Flags = slices.DeleteFunc(slices.Clone(s.Flags), func(f string) bool { return f == offZoneWithBFFlag })
	return bf, off
}

// twinsOffZone reports whether s is an off-zone grant with a battlefield half
// of its own: AffectedZone$ All, or a multi-zone static naming the battlefield.
func twinsOffZone(s lStatic) bool {
	return slices.Contains(s.Flags, offZoneFlag) && (s.Zone == int(kop.ZoneAny) || slices.Contains(s.Flags, offZoneWithBFFlag))
}
