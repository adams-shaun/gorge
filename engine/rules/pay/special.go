package pay

import "github.com/adams-shaun/gorge/engine/state"

// Special names the CR 116.2 special action a PurposeSpecial payment pays
// for: turn face up (CR 116.2b, split by the procedure Forge's SA properties
// tell apart), suspend (116.2f), foretell (116.2h), plot (116.2k) and unlock a
// door (116.2m). None is a spell or an activated ability, so a RestrictValid$
// "spend only to cast spells" cannot pay one and a Static.* restriction can.
type Special uint8

const (
	SpecialNone Special = iota
	SpecialMorphUp
	SpecialDisguiseUp
	SpecialManifestUp
	SpecialCloakUp
	SpecialUnlock
	SpecialForetell
	SpecialPlot
	SpecialSuspend
)

// SpecialOfCastMode is the special action a cast-flow mode (the decision
// option's Mode, pendingCast.mode) pays: foretell, plot and suspend ride the
// cast machinery but cast nothing.
func SpecialOfCastMode(mode string) Special {
	switch specialModeCodes.Code(mode) {
	case specialModeForetell:
		return SpecialForetell
	case specialModePlot:
		return SpecialPlot
	case specialModeSuspend:
		return SpecialSuspend
	}
	return SpecialNone
}

// SpecialOfScope is the special action a cost scope prices -- the one reading
// the CR 601.2f ValidSpell$ Static.* modifiers and the RestrictValid$ Static.*
// restrictions share, so the two cannot drift. Foretell has its own scope kind
// (which predates specialActionScope); plot and suspend ride a Spell scope
// under their cast mode; the rest are Kind "Static" modes.
func SpecialOfScope(scope CostScope) Special {
	switch scope.Kind {
	case "Foretell":
		return SpecialForetell
	case "Static":
		switch specialModeCodes.Code(scope.Mode) {
		case specialModeUnlock:
			return SpecialUnlock
		case specialModeMorphUp:
			return SpecialMorphUp
		case specialModeDisguiseUp:
			return SpecialDisguiseUp
		case specialModeManifestUp:
			return SpecialManifestUp
		case specialModeCloakUp:
			return SpecialCloakUp
		}
		return SpecialNone
	}
	return SpecialOfCastMode(scope.Mode)
}

type specialModeCode uint16

const (
	specialModeForetell specialModeCode = iota + 1
	specialModePlot
	specialModeSuspend
	specialModeUnlock
	specialModeMorphUp
	specialModeDisguiseUp
	specialModeManifestUp
	specialModeCloakUp
)

var specialModeCodes = state.NewStrCodes(
	state.StrEntry[specialModeCode]{Key: "foretell", Val: specialModeForetell},
	state.StrEntry[specialModeCode]{Key: "plot", Val: specialModePlot},
	state.StrEntry[specialModeCode]{Key: "suspend", Val: specialModeSuspend},
	state.StrEntry[specialModeCode]{Key: "unlock", Val: specialModeUnlock},
	state.StrEntry[specialModeCode]{Key: "morphup", Val: specialModeMorphUp},
	state.StrEntry[specialModeCode]{Key: "disguiseup", Val: specialModeDisguiseUp},
	state.StrEntry[specialModeCode]{Key: "manifestup", Val: specialModeManifestUp},
	state.StrEntry[specialModeCode]{Key: "cloakup", Val: specialModeCloakUp},
)

type specialPropertyCode uint16

const (
	specialPropertyForetelling specialPropertyCode = iota + 1
	specialPropertyPlotting
	specialPropertyUnlock
	specialPropertyMorphUp
	specialPropertyManifestUp
	specialPropertyCloakUp
	specialPropertyIsTurnFaceUp
)

