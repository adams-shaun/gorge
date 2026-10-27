package effects

import (
	"slices"
	"strings"
	"sync/atomic"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// This file is api:Clone's parameter compiler (W4 step 3 of
// docs/design/2026-10-03-rules-engine-lasagna-design.md, section
// 8), on CopyPermanent's pattern (copypermanent_params.go). compileClone is
// the ONLY reader of a Clone ability's own parameters: the resolution
// (clone.go) and rules' as-enters copy election (cast_etbchoice.go) read the
// compiled CloneParams. Every value that is a pure function of the text --
// the ChoiceZone$ classification, the type/keyword/name lists, the SetColor$
// parse, the ETB whitelist's text half -- is decided here once.
// internal/codeshape's cloneParamLeaks ratchet holds that: no parameter read
// in clone.go, and no read of a Clone-only key anywhere else in engine/rules/ or
// effects/.
//
// The lists below are SHARED by every resolution of the ability: a reader
// that hands one to game state (a continuous effect's AddTypes/AddKeywords/
// AddAbilities) clones it first, so no state object aliases the compiled
// record.

// CloneParams is one Clone ability's parameters, compiled once. Text fields
// are trimmed unless noted.
type CloneParams struct {
	// paramBinding is the Params map the struct was compiled from
	// (typed_params.go's identity rule).
	paramBinding

	// CopyFromChosenName is CopyFromChosenName$ True.
	CopyFromChosenName bool
	// Defined is Defined$ (the Defined tier's Ref text), the copy source.
	Defined string
	// Choices is Choices$, the mid-resolution (and ETB) copy-template
	// selector; ChoicesRaw is the untrimmed value.
	Choices    string
	ChoicesRaw string
	// ChoiceZone is ChoiceZone$; ChoiceZoneKind/ChoiceZoneOK are its
	// cloneChoiceZone classification.
	ChoiceZone     string
	ChoiceZoneKind state.Zone
	ChoiceZoneOK   bool
	// RememberCloneOrigin is RememberCloneOrigin$ True: the copied object joins
	// the become object's remembered set (Forge's CloneEffect addRemembered).
	RememberCloneOrigin bool
	// ChoiceOptional is ChoiceOptional$ True; ChoiceTitle is ChoiceTitle$.
	ChoiceOptional bool
	ChoiceTitle    string
	// CloneTarget is CloneTarget$, the become operand.
	CloneTarget string
	// ChoicesTargetBecome reports Forge CloneEffect's become arm for a
	// Choices$ source on a targeting ability with no CloneTarget$: the SA's
	// own target becomes the copy ("choose a creature; target creature
	// becomes a copy of it" -- Cytoshape, Kaya, Spirits' Justice), never the
	// resolving source.
	ChoicesTargetBecome bool
	// ExcludeChosen is ExcludeChosen$ True.
	ExcludeChosen bool
	// CloneZone is CloneZone$, the copy source's required zone.
	CloneZone string
	// Optional is Optional$ True.
	Optional bool

	// TypeAdds is the layer-4 add list: AddTypes$ members, then
	// SetCreatureTypes$ members; SetCreatureTypes reports a non-empty
	// SetCreatureTypes$ list.
	TypeAdds         []string
	SetCreatureTypes bool
	// The layer-4 removal flags: NonLegendary$, RemoveSubTypes$,
	// RemoveCardTypes$ and RemoveCreatureTypes$ True.
	NonLegendary        bool
	RemoveSubTypes      bool
	RemoveCardTypes     bool
	RemoveCreatureTypes bool
	// AddAbilities, AddSVars and AddTriggers are the named grants' SVar
	// names in printed order; StaticNames are AddStaticAbilities$' names.
	AddAbilities []string
	AddSVars     []string
	AddTriggers  []string
	StaticNames  []string
	// AddKeywords and PumpKeywords are the keyword lists; PumpDuration is
	// PumpDuration$. IfNew are AddKeywords$'s "IfNew <keyword>" conditional
	// riders, split out at compile: each member is the INNER keyword line
	// ("Vanishing:3") the copy gains only when the copied template's printed
	// face does not already carry that keyword's head (CR 707.9b; Flesh
	// Duplicate is the corpus's only instance). AddKeywords keeps the
	// unconditional members, which ride the ordinary layer-6 grant.
	AddKeywords  []string
	PumpKeywords []string
	PumpDuration string
	IfNew        []string
	// NewName is NewName$; GainThisAbility is GainThisAbility$ True.
	NewName         string
	GainThisAbility bool
	// SetPower and SetToughness are the raw P/T setters (the Num grammar).
	SetPower     ParamText
	SetToughness ParamText
	// SetColor is SetColor$; SetColors/SetColorOK are its colour parse.
	SetColor   string
	SetColors  []string
	SetColorOK bool
	// AddColors is AddColors$ (additive, CR 707.9b); AddColorsOK its parse.
	AddColors   string
	AddColorsL  []string
	AddColorsOK bool
	// IntoPlayTapped is IntoPlayTapped$ as the script spells it (the
	// no-entry Note quotes it); IntoPlayTappedSet reports a non-blank value
	// and IntoPlayTappedTrue the exact True spelling.
	IntoPlayTapped     string
	IntoPlayTappedSet  bool
	IntoPlayTappedTrue bool
	// Duration is Duration$; AttachedTo is AttachedTo$.
	Duration   string
	AttachedTo string
	// FaceDown is FaceDown$ True; KeepFacedownFalse is KeepFacedown$ False.
	FaceDown          bool
	KeepFacedownFalse bool

	// ETBShapeOK is the text half of rules' ETB copy whitelist
	// (etbCloneWhitelist): every key inside the supported set, a Choices$
	// selector the matcher can decide (including Mockingbird's cast-spend Y),
	// single-word AddKeywords$ heads and an absent-or-True IntoPlayTapped$.
	// The SVar-table half (each
	// StaticNames member readable) is the caller's.
	ETBShapeOK bool

	// Unread are the parameters present on the ability that no Clone reader
	// consumes: the resolution Notes them once.
	Unread []string
}

// cloneKnownKeys is every parameter key a Clone resolution consumes or
// deliberately ignores, sorted (changeZoneKnownKeys' contract): rules'
// TestCloneKnownKeysMatchTheCensus holds it equal to the parameter census's
// measured api:Clone read set plus its ignored and structural keys.
var cloneKnownKeys = [...]string{
	"AILifeThreshold", "AILogic", "AIMinTgts", "AINoRecursiveCheck", "AIPhyrexianPayment", "AITgtOwnCards", "AITgts",
	"AIXMax", "Activation", "ActivationAfterBlockers", "ActivationFirstCombat", "ActivationGameTypes",
	"ActivationLimit", "ActivationPhases", "ActivationZone", "Activator", "AddAbilities", "AddColors",
	"AddKeywords", "AddSVars", "AddStaticAbilities", "AddTriggers", "AddTypes",
	"AdditionalDesc", "AdditionalDescription", "Affected", "AlternateCost",
	"AlternativeCost", "Announce", "AnnounceTitle", "AttachedTo", "Boast", "CantCopy", "ChangeNumDesc", "ChangeTypeDesc",
	"ChangeValidDesc", "CharacteristicDefining", "CheckSVar", "ChoiceDesc", "ChoiceOptional", "ChoiceTitle", "ChoiceTitleAppend", "ChoiceZone",
	"Choices", "ChooseFromList", "ClassBand", "ClearImprinted", "CloneTarget", "CloneZone",
	"Condition", "ConditionActivationLimit", "ConditionCheckSVar", "ConditionCompare", "ConditionCompare2",
	"ConditionDefined", "ConditionDescription", "ConditionFirstCombat",
	"ConditionManaNotSpent", "ConditionManaSpent", "ConditionNotPresent", "ConditionOptionalPaid", "ConditionPhases", "ConditionPlayerTurn", "ConditionPresent", "ConditionPresent2",
	"ConditionSVarCompare", "ConditionZone", "CopyCard", "CopyFromChosenName", "Cost", "CostDesc", "Defined",
	"DefinedCards", "DefinedDesc", "DefinedTarget", "Description", "Duration", "Exclude", "ExcludeChosen", "Exhaust", "FaceDown", "ForgetOtherTargets", "FromDesc", "GainThisAbility", "GameActivationLimit", "GiftDescription", "IfDesc", "Image",
	"ImprintCards", "ImprintPlayed", "InstantSpeed", "IntoPlayTapped", "IsCurse",
	"IsPresent", "KW", "KeepFacedown", "Keyword", "KeywordLine", "LockInText", "ManaConversion", "MaxTotalTargetCMC",
	"MaxTotalTargetPower", "Mentor", "ModeCost", "Monstrosity", "NewController", "NewName",
	"NoPutDesc", "NonBasicSpell", "NonLegendary", "OpponentTurn", "Optional", "Planeswalker", "PlayCost",
	"PlayerTurn", "PowerUp", "PrecostDesc", "PresentCompare", "PresentDefined",
	"PresentZone", "PrimaryPrompt", "PumpDuration", "PumpKeywords", "RandomNumTargets", "ReduceAmount",
	"ReduceCost", "RememberAnimated", "RememberCloneOrigin", "RememberCostMana", "RememberObjects", "RememberTargets", "RemoveCardTypes",
	"RemoveCreatureTypes", "RemoveSubTypes", "ReplaceColor", "ReplaceGraveyard",
	"ReplaceGraveyardValid", "ReplaceMana", "ReplaceOnly", "ReplaceType", "ReturnAbility", "ReturnValid", "SVarCompare",
	"SelectPrompt", "SetChosenMode", "SetColor", "SetCreatureTypes", "SetPower",
	"SetToughness", "ShowCards", "SorcerySpeed", "SpellDescription", "StackDescription",
	"SubAbility", "TargetMax", "TargetMin", "TargetType", "TargetUnique",
	"TargetValidTargeting", "TargetingPlayer", "TargetingPlayerControls", "TargetsAtRandom",
	"TargetsForEachPlayer", "TargetsWithControllerProperty", "TargetsWithDefinedController",
	"TargetsWithDifferentCMC", "TargetsWithDifferentControllers",
	"TargetsWithDifferentNames", "TargetsWithEqualToughness", "TargetsWithRelatedProperty", "TargetsWithSameCardType",
	"TargetsWithSameController", "TargetsWithSameCreatureType", "TargetsWithSharedCardType",
	"TargetsWithSharedTypes", "TgtPrompt", "TgtZone", "TokenScript", "TriggerDescription",
	"Type", "Ultimate", "UnlessAI", "UnlessCost", "UnlessPayer", "UnlessResolveSubs",
	"UnlessSwitched", "ValidCard", "ValidCards", "ValidCardsDesc", "ValidChoices",
	"ValidCounterType", "ValidDescription", "ValidTgts", "ValidTgtsDesc", "VoteMessage", "WithoutManaCost",
	"XMax", "XMin",
}

// isCloneSA reports whether sa resolves as api:Clone.
func isCloneSA(sa *cards.SA) bool {
	return sa != nil && sa.API == "Clone"
}

// CloneOf returns sa's compiled Clone parameters: the configured record's
// when it is bound to sa's Params map, else the front cache's entry for that
// map, else a fresh compile stored in the front cache (ChangeZoneOf's
// contract).
func CloneOf(sa *cards.SA) *CloneParams {
	if f := LoadSAFacts(sa); f != nil {
		if p := f.Clone; p != nil && p.boundTo(sa.Params) {
			return p
		}
	}
	slot := &cloneFront[paramMapSlot(sa.Params)]
	if p := slot.Load(); p != nil && p.boundTo(sa.Params) {
		return p
	}
	p := compileClone(sa, DefinedOf(sa))
	if sa.Params != nil {
		slot.Store(p)
	}
	return p
}

// cloneFront is CloneOf's direct-mapped front cache (czFront's shape).
var cloneFront [1 << 10]atomic.Pointer[CloneParams]

// compileClone is the one reader of a Clone ability's own parameters; the
// Defined$ source selector comes from the Defined-reference tier's compiled
// DefinedParams.
func compileClone(sa *cards.SA, dr *DefinedParams) *CloneParams {
	p := &CloneParams{paramBinding: bindParams(sa)}
	p.CopyFromChosenName = strings.EqualFold(rawParamText(sa, "CopyFromChosenName").Text, "True")
	p.Defined = dr.Defined.Text
	p.ChoicesRaw = sa.ParamStr(cards.PKChoices)
	p.Choices = strings.TrimSpace(p.ChoicesRaw)
	p.ChoiceZone = strings.TrimSpace(sa.ParamStr(cards.PKChoiceZone))
	p.ChoiceZoneKind, p.ChoiceZoneOK = cloneChoiceZone(p.ChoiceZone)
	p.ChoiceOptional = isTrue(sa.ParamStr(cards.PKChoiceOptional))
	p.RememberCloneOrigin = isTrue(rawParamText(sa, "RememberCloneOrigin").Text)
	p.ChoiceTitle = strings.TrimSpace(sa.ParamStr(cards.PKChoiceTitle))
	p.CloneTarget = strings.TrimSpace(rawParamText(sa, "CloneTarget").Text)
	p.ChoicesTargetBecome = p.Choices != "" && p.CloneTarget == "" && TargetsOf(sa).Targeted()
	p.ExcludeChosen = isTrue(rawParamText(sa, "ExcludeChosen").Text)
	p.CloneZone = strings.TrimSpace(rawParamText(sa, "CloneZone").Text)
	p.Optional = isTrue(sa.ParamStr(cards.PKOptional))

	p.TypeAdds = splitAmp(strings.TrimSpace(sa.ParamStr(cards.PKAddTypes)))
	if setCreatureTypes := splitAmp(strings.TrimSpace(rawParamText(sa, "SetCreatureTypes").Text)); len(setCreatureTypes) > 0 {
		p.TypeAdds = append(p.TypeAdds, setCreatureTypes...)
		p.SetCreatureTypes = true
	}
	p.NonLegendary = isTrue(sa.ParamStr(cards.PKNonLegendary))
	p.RemoveSubTypes = isTrue(rawParamText(sa, "RemoveSubTypes").Text)
	p.RemoveCardTypes = isTrue(sa.ParamStr(cards.PKRemoveCardTypes))
	p.RemoveCreatureTypes = isTrue(sa.ParamStr(cards.PKRemoveCreatureTypes))
	p.AddAbilities = cloneNames(sa.ParamStr(cards.PKAddAbilities))
	p.AddSVars = cloneNames(sa.ParamStr(cards.PKAddSVars))
	p.AddTriggers = cloneNames(sa.ParamStr(cards.PKAddTriggers))
	p.StaticNames = strings.FieldsFunc(sa.ParamStr(cards.PKAddStaticAbilities), func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})
	p.AddKeywords, p.IfNew = cloneSplitIfNew(cards.SplitKeywordList(sa.ParamStr(cards.PKAddKeywords)))
	p.PumpKeywords = cards.SplitKeywordList(sa.ParamStr(cards.PKPumpKeywords))
	p.PumpDuration = strings.TrimSpace(sa.ParamStr(cards.PKPumpDuration))
	p.NewName = strings.TrimSpace(rawParamText(sa, "NewName").Text)
	p.GainThisAbility = isTrue(rawParamText(sa, "GainThisAbility").Text)
	setPower, ok := sa.Param(cards.PKSetPower)
	p.SetPower = ParamText{Text: setPower, Present: ok}
	setToughness, ok := sa.Param(cards.PKSetToughness)
	p.SetToughness = ParamText{Text: setToughness, Present: ok}
	p.SetColor = strings.TrimSpace(sa.ParamStr(cards.PKSetColor))
	if p.SetColor != "" {
		p.SetColors, p.SetColorOK = colorLetters(p.SetColor)
	}
	p.AddColors = strings.TrimSpace(sa.ParamStr(cards.PKAddColors))
	if p.AddColors != "" {
		p.AddColorsL, p.AddColorsOK = colorLetters(p.AddColors)
	}
	p.IntoPlayTapped = sa.ParamStr(cards.PKIntoPlayTapped)
	p.IntoPlayTappedSet = strings.TrimSpace(p.IntoPlayTapped) != ""
	p.IntoPlayTappedTrue = strings.EqualFold(p.IntoPlayTapped, "True")
	p.Duration = strings.TrimSpace(sa.ParamStr(cards.PKDuration))
	p.AttachedTo = strings.TrimSpace(sa.ParamStr(cards.PKAttachedTo))
	p.FaceDown = strings.EqualFold(sa.ParamStr(cards.PKFaceDown), "True")
	p.KeepFacedownFalse = strings.EqualFold(rawParamText(sa, "KeepFacedown").Text, "False")

	p.ETBShapeOK = cloneETBShapeOK(sa, p)
	p.Unread = unreadKeys(sa, cloneKnownKeys[:])
	return p
}

