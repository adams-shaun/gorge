package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Manifest-family (Manifest, ManifestDread, Cloak) shape check: the ONE
// home of the loud decisions that depend only on the ability's printed shape.
// The primitives call it before moving anything, and
// engine/rules/manifest_family_census_test.go runs every corpus carrier
// through it, so the census and the primitives cannot disagree about which
// shapes are implemented. An unresolvable Amount$ body is the one loud
// decision it cannot make (it needs the resolution's context); the
// primitives keep that runtime check.

// manifestDefinedKind classifies a Manifest Defined$ text.
type manifestDefinedKind uint8

const (
	manifestDefinedNone manifestDefinedKind = iota // no Defined$: the top-card shape
	manifestDefinedRemembered
	manifestDefinedZone // ValidLibrary .../ValidHand ...: definedSpec resolves it
	manifestDefinedOther
)

func classifyManifestDefined(sa *cards.SA) manifestDefinedKind {
	ref := DefinedRefOf(sa)
	if !ref.Set() {
		return manifestDefinedNone
	}
	spec := strings.TrimSpace(ref.Text)
	switch {
	case spec == "Remembered":
		return manifestDefinedRemembered
	case strings.HasPrefix(spec, "ValidLibrary "), strings.HasPrefix(spec, "ValidHand "):
		return manifestDefinedZone
	}
	return manifestDefinedOther
}

// ManifestFamilyShapeSupported reports whether the Manifest, ManifestDread or
// Cloak ability sa has a shape its primitive implements. declared says
// whether an SVar name is declared: the primitive also honours a runtime write
// (StoreSVar), the census only the declaring face's table.
func ManifestFamilyShapeSupported(api string, sa *cards.SA, declared func(string) bool) bool {
	switch api {
	case "Manifest":
		kind := classifyManifestDefined(sa)
		if kind == manifestDefinedOther {
			return false
		}
		if sa.ParamStr(cards.PKChoices) != "" {
			// The hand chooser; a Defined$ beside it has no corpus carrier.
			return kind == manifestDefinedNone && faceDownChoiceZoneSupported(sa) &&
				faceDownAmountDeclared(sa, declared)
		}
		// The object forms ignore Amount$ (Forge's getTargetCards).
		return kind != manifestDefinedNone || faceDownAmountDeclared(sa, declared)
	case "ManifestDread":
		return strings.TrimSpace(sa.ParamStr(cards.PKChoices)) == ""
	case "Cloak":
		if strings.TrimSpace(sa.ParamStr(cards.PKChoices)) != "" {
			if !faceDownChoiceZoneSupported(sa) {
				return false
			}
		} else if !definedPlayerRef(sa).Set() {
			spec := strings.TrimSpace(DefinedRefOf(sa).Text)
			if effCloakCodes.Code(spec) == 0 && !strings.HasPrefix(spec, "ValidLibrary ") {
				return false
			}
		}
		return faceDownAmountDeclared(sa, declared)
	}
	return false
}

// faceDownAmountDeclared is false for a literal X or Y Amount$ naming an SVar
// that is not declared: with no declaration the cast-X-less count would
// silently be 0 or the wrong number.
func faceDownAmountDeclared(sa *cards.SA, declared func(string) bool) bool {
	raw, present := sa.Param(cards.PKAmount)
	if !present {
		return true
	}
	raw = strings.TrimSpace(raw)
	return (raw != "X" && raw != "Y") || declared(raw)
}

// faceDownChoiceZoneSupported is true for the hand chooser: no ChoiceZone$, or
// exactly Hand (chooseFromZoneForFaceDown's one pool).
func faceDownChoiceZoneSupported(sa *cards.SA) bool {
	if !sa.HasParam(cards.PKChoiceZone) {
		return true
	}
	zones := choiceZones(sa)
	return len(zones) == 1 && zones[state.ZHand]
}

// manifestParams is the Manifest ability's parameters, compiled once.
type manifestParams struct {
	Kind     manifestDefinedKind
	Defined  string
	Choices  bool
	Remember bool
	Amount   ParamText
}

func compileManifest(sa *cards.SA) manifestParams {
	raw, present := sa.Param(cards.PKAmount)
	// Shuffle$ True is the pile shuffle (cr-g150's Cloak ruling): no library
	// shuffle and no event. Read so the parameter census sees it read.
	_ = sa.ParamStr(cards.PKShuffle)
	return manifestParams{
		Kind:     classifyManifestDefined(sa),
		Defined:  strings.TrimSpace(DefinedRefOf(sa).Text),
		Choices:  strings.TrimSpace(sa.ParamStr(cards.PKChoices)) != "",
		Remember: isTrue(sa.ParamStr(cards.PKRememberManifested)),
		Amount:   ParamText{Text: raw, Present: present},
	}
}
