package pay

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// PaymentPlanReflectedShape reports whether a reflected-mana ability (AB$
// ManaReflected) has a shape V1 can plan: "add one mana of a colour that a
// land an opponent controls could produce" (Exotic Orchard), the same read
// over the payer's OWN permanents (Reflecting Pool, Gond Gate), a colour read
// ("Is": Mox Amber, Meteor Crater) and the Defined.* selectors no payment
// step can shrink (Chrome Mox's imprint, Pit of Offerings' exiled cards, a
// graveyard scan: a step only ever adds to a graveyard) and the source's own
// colours (Katilda's granted "any of this creature's colors"). The reflected
// colours are a finite choice the census resolves with the resolution's own
// object walk (effects.ManaReflectedObjects). ReflectProperty$ Produce reads
// printed mana abilities and Is reads colours, never tapped state, so a
// planned tap cannot move them; the one thing a step can do is remove an own
// reflected object (a self-sacrificing land), which the census handles by
// dropping those objects' contribution (rules.paymentPlanReflectedAlt).
// Everything else stays deferred: Defined.Untapped (a payment changes the
// tapped set), Defined.Sacrificed, an Amount$, a RestrictValid$, an
// activation gate, a Produced$ override, and ReflectProperty$ Produced.
func PaymentPlanReflectedShape(ma *cards.SA) bool {
	if ma == nil || ma.APIKind() != cards.APIManaReflected {
		return false
	}
	mr := effects.ManaReflectedOf(ma)
	if !paymentPlanReflectedProperties.Has(mr.ReflectProperty) || mr.Produced != "" || mr.Amount.Present ||
		mr.RestrictValid != "" || mr.ClassBand != "" || mr.IsPresent != "" {
		return false
	}
	return reflectsStableObjects(mr.Valid, mr.ReflectProperty)
}

// reflectsStableObjects reports whether the Valid$ spec names objects no
// planned payment step can add or (apart from a consuming mana ability the
// census filters) remove: a Defined.<selector> on the allow-list, Defined.Self
// under a colour read (a mana ability never changes its source's colours), or
// object alternatives that each carry OppCtrl or YouCtrl.
func reflectsStableObjects(spec, property string) bool {
	if spec == "" {
		return false
	}
	if rest, ok := strings.CutPrefix(spec, "Defined."); ok {
		sel, _, _ := strings.Cut(strings.TrimSpace(rest), " ")
		return paymentPlanReflectedDefined.Has(sel) || (sel == "Self" && property == "Is")
	}
	for alt := range strings.SplitSeq(spec, ",") {
		_, props, ok := strings.Cut(strings.TrimSpace(alt), ".")
		if !ok {
			return false
		}
		ctl := false
		for p := range strings.SplitSeq(props, "+") {
			ctl = ctl || paymentPlanReflectedControl.Has(p)
		}
		if !ctl {
			return false
		}
	}
	return true
}

// PaymentPlanReflectedCounts is the census's per-colour production of a
// PaymentPlanReflectedShape ability: one count per colour cols names (the
// resolution's candidate list), in the WindowAlt Counts convention a
// Produced$ Any alternative already uses. ok is false when no colour is
// named.
func PaymentPlanReflectedCounts(cols []string) (counts [6]int32, ok bool) {
	counts, _ = cards.ProducedCounts(strings.Join(cols, " "))
	// {C} stays out: the planner's choice expansion offers coloured
	// alternatives only (appendUnitAlternatives), and the census bound must
	// agree with it. A colourless pip is paid by the board's real {C} sources.
	counts[state.MC] = 0
	for _, n := range counts {
		if n > 0 {
			return counts, true
		}
	}
	return counts, false
}

// planAltChoiceColours is the colour list a choice-shaped census alternative
// expands into: a reflected ability's own census counts (the candidates the
// census resolved), else PaymentPlanChoiceColours' read of Produced$.
func planAltChoiceColours(e Engine, id state.ObjID, alt WindowAlt) []string {
	if alt.Ma.APIKind() != cards.APIManaReflected {
		return PaymentPlanChoiceColours(e, id, alt.Ma)
	}
	if !PaymentPlanReflectedShape(alt.Ma) {
		return nil
	}
	var cols []string
	for i, n := range alt.Counts {
		if n > 0 {
			cols = append(cols, string(cards.ManaSymbol(i)))
		}
	}
	return cols
}

// paymentPlanReflectedProperties is the ReflectProperty$ set
// PaymentPlanReflectedShape admits.
var paymentPlanReflectedProperties = state.NewNameSet("Produce", "Is")

// paymentPlanReflectedDefined is the Defined.<selector> head set the shape
// admits: the sets no payment step can shrink.
var paymentPlanReflectedDefined = state.NewNameSet("Imprinted", "ExiledWith", "ValidGraveyard")

// paymentPlanReflectedControl is the controller property a Valid$ alternative
// must carry.
var paymentPlanReflectedControl = state.NewNameSet("OppCtrl", "YouCtrl")

// paymentPlanReflectedParamSet is the reflected-only parameters the shape
// gate's key walk accepts on a PaymentPlanReflectedShape ability (their
// values are vetted by that predicate).
var paymentPlanReflectedParamSet = state.NewNameSet("ColorOrType", "ReflectProperty", "Valid")
