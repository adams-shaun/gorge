package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A targeted ChangeZone over Origin$ Graveyard,Exile (Sorceress's Schemes:
// "return target instant or sorcery card from your graveyard or exiled card
// with flashback you own to your hand"). gorge's changeZoneTargetZones reads
// the zones from the inZone<Z> word of each comma-split ValidTgts$ alternative,
// in alternative order, and each alternative matches only in its own zone.
// The kernel's one slot (kop.TgtGraveExileCard) holds the whole chain, each
// clause carrying its own Zone, and walks the graveyards before exile.

// graveExileTarget readies the lowering of a targeted Graveyard,Exile move: it
// refuses every shape but the lockstep-read one by name, then marks the
// context so objOrTargetOpt builds the two-zone slot. The move takes the
// object from whichever zone it is in (origin zAny at the caller).
func (lc *lowerCtx) graveExileTarget(ps *params) error {
	ps.get("Origin") // read
	for _, k := range []string{"ChangeType", "Hidden", "Optional", "Mandatory", "DefinedPlayer", "Chooser", "GainControl", "LibraryPosition", "TgtZone", "TargetType", "Defined"} {
		if _, ok := ps.get(k); ok {
			return refuse("ChangeZone Origin$ Graveyard,Exile with ValidTgts$ and %s$", k)
		}
	}
	if d := ps.str("Destination"); d != "Hand" {
		return refuse("ChangeZone Origin$ Graveyard,Exile with ValidTgts$ to %q (only Hand is read)", d)
	}
	lc.graveExileCard = true
	return nil
}

// graveExileSplit splits a ValidTgts$ over Origin$ Graveyard,Exile into its
// alternatives with the single inZone<Z> word of each removed, returning the
// re-joined text and each alternative's zone. It refuses an alternative with
// no zone word, two, or another zone, and an exile alternative before a
// graveyard one (the kernel walks the graveyards first).
func graveExileSplit(vt string) (string, []kop.Zone, error) {
	alts := strings.Split(vt, ",")
	zones := make([]kop.Zone, 0, len(alts))
	for i, alt := range alts {
		head, rest, _ := strings.Cut(alt, ".")
		var props []string
		z := kop.ZoneNone
		for _, p := range strings.Split(rest, "+") {
			zn, isZone := strings.CutPrefix(p, "inZone")
			if !isZone {
				if p != "" {
					props = append(props, p)
				}
				continue
			}
			if z != kop.ZoneNone {
				return "", nil, refuse("ChangeZone Origin$ Graveyard,Exile ValidTgts$ alternative %q names two zones", alt)
			}
			switch zn {
			case "Graveyard":
				z = kop.ZoneGraveyard
			case "Exile":
				z = kop.ZoneExile
			default:
				return "", nil, refuse("ChangeZone Origin$ Graveyard,Exile ValidTgts$ alternative %q names zone %q", alt, zn)
			}
		}
		if z == kop.ZoneNone {
			return "", nil, refuse("ChangeZone Origin$ Graveyard,Exile ValidTgts$ alternative %q has no inZone word", alt)
		}
		if z == kop.ZoneExile && i == 0 {
			return "", nil, refuse("ChangeZone Origin$ Graveyard,Exile ValidTgts$ names Exile first (the kernel walks the graveyards first)")
		}
		if z == kop.ZoneGraveyard && len(zones) > 0 && zones[len(zones)-1] == kop.ZoneExile {
			return "", nil, refuse("ChangeZone Origin$ Graveyard,Exile ValidTgts$ names a graveyard after Exile (the kernel walks the graveyards first)")
		}
		zones = append(zones, z)
		if len(props) > 0 {
			head += "." + strings.Join(props, "+")
		}
		alts[i] = head
	}
	return strings.Join(alts, ","), zones, nil
}
