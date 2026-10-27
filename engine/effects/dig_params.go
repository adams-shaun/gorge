package effects

import (
	"slices"
	"strings"
	"sync/atomic"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// This file is api:Dig's parameter compiler (W4 step 3 of
// docs/design/2026-10-03-rules-engine-lasagna-design.md, section
// 8), on CopyPermanent's pattern (copypermanent_params.go). compileDig is the
// ONLY reader of a Dig ability's own parameters: the resolution (dig.go)
// reads the compiled DigParams, including the defaulted ChangeValid$ spec,
// the primary destination and the remainder's second destination, which are
// pure functions of the text and so are decided here once.
// internal/codeshape's digParamLeaks ratchet holds that: no parameter read in
// dig.go, and no read of a Dig-only key anywhere else in engine/rules/ or effects/.
//
// The machinery Dig shares with the ChangeZone movers -- the face-down exile
// marker (ExileFaceDown$, WithMayLook$), the attacking-entry rider, the
// StaticEffect$ registration, ForgetOtherRemembered$ and the Defined$ player
// resolver -- still reads its own keys in its own files; that is the next
// tier.

// DigParams is one Dig ability's parameters, compiled once.
type DigParams struct {
	// paramBinding is the Params map the struct was compiled from
	// (typed_params.go's identity rule).
	paramBinding

	// DigNum is DigNum$ (the Num grammar, default 1); DigNumX reports the
	// bare X spelling the trigger-SVar fallback reads.
	DigNum  ParamText
	DigNumX bool
	// ChangeNum is ChangeNum$. ChangeNumAny is the any-number cap ("Any");
	// ChangeNumCapped is a value that narrows the cap (neither absent, All
	// nor Any), resolved through the Num grammar with DigNum as default.
	ChangeNum    ParamText
	ChangeNumAny bool
	// ChangeNumAll is the literal ChangeNum$ All: every matching card in
	// the window moves, so a take election is all-or-nothing.
	ChangeNumAll    bool
	ChangeNumCapped bool
	// WithTotalCMC is WithTotalCMC$, the cumulative mana-value budget.
	WithTotalCMC ParamText
	// Spec is ChangeValid$ (default "Card") through permanentCardSpec.
	Spec string
	// Dest is DestinationZone$ (default Hand), parsed.
	Dest state.Zone
	// Optional is Optional$ spelled exactly True; PromptToSkipOptional is
	// PromptToSkipOptionalAbility$ True or a non-blank OptionalAbilityPrompt$.
	Optional             bool
	PromptToSkipOptional bool
	// RevealWin is Reveal$ True without NoReveal$ True.
	RevealWin bool
	// The boolean riders: NoLooking$, ForceRevealToController$,
	// SkipReorder$, Tapped$, FromBottom$, RestRandomOrder$, Imprint$,
	// GainControl$ and RememberChanged$ True.
	NoLooking       bool
	ForceReveal     bool
	SkipReorder     bool
	Tapped          bool
	FromBottom      bool
	RestRandomOrder bool
	Imprint         bool
	GainControl     bool
	RememberChanged bool
	// PrimaryPos is LibraryPosition$.
	PrimaryPos string
	// Dest2Name and Pos2 are DestinationZone2$ and LibraryPosition2$ after
	// the bottom-of-library defaults (an omitted second destination, or an
	// explicit Library one with no second position, is the bottom).
	Dest2Name string
	Pos2      string
	// Choser is Choser$.
	Choser string
	// DestZone2Optional is DestinationZone2Optional$ True (Kellan, Daring
	// Traveler: "Otherwise, you may put it into your graveyard"): the
	// remainder move is a may election.
	DestZone2Optional bool
	// WithCounters is WithCounters$ (a counter kind) and WithCountersAmount$
	// its Num-grammar amount (default 1): a battlefield take enters with them.
	WithCounters       string
	WithCountersAmount ParamText
	// ExileWithCounters is ExileWithCounters$ (a counter kind): a card the Dig
	// moves to exile gets one counter of it there (CR 122.1; Karn, Scion of
	// Urza's "exile the other with a silver counter on it").
	ExileWithCounters string
	// WithDifferentPowers is WithDifferentPowers$ True: the take is a set
	// whose chosen cards must have pairwise different powers (Rip, Spawn
	// Hunter). The ask carries decision.SetPropDistinct with one power token
	// per option; the deterministic take is the greedy first-of-each-power.
	WithDifferentPowers bool
	// RevealOptional is RevealOptional$ True (Urza Assembles the Titans: "you
	// may reveal the top card"): the digging player elects whether the window is
	// revealed. RememberRevealed is RememberRevealed$ True: a revealed window
	// joins Remembered before the ChangeValid$ filter runs. SetChosenPlayer is
	// SetChosenPlayer$ True (Memories Returning): the resolved Choser$ becomes the
	// source's chosen player. ForceRevealTo is the ForceReveal$ player selector
	// (Gonti, Night Minister): the player the window is revealed to.
	RevealOptional   bool
	RememberRevealed bool
	SetChosenPlayer  bool
	ForceRevealTo    string

	// Unread are the parameters present on the ability that no Dig reader
	// consumes: the resolution Notes them once.
	Unread []string
}

// digKnownKeys is every parameter key a Dig resolution consumes or
// deliberately ignores, sorted (changeZoneKnownKeys' contract): rules'
// TestDigKnownKeysMatchTheCensus holds it equal to the parameter census's
// measured api:Dig read set plus its ignored and structural keys.
var digKnownKeys = [...]string{
	"AILifeThreshold",
	"AILogic",
	"AIMinTgts", "AINoRecursiveCheck",
	"AIPhyrexianPayment",
	"AITgtOwnCards", "AITgts",
	"AIXMax", "Activation",
	"ActivationAfterBlockers",
	"ActivationFirstCombat",
	"ActivationGameTypes",
	"ActivationLimit",
	"ActivationPhases",
	"ActivationZone",
	"Activator",
	"AdditionalDesc",
	"AdditionalDescription",
	"Affected",
	"AffectedZone",
	"AlternateCost",
	"AlternativeCost",
	"Announce",
	"AnnounceTitle",
	"Attacking",
	"Boast",
	"CantCopy",
	"ChangeNum",
	"ChangeNumDesc", "ChangeTypeDesc",
	"ChangeValid",
	"ChangeValidDesc",
	"CharacteristicDefining",
	"CheckSVar",
	"ChoiceDesc", "ChoiceTitle",
	"ChoiceTitleAppend", "ChoiceZone",
	"Choices",
	"ChooseFromList",
	"Choser",
	"ClassBand",
	"ClearImprinted",
	"Condition",
	"ConditionActivationLimit",
	"ConditionCheckSVar",
	"ConditionCompare",
	"ConditionCompare2",
	"ConditionDefined",
	"ConditionDescription",
	"ConditionFirstCombat",
	"ConditionManaNotSpent", "ConditionManaSpent", "ConditionNotPresent",
	"ConditionOptionalPaid",
	"ConditionPhases",
	"ConditionPlayerTurn",
	"ConditionPresent",
	"ConditionPresent2",
	"ConditionSVarCompare",
	"ConditionZone",
	"CopyCard",
	"Cost",
	"CostDesc",
	"Defined",
	"DefinedCards",
	"DefinedDesc",
	"DefinedExiler",
	"DefinedTarget",
	"Description",
	"DestZone2Optional",
	"DestinationZone",
	"DestinationZone2",
	"DigNum",
	"Exclude",
	"Exhaust",
	"ExileFaceDown",
	"ExileWithCounters",
	"FaceDown",
	"FaceDownPower",
	"FaceDownSetType",
	"FaceDownToughness",
	"ForceReveal",
	"ForceRevealToController",
	"Foretold",
	"ForgetOtherRemembered",
	"ForgetOtherTargets",
	"FromBottom",
	"FromDesc", "GainControl",
	"GainsAbilitiesOf",
	"GainsTriggerAbsOf",
	"GameActivationLimit",
	"GiftDescription", "IfDesc",
	"Image",
	"Imprint",
	"ImprintCards",
	"ImprintPlayed",
	"InstantSpeed",
	"IsCurse",
	"IsPresent",
	"KW",
	"Keyword",
	"KeywordLine",
	"LibraryPosition",
	"LibraryPosition2",
	"LockInText", "ManaConversion",
	"MaxTotalTargetCMC",
	"MaxTotalTargetPower",
	"Mentor",
	"ModeCost",
	"Monstrosity",
	"NewController",
	"NoLooking",
	"NoPutDesc",
	"NoReveal",
	"NonBasicSpell",
	"OpponentTurn",
	"Optional",
	"OptionalAbilityPrompt",
	"Pawprint", "Planeswalker",
	"PlayCost",
	"PlayerTurn",
	"PowerUp",
	"PrecostDesc",
	"PresentCompare",
	"PresentDefined",
	"PresentZone",
	"PrimaryPrompt",
	"PromptToSkipOptionalAbility",
	"RandomNumTargets",
	"ReduceAmount",
	"ReduceCost",
	"RememberAnimated",
	"RememberChanged",
	"RememberChosen", "RememberCostMana",
	"RememberObjects",
	"RememberRevealed",
	"RememberTargets",
	"ReplaceColor",
	"ReplaceGraveyard",
	"ReplaceGraveyardValid",
	"ReplaceMana",
	"ReplaceOnly",
	"ReplaceType",
	"RestRandomOrder",
	"ReturnAbility",
	"ReturnValid",
	"Reveal",
	"RevealOptional",
	"SVarCompare",
	"SelectPrompt",
	"SetChosenMode",
	"SetChosenPlayer",
	"SetColor",
	"SharedRestrictions",
	"ShowCards",
	"SkipReorder",
	"SorcerySpeed",
	"SpellDescription",
	"StackDescription",
	"StaticEffect",
	"StaticEffectCheckSVar",
	"StaticEffectSVarCompare",
	"SubAbility",
	"Tapped",
	"TargetMax",
	"TargetMin",
	"TargetType",
	"TargetUnique",
	"TargetValidTargeting",
	"TargetingPlayer",
	"TargetingPlayerControls",
	"TargetsAtRandom",
	"TargetsForEachPlayer",
	"TargetsWithControllerProperty",
	"TargetsWithDefinedController",
	"TargetsWithDifferentCMC",
	"TargetsWithDifferentControllers",
	"TargetsWithDifferentNames",
	"TargetsWithEqualToughness",
	"TargetsWithRelatedProperty",
	"TargetsWithSameCardType",
	"TargetsWithSameController",
	"TargetsWithSameCreatureType",
	"TargetsWithSharedCardType",
	"TargetsWithSharedTypes",
	"TgtPrompt",
	"TgtZone",
	"TokenScript",
	"TriggerDescription",
	"Type",
	"Ultimate",
	"Unearth",
	"UnlessAI",
	"UnlessCost",
	"UnlessPayer",
	"UnlessResolveSubs",
	"UnlessSwitched",
	"ValidCard",
	"ValidCards",
	"ValidCardsDesc",
	"ValidChoices",
	"ValidCounterType",
	"ValidDescription",
	"ValidTgts",
	"ValidTgtsDesc",
	"VoteMessage",
	"WithCounters",
	"WithCountersAmount",
	"WithDifferentPowers",
	"WithMayLook",
	"WithTotalCMC",
	"WithoutManaCost",
	"XMax",
	"XMin",
}

// isDigSA reports whether sa resolves as api:Dig.
func isDigSA(sa *cards.SA) bool {
	return sa != nil && (sa.CompiledAPI() == cards.APIDig || sa.API == "Dig")
}

// DigOf returns sa's compiled Dig parameters: the configured record's when
// it is bound to sa's Params map, else the front cache's entry for that map,
// else a fresh compile stored in the front cache (ChangeZoneOf's contract).
func DigOf(sa *cards.SA) *DigParams {
	if f := LoadSAFacts(sa); f != nil {
		if p := f.Dig; p != nil && p.boundTo(sa.Params) {
			return p
		}
	}
	slot := &digFront[paramMapSlot(sa.Params)]
	if p := slot.Load(); p != nil && p.boundTo(sa.Params) {
		return p
	}
	p := compileDig(sa)
	if sa.Params != nil {
		slot.Store(p)
	}
	return p
}

// digFront is DigOf's direct-mapped front cache (czFront's shape).
var digFront [1 << 10]atomic.Pointer[DigParams]

// compileDig is the one reader of a Dig ability's parameters.
func compileDig(sa *cards.SA) *DigParams {
	p := &DigParams{paramBinding: bindParams(sa)}
	p.DigNum = rawParamText(sa, "DigNum")
	p.DigNumX = strings.TrimSpace(p.DigNum.Text) == "X"
	changeNum, ok := sa.Param(cards.PKChangeNum)
	p.ChangeNum = ParamText{Text: changeNum, Present: ok}
	p.ChangeNumAll = changeNum == "All"
	if changeNum != "" && changeNum != "All" {
		p.ChangeNumAny = changeNum == "Any"
		p.ChangeNumCapped = !p.ChangeNumAny
	}
	p.WithTotalCMC = rawParamText(sa, "WithTotalCMC")
	spec := rawParamText(sa, "ChangeValid").Text
	if spec == "" {
		spec = "Card"
	}
	p.Spec = permanentCardSpec(spec)
	destName := rawParamText(sa, "DestinationZone").Text
	if destName == "" {
		destName = "Hand"
	}
	p.Dest = ParseZone(destName)
	p.Optional = sa.ParamStr(cards.PKOptional) == "True"
	p.PromptToSkipOptional = isTrue(rawParamText(sa, "PromptToSkipOptionalAbility").Text) ||
		strings.TrimSpace(rawParamText(sa, "OptionalAbilityPrompt").Text) != ""
	p.RevealWin = isTrue(sa.ParamStr(cards.PKReveal)) && !isTrue(sa.ParamStr(cards.PKNoReveal))
	p.NoLooking = isTrue(sa.ParamStr(cards.PKNoLooking))
	p.ForceReveal = isTrue(rawParamText(sa, "ForceRevealToController").Text)
	p.SkipReorder = isTrue(rawParamText(sa, "SkipReorder").Text)
	p.Tapped = isTrue(sa.ParamStr(cards.PKTapped))
	p.FromBottom = isTrue(rawParamText(sa, "FromBottom").Text)
	p.RestRandomOrder = isTrue(rawParamText(sa, "RestRandomOrder").Text)
	p.Imprint = isTrue(sa.ParamStr(cards.PKImprint))
	p.GainControl = isTrue(sa.ParamStr(cards.PKGainControl))
	p.RememberChanged = isTrue(sa.ParamStr(cards.PKRememberChanged))
	p.PrimaryPos = strings.TrimSpace(sa.ParamStr(cards.PKLibraryPosition))
	p.Dest2Name = strings.TrimSpace(rawParamText(sa, "DestinationZone2").Text)
	p.Pos2 = strings.TrimSpace(rawParamText(sa, "LibraryPosition2").Text)
	// An omitted DestinationZone2$ is the library; its LibraryPosition2$
	// defaults to -1 (bottom) only when it too is omitted; an explicit
	// LibraryPosition2$ 0 keeps the rest on top (Jace, the Mind Sculptor;
	// Goblin Guide).
	if p.Dest2Name == "" {
		p.Dest2Name = "Library"
		if p.Pos2 == "" {
			p.Pos2 = "-1"
		}
	} else if strings.EqualFold(p.Dest2Name, "Library") && p.Pos2 == "" {
		p.Pos2 = "-1"
	}
	p.Choser = strings.TrimSpace(rawParamText(sa, "Choser").Text)
	p.DestZone2Optional = isTrue(rawParamText(sa, "DestZone2Optional").Text)
	p.WithCounters = strings.TrimSpace(rawParamText(sa, "WithCounters").Text)
	p.WithCountersAmount = rawParamText(sa, "WithCountersAmount")
	p.ExileWithCounters = strings.TrimSpace(rawParamText(sa, "ExileWithCounters").Text)
	p.WithDifferentPowers = isTrue(rawParamText(sa, "WithDifferentPowers").Text)
	p.RevealOptional = isTrue(rawParamText(sa, "RevealOptional").Text)
	p.RememberRevealed = isTrue(rawParamText(sa, "RememberRevealed").Text)
	p.SetChosenPlayer = isTrue(rawParamText(sa, "SetChosenPlayer").Text)
	p.ForceRevealTo = strings.TrimSpace(rawParamText(sa, "ForceReveal").Text)
	p.Unread = unreadKeys(sa, digKnownKeys[:])
	return p
}

// DigKnownKeys is a copy of digKnownKeys, for the census check.
func DigKnownKeys() []string { return slices.Clone(digKnownKeys[:]) }