// CloneIfNew returns the conditional keyword riders of an api:Clone body,
// each already trimmed of Forge's "IfNew " prefix (Flesh Duplicate's
// `AddKeywords$ IfNew Vanishing:3`). The entry-counter machinery reads them
// at the entry's body scan (engine/rules/entry_counters.go): the clone body applies
// the copy basis face only after the move folds, so a rider's counters are
// derived from the rider itself, its condition compared against the chosen
// template's printed face.
func CloneIfNew(sa *cards.SA) []string { return CloneOf(sa).IfNew }

// cloneETBShapeOK is ETBShapeOK's computation: rules' etbCloneWhitelist's
// POSITIVE key whitelist (Choices$, AddKeywords$, AddTypes$,
// SpellDescription$, AddStaticAbilities$, IntoPlayTapped$, AddTriggers$,
// AddSVars$, SetPower$/SetToughness$ (literal), SetColor$/AddColors$ and
// NonLegendary$ True -- the CR 707.9b exceptions CopyExcept writes) plus its
// text-only value checks. The key scan only answers a boolean, so the map's
// iteration order cannot reach an event.
func cloneETBShapeOK(sa *cards.SA, p *CloneParams) bool {
	for k := range sa.Params {
		if !cloneETBShapeKeys.Has(k) {
			return false
		}
	}
	if SpecNeedsResolver(p.Choices) && p.Choices != mockingbirdETBSelector {
		return false
	}
	// ChoiceZone$ names the zone the template is chosen from (Echoing Deeps'
	// "land card in a graveyard"); a zone this build cannot classify fails
	// closed rather than offering a battlefield pool.
	if p.ChoiceZone != "" && !p.ChoiceZoneOK {
		return false
	}
	if _, ok := sa.Params["RememberCloneOrigin"]; ok && !p.RememberCloneOrigin {
		return false
	}
	for _, kw := range p.AddKeywords {
		if strings.ContainsAny(cards.KeywordHead(kw), " \t") {
			return false
		}
	}
	for _, kw := range p.IfNew {
		// The conditional rider's inner keyword must be a single-word head
		// the fold-time expansion can mint on the copy basis face
		// (cards.ApplyKeywordExpansion), and its entry-counter grants must be
		// fully derivable from the line AND nonempty (cards.
		// KeywordEntryCounters) -- the counters are folded into the move's
		// Pairs payload from the rider, before the basis face exists, so a
		// rider that names no counters or names them underivably fails
		// closed. The body stays withheld and the carrier keeps its loud
		// fallback, the same boundary every out-of-scope ETB rider lives
		// behind.
		if strings.ContainsAny(cards.KeywordHead(kw), " \t") ||
			!cards.KeywordExpandable(cards.KeywordHead(kw)) {
			return false
		}
		cs, complete := cards.KeywordEntryCounters(kw)
		if !complete || len(cs) == 0 {
			return false
		}
	}
	// The rider rides the ClonePermanent event's variant payload, whose
	// Counter is already taken by the gain-this-ability provenance and whose
	// basis face is resolved from the name universe on the chosen-name
	// route (where the conditional's template face is not the copied face).
	// A rider next to either is out of scope and fails closed.
	if len(p.IfNew) > 0 && (p.GainThisAbility || p.CopyFromChosenName) {
		return false
	}
	if _, ok := sa.Param(cards.PKIntoPlayTapped); ok && !p.IntoPlayTappedTrue {
		return false
	}
	// The characteristic exceptions ride the copy's CopyExcept (CR 707.9b),
	// so the as-enters route carries them exactly as the becomes-a-copy
	// route does. Only values the body resolves without an entry-time
	// reader are offered: a literal P/T (Quicksilver Gargantuan's 7/7, not
	// Hulking Metamorph's Count$CardPower), a colour list this build parses
	// (Vesuvan Doppelganger's SetColor$ Blue), and NonLegendary$ True.
	for _, pt := range [...]ParamText{p.SetPower, p.SetToughness} {
		if n, ok := literalInt(pt); pt.Present && (!ok || n < 0) {
			return false
		}
	}
	if (p.SetColor != "" && !p.SetColorOK) || (p.AddColors != "" && !p.AddColorsOK) {
		return false
	}
	if _, ok := sa.Param(cards.PKNonLegendary); ok && !p.NonLegendary {
		return false
	}
	return true
}

