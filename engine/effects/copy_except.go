package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// copyExceptSpec is one copy effect's characteristic exceptions as its
// parameters state them (CR 707.9a/b). copyExceptions settles it against the
// copy object into the FINAL values a CopyExcept event carries, which the
// fold writes onto the copy's layer-1 basis face: the exceptions are then
// copiable values, so a copy of the copy inherits them (CR 707.3).
type copyExceptSpec struct {
	// The type exceptions, applied in the order engine/rules/chars'
	// applyTypeEffect applies one effect's strips: removeCardTypes keeps only
	// the supertypes; removeCreatureTypes drops the creature subtypes;
	// removeLegendary drops Legendary; then addTypes append once each.
	addTypes                                              []string
	removeCardTypes, removeCreatureTypes, removeLegendary bool
	// setPower/setToughness, each when present, replace that half of the P/T.
	setPower, setToughness               int32
	setPowerPresent, setToughnessPresent bool
	// keywords are the AddKeywords$ lines (CR 707.9a).
	keywords []string
}

// copyExceptions settles spec against o, the object that has just become the
// copy (its Face() is the copied basis), into the CopyExcept payload. A
// characteristic no exception names is left unset, so the fold keeps the
// copied value.
func copyExceptions(o *state.Object, s copyExceptSpec) events.CopyExceptions {
	var x events.CopyExceptions
	f := o.Face()
	if f == nil {
		return x
	}
	if s.removeCardTypes || s.removeCreatureTypes || s.removeLegendary || len(s.addTypes) > 0 {
		x.TypesSet = true
		for _, t := range f.Types {
			switch {
			case s.removeCardTypes && !copySupertype(t):
			case s.removeCreatureTypes && CreatureTypeWords(t):
			case s.removeLegendary && strings.EqualFold(t, "Legendary"):
			default:
				x.Types = append(x.Types, t)
			}
		}
		for _, add := range s.addTypes {
			if !copyHasType(x.Types, add) {
				x.Types = append(x.Types, add)
			}
		}
	}
	if s.setPowerPresent || s.setToughnessPresent {
		x.Power, x.Toughness, x.PTSet = int32(f.Power()), int32(f.Toughness()), true
		if s.setPowerPresent {
			x.Power = s.setPower
		}
		if s.setToughnessPresent {
			x.Toughness = s.setToughness
		}
	}
	x.Keywords = append([]string(nil), s.keywords...)
	return x
}

func copyHasType(types []string, t string) bool {
	for _, have := range types {
		if strings.EqualFold(have, t) {
			return true
		}
	}
	return false
}

// copySupertypes is the CR 205.4a supertype vocabulary, the list
// engine/rules/chars.SupertypeWords carries (TestCopySupertypesMatchChars
// holds the two equal).
var copySupertypes = [...]string{"Basic", "Legendary", "Ongoing", "Snow", "World"}

// CopySupertypes exposes copySupertypes to the vocabulary guard test.
func CopySupertypes() []string { return copySupertypes[:] }

func copySupertype(t string) bool {
	for _, w := range copySupertypes {
		if strings.EqualFold(w, t) {
			return true
		}
	}
	return false
}
