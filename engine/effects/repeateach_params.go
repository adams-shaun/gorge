package effects

import (
	"slices"
	"strings"
	"sync/atomic"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// This file is api:RepeatEach's parameter compiler (W4 step 3 of
// docs/design/2026-10-03-rules-engine-lasagna-design.md, section
// 8), on CopyPermanent's pattern (copypermanent_params.go).
// compileRepeatEach is the ONLY reader of a RepeatEach ability's own
// parameters: the resolution (repeateach.go) reads the compiled
// RepeatEachParams, including the RepeatCards$ scan's Zone$ set, which is a
// pure function of the text and so is decided here once.
// internal/codeshape's repeatEachParamLeaks ratchet holds that: no parameter
// read in repeateach.go, and no read of a RepeatEach-only key anywhere else
// in engine/rules/ or effects/.
//
// The DefinedCards$ subject list comes from the Defined-reference tier: the
// compiler keeps the compiled DefinedParams' card selector.

// RepeatEachParams is one RepeatEach ability's parameters, compiled once.
// Text fields are trimmed unless noted.
type RepeatEachParams struct {
	// paramBinding is the Params map the struct was compiled from
	// (typed_params.go's identity rule).
	paramBinding

	// SubAbility is RepeatSubAbility$, raw (the SVar name the body
	// resolves).
	SubAbility string
	// DamageMap, ChangeZoneTable, AmountFromVotes, ClearRemembered and
	// OptionalForEach are DamageMap$, ChangeZoneTable$, AmountFromVotes$,
	// ClearRememberedBeforeLoop$ (or ClearRemembered$) and
	// RepeatOptionalForEachPlayer$ True.
	DamageMap       bool
	ChangeZoneTable bool
	AmountFromVotes bool
	ClearRemembered bool
	OptionalForEach bool
	// OptionalMessage is RepeatOptionalMessage$.
	OptionalMessage string
	// Players and SpellAbilities are RepeatPlayers$ and
	// RepeatSpellAbilities$, raw (non-empty selects that subject list);
	// Targeted is a non-empty RepeatTargeted$.
	Players        string
	SpellAbilities string
	Targeted       bool
	// DefinedCards is the Defined tier's DefinedCards$ selector ("" when
	// absent): it names the card subjects ahead of RepeatCards$.
	DefinedCards string
	// Cards is RepeatCards$: the card filter the zone scan matches; Zones is
	// the scan's Zone$ set (the battlefield when Zone$ is blank).
	Cards string
	Zones ZoneMask
	// StartingWith is StartingWith$: "You" rotates a player-subject loop to
	// begin at the resolving controller (CR 101.4's APNAP order is Forge's
	// default; this names a different starting player).
	StartingWith string
	// ChooseOrder is ChooseOrder$: True orders the card subjects by the
	// controller, any other non-empty value names the chooser.
	ChooseOrder string
	// TypesFrom is RepeatTypesFrom$: its matching cards supply the distinct
	// card types bound as ChosenType for each iteration.
	TypesFrom string
	// UseImprinted is UseImprinted$ True: the iteration's subject binds as
	// Imprinted only, and the body's Remembered stays the host's own list.
	UseImprinted bool

	// Unread are the parameters present on the ability that no RepeatEach
	// reader consumes: the resolution Notes them once.
	Unread []string
}

// repeatEachKnownKeys is every parameter key a RepeatEach resolution
// consumes or deliberately ignores, sorted (changeZoneKnownKeys' contract):
// rules' TestRepeatEachKnownKeysMatchTheCensus holds it equal to the
// parameter census's measured api:RepeatEach read set plus its ignored and
// structural keys.
var repeatEachKnownKeys = [...]string{
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
	"AddType",
	"AddTypes",
	"AdditionalDesc",
	"AdditionalDescription",
	"Affected",
	"AlternateCost",
	"AlternativeCost",
	"AmountFromVotes",
	"Announce",
	"AnnounceTitle",
	"Boast",
	"CantCopy",
	"ChangeNumDesc", "ChangeTypeDesc",
	"ChangeValidDesc",
	"ChangeZoneTable",
	"CharacteristicDefining",
	"CheckSVar",
	"ChoiceDesc", "ChoiceTitle",
	"ChoiceTitleAppend", "ChoiceZone",
	"Choices",
	"ChooseFromList",
	"ChooseOrder",
	"ClassBand",
	"ClearImprinted",
	"ClearRemembered",
	"ClearRememberedBeforeLoop",
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
	"DamageMap",
	"Defined",
	"DefinedCards",
	"DefinedDesc",
	"DefinedTarget",
	"Description",
	"Exclude",
	"Exhaust",
	"ForgetOtherTargets",
	"FromDesc", "GameActivationLimit",
	"GiftDescription", "IfDesc",
	"Image",
	"ImprintCards",
	"ImprintPlayed",
	"InstantSpeed",
	"IsCurse",
	"IsPresent",
	"KW",
	"Keyword",
	"KeywordLine",
	"LockInText", "ManaConversion",
	"MaxTotalTargetCMC",
	"MaxTotalTargetPower",
	"Mentor",
	"ModeCost",
	"Monstrosity",
	"NewController",
	"NoPutDesc",
	"NonBasicSpell",
	"OpponentTurn",
	"Planeswalker",
	"PlayCost",
	"PlayerTurn",
	"PowerUp",
	"PrecostDesc",
	"PresentCompare",
	"PresentDefined",
	"PresentZone",
	"PrimaryPrompt",
	"RandomNumTargets",
	"ReduceAmount",
	"ReduceCost",
	"RememberAnimated",
	"RememberCostMana",
	"RememberObjects",
	"RememberTargets",
	"RepeatCards",
	"RepeatOptionalForEachPlayer",
	"RepeatOptionalMessage",
	"RepeatPlayers",
	"RepeatSpellAbilities",
	"RepeatSubAbility",
	"RepeatTargeted",
	"RepeatTypesFrom",
	"ReplaceColor",
	"ReplaceGraveyard",
	"ReplaceGraveyardValid",
	"ReplaceMana",
	"ReplaceOnly",
	"ReplaceType",
	"ReturnAbility",
	"ReturnValid",
	"SVarCompare",
	"SelectPrompt",
	"SetChosenMode",
	"SetColor",
	"ShowCards",
	"SorcerySpeed",
	"SpellDescription",
	"StackDescription",
	"StartingWith",
	"SubAbility",
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
	"UnlessAI",
	"UnlessCost",
	"UnlessPayer",
	"UnlessResolveSubs",
	"UnlessSwitched",
	"UseImprinted",
	"ValidCard",
	"ValidCards",
	"ValidCardsDesc",
	"ValidChoices",
	"ValidCounterType",
	"ValidDescription",
	"ValidTgts",
	"ValidTgtsDesc",
	"VoteMessage",
	"WithoutManaCost",
	"XMax",
	"XMin",
	"Zone",
}

// isRepeatEachSA reports whether sa resolves as api:RepeatEach.
func isRepeatEachSA(sa *cards.SA) bool {
	return sa != nil && sa.API == "RepeatEach"
}

// RepeatEachOf returns sa's compiled RepeatEach parameters: the configured
// record's when it is bound to sa's Params map, else the front cache's entry
// for that map, else a fresh compile stored in the front cache
// (ChangeZoneOf's contract).
func RepeatEachOf(sa *cards.SA) *RepeatEachParams {
	if f := LoadSAFacts(sa); f != nil {
		if p := f.RepeatEach; p != nil && p.boundTo(sa.Params) {
			return p
		}
	}
	slot := &repeatEachFront[paramMapSlot(sa.Params)]
	if p := slot.Load(); p != nil && p.boundTo(sa.Params) {
		return p
	}
	p := compileRepeatEach(sa, DefinedOf(sa))
	if sa.Params != nil {
		slot.Store(p)
	}
	return p
}

// repeatEachFront is RepeatEachOf's direct-mapped front cache (czFront's
// shape).
var repeatEachFront [1 << 10]atomic.Pointer[RepeatEachParams]

// repeatCardsZones is the RepeatCards$ scan's Zone$ vocabulary; any other
// entry names no zone.
var repeatCardsZones = [...]struct {
	name string
	zone state.Zone
}{
	{"Battlefield", state.ZBattlefield}, {"Hand", state.ZHand}, {"Library", state.ZLibrary},
	{"Graveyard", state.ZGraveyard}, {"Exile", state.ZExile}, {"Stack", state.ZStack},
}

// compileRepeatEach is the one reader of a RepeatEach ability's own
// parameters; DefinedCards$ comes from the Defined-reference tier's compiled
// DefinedParams.
func compileRepeatEach(sa *cards.SA, dr *DefinedParams) *RepeatEachParams {
	p := &RepeatEachParams{paramBinding: bindParams(sa), DefinedCards: dr.Cards.Text}
	p.SubAbility = sa.ParamStr(cards.PKRepeatSubAbility)
	p.DamageMap = isTrue(sa.ParamStr(cards.PKDamageMap))
	p.ChangeZoneTable = isTrue(sa.ParamStr(cards.PKChangeZoneTable))
	p.AmountFromVotes = isTrue(sa.ParamStr(cards.PKAmountFromVotes))
	p.ClearRemembered = isTrue(sa.ParamStr(cards.PKClearRememberedBeforeLoop)) ||
		isTrue(sa.ParamStr(cards.PKClearRemembered))
	p.OptionalForEach = isTrue(sa.ParamStr(cards.PKRepeatOptionalForEachPlayer))
	p.OptionalMessage = strings.TrimSpace(sa.ParamStr(cards.PKRepeatOptionalMessage))
	p.Players = sa.ParamStr(cards.PKRepeatPlayers)
	p.SpellAbilities = sa.ParamStr(cards.PKRepeatSpellAbilities)
	p.Targeted = sa.ParamStr(cards.PKRepeatTargeted) != ""
	p.Cards = strings.TrimSpace(sa.ParamStr(cards.PKRepeatCards))
	p.Zones = zoneMaskOf([]state.Zone{state.ZBattlefield})
	if raw := strings.TrimSpace(sa.ParamStr(cards.PKZone)); raw != "" {
		p.Zones = 0
		for z := range strings.SplitSeq(raw, ",") {
			z = strings.TrimSpace(z)
			for _, rz := range repeatCardsZones {
				if rz.name == z {
					p.Zones |= 1 << rz.zone
				}
			}
		}
	}
	p.StartingWith = strings.TrimSpace(sa.ParamStr(cards.PKStartingWith))
	p.ChooseOrder = strings.TrimSpace(sa.ParamStr(cards.PKChooseOrder))
	p.TypesFrom = strings.TrimSpace(sa.ParamStr(cards.PKRepeatTypesFrom))
	p.UseImprinted = isTrue(sa.ParamStr(cards.PKUseImprinted))
	p.Unread = unreadKeys(sa, repeatEachKnownKeys[:])
	return p
}

// RepeatEachKnownKeys is a copy of repeatEachKnownKeys, for the census
// check.
func RepeatEachKnownKeys() []string { return slices.Clone(repeatEachKnownKeys[:]) }