var specialPropertyCodes = state.NewStrCodes(
	state.StrEntry[specialPropertyCode]{Key: "Foretelling", Val: specialPropertyForetelling},
	state.StrEntry[specialPropertyCode]{Key: "Plotting", Val: specialPropertyPlotting},
	state.StrEntry[specialPropertyCode]{Key: "Unlock", Val: specialPropertyUnlock},
	state.StrEntry[specialPropertyCode]{Key: "MorphUp", Val: specialPropertyMorphUp},
	state.StrEntry[specialPropertyCode]{Key: "ManifestUp", Val: specialPropertyManifestUp},
	state.StrEntry[specialPropertyCode]{Key: "CloakUp", Val: specialPropertyCloakUp},
	state.StrEntry[specialPropertyCode]{Key: "isTurnFaceUp", Val: specialPropertyIsTurnFaceUp},
)

// SpecialHasProperty evaluates one Forge SpellAbilityProperty token against
// the special action s (ValidSpell$ Static.<token> and RestrictValid$
// Static.<token> read the same set). known is false for a token that is not an
// SA property (a card filter such as Creature); MorphUp is Forge's isMorphUp
// (Morph and Megamorph), isTurnFaceUp the union of the four turn-up
// procedures.
func SpecialHasProperty(s Special, token string) (match, known bool) {
	switch specialPropertyCodes.Code(token) {
	case specialPropertyForetelling:
		return s == SpecialForetell, true
	case specialPropertyPlotting:
		return s == SpecialPlot, true
	case specialPropertyUnlock:
		return s == SpecialUnlock, true
	case specialPropertyMorphUp:
		return s == SpecialMorphUp, true
	case specialPropertyManifestUp:
		return s == SpecialManifestUp, true
	case specialPropertyCloakUp:
		return s == SpecialCloakUp, true
	case specialPropertyIsTurnFaceUp:
		return s == SpecialMorphUp || s == SpecialDisguiseUp || s == SpecialManifestUp || s == SpecialCloakUp, true
	}
	return false, false
}

// SpecialDescriptor is the descriptor of a payment for special action s on
// object id. The offer gate and the payment of each special action build it
// through this one constructor so the restriction class they read is the same.
func SpecialDescriptor(id state.ObjID, s Special, cost Cost) Descriptor {
	return Descriptor{ID: id, Class: PurposeSpecial, Special: s, Cost: &cost, XAnnounced: cost.X > 0}
}

// Priced is the descriptor template d (identity and class, no cost yet) priced
// at c, the X marker derived from the raw cost exactly as DescriptorFor does.
// The cost is read in place, never retained.
func (d Descriptor) Priced(c *Cost) Descriptor {
	d.Cost = c
	d.XAnnounced = c.X > 0
	return d
}

// staticRestrictionTail evaluates the +-joined tail of a RestrictValid$
// Static.<tail> term against the descriptor's special action: SA-property
// tokens test d.Special, and the remaining tokens come back joined as the card
// filter for the paid object. ok is false when d is not a special action or a
// property token fails.
func staticRestrictionTail(d Descriptor, tail string) (filter string, ok bool) {
	if d.Class != PurposeSpecial {
		return "", false
	}
	for start := 0; start <= len(tail); {
		end := start
		for end < len(tail) && tail[end] != '+' {
			end++
		}
		tok := tail[start:end]
		start = end + 1
		match, known := SpecialHasProperty(d.Special, tok)
		switch {
		case !known:
			if filter != "" {
				filter += "+"
			}
			filter += tok
		case !match:
			return "", false
		}
	}
	return filter, true
}

// CostPayableSpecial is the offer gate for special action sp on id: the same
// payability read CostPayable makes, priced against the restriction-aware pool
// of the special-action descriptor the payment (PayManaSpecial) builds.
func CostPayableSpecial(e Engine, p state.PlayerID, id state.ObjID, sp Special, cost Cost) bool {
	return costPayableDescriptor(e, p, SpecialDescriptor(id, sp, cost), cost, e.Eval().MayPlayRider(p, id))
}

// PayManaSpecial pays a special action's mana cost under the descriptor
// CostPayableSpecial priced, so the offer and the payment read one class.
func PayManaSpecial(e Engine, p state.PlayerID, id state.ObjID, sp Special, cost Cost) bool {
	ok, _, _, _, _ := PayManaDescriptorForSpent(e, p, SpecialDescriptor(id, sp, cost), cost, nil, PipRider{})
	return ok
}
