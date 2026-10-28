package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// stat:CantTarget is the printed S:Mode$ CantTarget static (Thrun, Breaker of
// Silence; Ground Seal; Karplusan Strider; ...) and the Effect-delivered body
// Vines of Vastwood and Autumn's Veil register through StaticAbilities$. Its
// own init keeps the registration out of the shared statics.go list, where a
// concurrent ticket also adds a mode.
func init() { effects.RegisterNonAPI("stat:CantTarget") }

// cantTargetBody is the scoping grammar a CantTarget restriction carries,
// extracted from either delivery path. One matcher judges both so the
// printed static and the Effect-delivered body cannot drift.
type cantTargetBody struct {
	validCard     string
	validTarget   string
	validCards    string
	validCreature string
	validSA       string
	validSource   string
	activator     string
	affectedZone  string
	// sourceCanOnlyTarget marks Wall of Shadows' unmodelled
	// SourceCanOnlyTarget$ body; see cantTargetBlocks.
	sourceCanOnlyTarget string
	remembered          []state.ObjID
	// effectDelivered distinguishes the registered Effect body's historical
	// "no ValidCard$/ValidTarget$ means the remembered set" reading from a
	// printed static, where an empty ValidTarget$ means every card in the
	// AffectedZone$ (Ground Seal's "cards in graveyards can't be targeted").
	effectDelivered bool
}

func cantTargetBodyOfEffect(ce *ContinuousEffect) cantTargetBody {
	return cantTargetBody{
		validCard:           ce.RestrictParam(cards.PKValidCard),
		validTarget:         ce.RestrictParam(cards.PKValidTarget),
		validCards:          ce.RestrictParam(cards.PKValidCards),
		validCreature:       ce.RestrictParam(cards.PKValidCreature),
		validSA:             ce.RestrictParam(cards.PKValidSA),
		validSource:         ce.RestrictParam(cards.PKValidSource),
		activator:           ce.RestrictParam(cards.PKActivator),
		affectedZone:        ce.RestrictParam(cards.PKAffectedZone),
		sourceCanOnlyTarget: ce.RestrictParams["SourceCanOnlyTarget"],
		remembered:          ce.Remembered,
		effectDelivered:     true,
	}
}

func cantTargetBodyOfStatic(sv staticView) cantTargetBody {
	return cantTargetBody{
		validCard:           sv.ParamStr(cards.PKValidCard),
		validTarget:         sv.ParamStr(cards.PKValidTarget),
		validCards:          sv.ParamStr(cards.PKValidCards),
		validCreature:       sv.ParamStr(cards.PKValidCreature),
		validSA:             sv.ParamStr(cards.PKValidSA),
		validSource:         sv.ParamStr(cards.PKValidSource),
		activator:           sv.ParamStr(cards.PKActivator),
		affectedZone:        sv.ParamStr(cards.PKAffectedZone),
		sourceCanOnlyTarget: sv.Params["SourceCanOnlyTarget"],
		remembered:          sv.Remembered,
	}
}

// cantTargetBlocks is Forge's applyCantTargetAbility order: AffectedZone$,
// ValidTarget$, ValidSA$, ValidSource$, Activator$. It reports whether this
// restriction withholds id from actor's source/sa.
func (e *Engine) cantTargetBlocks(b cantTargetBody, id state.ObjID, actor state.PlayerID, source state.ObjID, sa *cards.SA, staticSource state.ObjID, staticController state.PlayerID, sc effects.SpecContext) bool {
	// SourceCanOnlyTarget$ (Wall of Shadows) is not modelled. Dropping it and
	// honouring ValidTarget$ Card.Self would withhold every spell, the
	// opposite of the card's text; fail toward "no restriction" instead.
	if b.sourceCanOnlyTarget != "" {
		return false
	}
	o := e.G.Obj(id)
	if o == nil {
		return false
	}
	if b.affectedZone == "" {
		// Forge's default AffectedZone$ is the battlefield: a printed static
		// functions only from the battlefield (CR 604.3), so the historic
		// caller-side zone gate is part of the matcher now.
		if o.Zone != state.ZBattlefield {
			return false
		}
	} else if !zoneSpecContains(b.affectedZone, o.Zone) {
		return false
	}
	spec := b.validCard
	if spec == "" {
		spec = b.validTarget
	}
	if spec == "" {
		spec = b.validCards
	}
	if spec == "" {
		spec = b.validCreature
	}
	for _, r := range b.remembered {
		sc.Remembered = append(sc.Remembered, state.Target{Obj: r})
	}
	if spec != "" {
		if !e.matchesSpec(spec, id, sc) {
			return false
		}
	} else if b.effectDelivered && len(b.remembered) == 0 {
		// restrictionApplies' Effect-delivered convention: a body with
		// neither a ValidCard$/ValidTarget$ spec nor Remembered matches
		// nothing. A printed static with no ValidTarget$ (Ground Seal) means
		// every card in its AffectedZone$.
		return false
	}
	if !e.cantTargetValidSA(b.validSA, source, sa, staticController) {
		return false
	}
	if b.validSource != "" {
		// ValidSource$ names the targeting spell/ability's host card: the
		// spell card itself, or the permanent that granted an ability. It is
		// matched from the restriction controller's perspective so
		// Card.nonGreen+OppCtrl means an opponent of Thrun's controller.
		host := e.protectionSource(source)
		if host == 0 || !e.matchesSpecFrom(b.validSource, host, staticController, staticSource) {
			return false
		}
	}
	if b.activator != "" && !effects.MatchesPlayerSpecCtx(e.G, b.activator, actor, staticController, e.playerSpecCtx(staticSource)) {
		return false
	}
	return true
}

// cantTargetValidSA classifies the targeting spell/ability with the same
// Spell/Activated/Triggered split counterValidSA uses. The stack object is
// authoritative; only when it is gone can the SA's own kind decide the two
// exact one-word spellings (SP is a spell, AB an activated ability). A list
// or a triggered DB/ST body is unclassifiable there, so it fails open (no
// restriction).
func (e *Engine) cantTargetValidSA(spec string, source state.ObjID, sa *cards.SA, staticController state.PlayerID) bool {
	if spec == "" {
		return true
	}
	if o := e.G.Obj(source); o != nil {
		return e.counterValidSA(o, spec, staticController, source)
	}
	if sa == nil {
		return false
	}
	switch sa.Kind {
	case "SP":
		return spec == "Spell"
	case "AB":
		return spec == "Activated"
	}
	return false
}