// cloneSplitIfNew moves every "IfNew <keyword>" member of a Clone's
// AddKeywords$ list into the conditional rider list and returns the
// unconditional remainder. The prefix is Forge's spelling (Flesh Duplicate's
// `AddKeywords$ IfNew Vanishing:3`); an "IfNew" member with no keyword after
// it is not a rider this build can read, so it is dropped from the remainder
// and kept as a rider with an empty keyword -- whose head check in
// cloneETBShapeOK then fails closed.
func cloneSplitIfNew(kws []string) (rest, ifNew []string) {
	for _, kw := range kws {
		if inner, ok := strings.CutPrefix(kw, "IfNew "); ok {
			ifNew = append(ifNew, strings.TrimSpace(inner))
			continue
		}
		rest = append(rest, kw)
	}
	return rest, ifNew
}

// CloneKnownKeys is a copy of cloneKnownKeys, for the census check.
func CloneKnownKeys() []string { return slices.Clone(cloneKnownKeys[:]) }

// cloneETBShapeKeys are the keys the implemented as-enters Clone shape carries.
// SubAbility$ is admitted as a key only; its chain's shape is
// CloneETBSubChainOK's (the "enters with additional counters" rider).
var cloneETBShapeKeys = state.NewNameSet(
	"Choices", "AddKeywords", "AddTypes", "SpellDescription",
	"AddStaticAbilities", "IntoPlayTapped", "AddTriggers", "AddSVars", "SubAbility",
	"SetPower", "SetToughness", "SetColor", "AddColors", "NonLegendary",
	"ChoiceZone", "NewName", "ChoiceTitle", "RememberCloneOrigin",
)
