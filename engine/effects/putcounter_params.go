package effects

import (
	"slices"
	"strings"
	"sync/atomic"

	"github.com/adams-shaun/gorge/engine/cards"
)

// This file is api:PutCounter's parameter compiler (W4 step 3 of
// docs/design/2026-10-03-rules-engine-lasagna-design.md, section
// 8), on ChangeZone's pattern (changezone_params.go). compilePutCounter is the
// ONLY reader of a PutCounter ability's own parameters: the resolution
// (counters_put.go), the rules-side entry-counter fold (entry_counters.go:
// ETB$, CounterType$, CounterNum$ and the asking-modifier presence gate), the
// Adapt$/Monstrosity$ offer gates, the opening-hand counter put and the
// per-recipient counter-kind answer binding all read the compiled
// PutCounterParams, so the offer, the entry fold and the resolution can no
// longer read a parameter differently. internal/codeshape's
// putCounterParamLeaks ratchet holds that: no parameter read in
// counters_put.go, and no read of a PutCounter-only key anywhere else in
// engine/rules/ or effects/.
//
// The generic machinery a PutCounter runs -- the Defined$ resolver, the
// cast/activation/targeting tier (ValidTgts$, Cost$), the Monstrosity$ cost
// property (saFlagProperty) -- still reads its own keys; those are the next
// tier.

// PutCounterParams is one PutCounter ability's parameters, compiled once.
type PutCounterParams struct {
	// paramBinding is the Params map the struct was compiled from
	// (typed_params.go's identity rule).
	paramBinding

	// The count: CounterNum$, then the Adapt$/Monstrosity$/Renown$ fallbacks
	// (each Set when its trimmed text is non-empty).
	CounterNum     ParamText
	CounterNumSet  bool
	Adapt          ParamText
	AdaptSet       bool
	Monstrosity    ParamText
	MonstrositySet bool
	Renown         ParamText
	RenownSet      bool

	// CounterType is CounterType$ as written; Kind is its canonical spelling
	// (canonicalCounterKind), "P1P1" when absent or empty.
	CounterType string
	Kind        string
	// Kinds is Kind split as a comma list (splitCounterKinds).
	Kinds []string

	// Placer$, trimmed.
	Placer string
	// TriggeredCounterMap$ (presence, Forge's hasParam) copies the triggering
	// CounterPlayerAddedAll batch's per-kind counter map instead of reading
	// CounterType$/CounterNum$: one put of each kind, with that kind's batch
	// count, on every Defined recipient. CounterMapValues$, when present,
	// replaces every kind's count with that value (Aragorn, Company Leader's
	// "one of each of those kinds").
	TriggeredCounterMap bool
	CounterMapValues    ParamText
	// Optional$ True.
	OptionalTrue bool
	// Tribute$ True (CR 702.104's tribute, kw_tribute.go's expansion): the
	// put is the tribute offer -- the entering creature's controller picks
	// the opponent, the chosen opponent elects pay/decline, and an
	// acceptance folds the events.AlterAttribute "Tributed" designation.
	Tribute bool
	// Bolster$ and Support$ (presence selects the shape).
	Bolster ParamText
	Support ParamText
	// Divided is a non-empty DividedAsYouChoose$.
	Divided bool
	// DividedRandomly is DividedRandomly$ True: the CounterNum$ total is
	// divided among the recipients at random, one rng draw per counter
	// (Orcish Catapult, Faerie Dragon), instead of placed on each.
	DividedRandomly bool
	// Choices$, trimmed; Chooser$ and ChoiceTitle$ as written.
	Choices     string
	Chooser     string
	ChoiceTitle string
	// MinChoiceAmount$ and ChoiceAmount$ (the pick's bounds).
	MinChoiceAmount ParamText
	ChoiceAmount    ParamText
	// ETB$ True: the counters land on the entering object.
	ETB bool
	// EachFromSource$, trimmed (the CounterType$ EachFromSource referent).
	EachFromSource string
	// ExistingCounter is CounterType$ ExistingCounter: not a counter kind but
	// "a kind the recipient already carries" (Animation Module, Powerful
	// Broker); EachExistingCounter is EachExistingCounter$ True ("for each
	// kind of counter on ..."), else one existing kind is chosen.
	ExistingCounter     bool
	EachExistingCounter bool
	// PutOnDefined$ (trimmed), PutOnEachOther$ (trimmed spec) and
	// OnlyNewKind$ True: the recipients and the skip rule of the
	// ExistingCounter + Choices$ shape (Aven Courier, Contractual Safeguard).
	PutOnDefined   string
	PutOnEachOther string
	OnlyNewKind    bool
	// CounterTypePerDefined$ True, RandomType$ True, a non-empty
	// ChooseDifferent$.
	CounterTypePerDefined bool
	RandomType            bool
	ChooseDifferent       bool
	// CounterNumPerDefined$, trimmed.
	CounterNumPerDefined string
	// RememberPut$ True, RememberCards$ True.
	RememberPut   bool
	RememberCards bool
	// RememberAmount is RememberAmount$ True: the placed COUNT transport
	// (one remembered entry per counter actually placed after replacement).
	RememberAmount bool
	// ForColor is ForColor$: the put runs once per colour W, U, B, R, G with
	// that colour bound as the source's chosen colour (Call the Spirit
	// Dragons).
	ForColor bool
	// UpTo is UpTo$ True: the amount per recipient is chosen in
	// [UpToMin, CounterNum$] on resolution. UpToMin is UpToMin$ (default 0).
	UpTo    bool
	UpToMin ParamText
	// CounterTypes$ (plural, distinct from CounterType$): one counter of each
	// listed kind. OneOfEach is its canonical list when every entry is a plain
	// counter kind; SpecialTypes names entries this build does not model as a
	// plain kind, named loudly at resolution. ChosenFromList is the
	// "ChosenFromList" entry (a kind chosen per recipient from TypeList$,
	// canonical kinds, Elspeth Resplendent); EachTypeSpec is the spec of an
	// "EachType_<spec>" entry that rides a Choices$ recipient pick (one pick
	// per distinct counter kind among the matching battlefield objects,
	// Exotic Pets).
	CounterTypes   string
	OneOfEach      []string
	SpecialTypes   []string
	ChosenFromList bool
	TypeList       []string
	EachTypeSpec   string

	// EntryFoldBlocked reports that the ability carries an asking or
	// per-recipient modifier the entry-counter fold must leave to the
	// ordinary body path (entryBodyAbsorbable): Optional$, Choices$,
	// Divided$, DividedAsYouChoose$, DividedRandomly$, RandomType$, Bolster$, Support$, Adapt$,
	// Monstrosity$, Renown$, CounterNumPerDefined$, CounterTypePerDefined$,
	// EachFromSource$ or PerDefined$, present with any value.
	EntryFoldBlocked bool

	// Unread are the parameters present on the ability that no PutCounter
	// reader consumes: the resolution Notes them once.
	Unread []string
}

// putCounterKnownKeys is every parameter key a PutCounter resolution consumes
// or deliberately ignores, sorted (changeZoneKnownKeys' contract): rules'
// TestPutCounterKnownKeysMatchTheCensus holds it equal to the parameter
// census's measured api:PutCounter read set plus its ignored and structural
// keys.
var putCounterKnownKeys = [...]string{
	"AILifeThreshold", "AILogic", "AIMinTgts", "AINoRecursiveCheck", "AIPhyrexianPayment",
	"AITgtOwnCards", "AITgts", "AIXMax", "Activation", "ActivationAfterBlockers", "ActivationFirstCombat",
	"ActivationGameTypes", "ActivationLimit", "ActivationPhases", "ActivationZone",
	"Activator", "Adapt",
	"AdditionalDesc", "AdditionalDescription", "Affected", "AlternateCost",
	"AlternativeCost", "Announce", "AnnounceTitle", "BecomeStartingPlayer", "Boast",
	"Bolster", "CantCopy", "ChangeNumDesc", "ChangeTypeDesc", "ChangeValidDesc", "CharacteristicDefining", "CheckSVar",
	"ChoiceAmount", "ChoiceDesc", "ChoiceTitle", "ChoiceTitleAppend", "ChoiceZone", "Choices", "ChooseDifferent",
	"ChooseFromList", "Chooser", "ClassBand", "ClearImprinted", "Condition",
	"ConditionActivationLimit", "ConditionCheckSVar", "ConditionCompare", "ConditionCompare2",
	"ConditionDefined", "ConditionDescription", "ConditionFirstCombat",
	"ConditionManaNotSpent", "ConditionManaSpent", "ConditionNotPresent", "ConditionOptionalPaid", "ConditionPhases", "ConditionPlayerTurn",
	"ConditionPresent", "ConditionPresent2", "ConditionSVarCompare", "ConditionZone", "CopyCard", "Cost", "CostDesc",
	"CounterMapValues", "CounterNum", "CounterNumPerDefined", "CounterType", "CounterTypePerDefined", "CounterTypes",
	"Defined", "DefinedCards", "DefinedDesc", "DefinedTarget", "Description", "Divided",
	"DividedAsYouChoose", "DividedRandomly", "ETB", "EachExistingCounter", "EachFromSource", "Exclude",
	"Exhaust", "ForColor", "ForgetOtherTargets", "FromDesc", "GameActivationLimit", "GiftDescription", "IfDesc", "Image", "ImprintCards", "ImprintPlayed",
	"InstantSpeed", "IsCurse", "IsPresent", "KW",
	"Keyword", "KeywordLine", "LockInText", "ManaConversion", "MaxTotalTargetCMC", "MaxTotalTargetPower", "Mentor",
	"MinChoiceAmount", "ModeCost", "Monstrosity", "NewController",
	"NoPutDesc", "NonBasicSpell", "OnlyNewKind", "OpponentTurn", "Optional", "Pawprint", "PerDefined",
	"Placer", "Planeswalker", "PlayCost", "PlayerTurn", "PowerUp", "PrecostDesc",
	"PresentCompare", "PresentDefined", "PresentZone", "PrimaryPrompt", "PutOnDefined", "PutOnEachOther", "RandomNumTargets", "RandomType", "ReduceAmount",
	"ReduceCost", "RememberAmount", "RememberAnimated", "RememberCards", "RememberCostMana", "RememberObjects",
	"RememberPut", "RememberTargets", "Renown", "ReplaceColor", "ReplaceGraveyard",
	"ReplaceGraveyardValid", "ReplaceMana", "ReplaceOnly", "ReplaceType",
	"ReturnAbility", "ReturnValid", "SVarCompare", "SelectPrompt", "SetChosenMode", "SetColor",
	"ShowCards", "SorcerySpeed", "SpecifyCounter", "SpellDescription", "StackDescription",
	"SubAbility", "Support", "TargetMax", "TargetMin",
	"TargetType", "TargetUnique", "TargetValidTargeting", "TargetingPlayer",
	"TargetingPlayerControls", "TargetsAtRandom", "TargetsForEachPlayer",
	"TargetsWithControllerProperty", "TargetsWithDefinedController",
	"TargetsWithDifferentCMC", "TargetsWithDifferentControllers",
	"TargetsWithDifferentNames", "TargetsWithEqualToughness",
	"TargetsWithRelatedProperty", "TargetsWithSameCardType", "TargetsWithSameController",
	"TargetsWithSameCreatureType", "TargetsWithSharedCardType",
	"TargetsWithSharedTypes", "TgtPrompt", "TgtZone", "TokenScript",
	"Tribute", "TriggerController", "TriggerDescription", "TriggeredCounterMap", "Type", "TypeList", "Ultimate",
	"UnlessAI", "UnlessCost", "UnlessPayer", "UnlessResolveSubs", "UnlessSwitched", "UpTo", "UpToMin",
	"ValidCard", "ValidCards", "ValidCardsDesc", "ValidChoices", "ValidCounterType",
	"ValidDescription", "ValidTgts", "ValidTgtsDesc", "VoteMessage",
	"WithoutManaCost", "XMax", "XMin",
}

// isPutCounterSA reports whether sa resolves as api:PutCounter.
func isPutCounterSA(sa *cards.SA) bool {
	return sa != nil && (sa.CompiledAPI() == cards.APIPutCounter || sa.API == "PutCounter")
}

// IsPutCounter reports whether sa resolves as api:PutCounter (the rules-side
// readers' gate before PutCounterOf).
func IsPutCounter(sa *cards.SA) bool { return isPutCounterSA(sa) }

// PutCounterOf returns sa's compiled PutCounter parameters: the configured
// record's when it is bound to sa's Params map, else the front cache's entry
// for that map, else a fresh compile stored in the front cache (ChangeZoneOf's
// contract).
func PutCounterOf(sa *cards.SA) *PutCounterParams {
	if f := LoadSAFacts(sa); f != nil {
		if p := f.PutCounter; p != nil && p.boundTo(sa.Params) {
			return p
		}
	}
	slot := &putCounterFront[paramMapSlot(sa.Params)]
	if p := slot.Load(); p != nil && p.boundTo(sa.Params) {
		return p
	}
	p := compilePutCounter(sa)
	if sa.Params != nil {
		slot.Store(p)
	}
	return p
}

// putCounterFront is PutCounterOf's direct-mapped front cache (czFront's
// shape).
var putCounterFront [1 << 10]atomic.Pointer[PutCounterParams]

// CounterNumResolved is CounterNum$ through NumResolved (default 1): the
// entry-counter fold's count, the same read the resolution makes.
func (p *PutCounterParams) CounterNumResolved(h Host, c *Ctx) (int32, bool) {
	return numResolvedText(h, c, p.CounterNum, 1)
}

// CounterNumValue is CounterNum$ through Num (default 1).
func (p *PutCounterParams) CounterNumValue(h Host, c *Ctx) int32 {
	return numText(h, c, p.CounterNum, 1)
}

// compilePutCounter is the one reader of a PutCounter ability's parameters.
func compilePutCounter(sa *cards.SA) *PutCounterParams {
	p := &PutCounterParams{paramBinding: bindParams(sa)}
	// Each key is read by its literal name (the parameter census attributes
	// a read by the key the call spells).
	num, numOK := sa.Param(cards.PKCounterNum)
	if strings.EqualFold(strings.TrimSpace(num), "PreventedDamage") {
		// A prevention rider's delayed body (Sacred Boon, Scars of the
		// Veteran): the registration's remembered number is the prevented
		// amount.
		num = "Count$TriggerRememberAmount"
	}
	p.CounterNum, p.CounterNumSet = paramTextSet(num, numOK)
	adapt, adaptOK := sa.Param(cards.PKAdapt)
	p.Adapt, p.AdaptSet = paramTextSet(adapt, adaptOK)
	mono, monoOK := sa.Param(cards.PKMonstrosity)
	p.Monstrosity, p.MonstrositySet = paramTextSet(mono, monoOK)
	renown, renownOK := sa.Param(cards.PKRenown)
	p.Renown, p.RenownSet = paramTextSet(renown, renownOK)

	p.CounterType = sa.ParamStr(cards.PKCounterType)
	p.Kind = canonicalCounterKind(p.CounterType)
	if p.Kind == "" {
		p.Kind = "P1P1"
	}
	kinds := splitCounterKinds(p.Kind)
	p.Kinds = kinds[:len(kinds):len(kinds)]
	// CounterTypes$ (plural) is a DIFFERENT key from CounterType$: one counter
	// of each listed kind, accumulating repeats (Scavenged Brawler's four P1P1
	// entries). An entry outside that grammar (ChosenFromList, EachType_*) is
	// kept in SpecialTypes so the resolution names it loudly rather than
	// silently dropping it. ChosenFromList and EachType_<spec> (with a
	// Choices$ pick) are the two modelled entries.
	choices, choicesOK := sa.Param(cards.PKChoices)
	p.CounterTypes = strings.TrimSpace(rawParamText(sa, "CounterTypes").Text)
	if p.CounterTypes != "" {
		for _, k := range splitCounterKinds(canonicalCounterKind(p.CounterTypes)) {
			if strings.EqualFold(k, "ChosenFromList") {
				p.ChosenFromList = true
				continue
			}
			if spec, ok := strings.CutPrefix(k, "EachType_"); ok && spec != "" && p.EachTypeSpec == "" && strings.TrimSpace(choices) != "" {
				p.EachTypeSpec = spec
				continue
			}
			if strings.Contains(k, "_") {
				p.SpecialTypes = append(p.SpecialTypes, k)
				continue
			}
			p.OneOfEach = append(p.OneOfEach, k)
		}
	}

	if p.ChosenFromList {
		p.TypeList = splitCounterKinds(canonicalCounterKind(rawParamText(sa, "TypeList").Text))
	}

	p.Placer = strings.TrimSpace(sa.ParamStr(cards.PKPlacer))
	triggeredCounterMapOK := rawParamText(sa, "TriggeredCounterMap").Present
	p.TriggeredCounterMap = triggeredCounterMapOK
	p.CounterMapValues = rawParamText(sa, "CounterMapValues")
	optional, optionalOK := sa.Param(cards.PKOptional)
	p.OptionalTrue = isTrue(optional)
	bolster, bolsterOK := sa.Param(cards.PKBolster)
	p.Bolster = ParamText{Text: bolster, Present: bolsterOK}
	support, supportOK := sa.Param(cards.PKSupport)
	p.Support = ParamText{Text: support, Present: supportOK}
	divided, dividedOK := dividedParam(sa)
	p.Divided = dividedOK
	dividedRandomly, dividedRandomlyOK := sa.Param(cards.PKDividedRandomly)
	p.DividedRandomly = isTrue(dividedRandomly)
	p.Choices = strings.TrimSpace(choices)
	p.Chooser = sa.ParamStr(cards.PKChooser)
	p.ChoiceTitle = sa.ParamStr(cards.PKChoiceTitle)
	p.MinChoiceAmount = rawParamText(sa, "MinChoiceAmount")
	p.ChoiceAmount = rawParamText(sa, "ChoiceAmount")
	p.ETB = isTrue(sa.ParamStr(cards.PKETB))
	eachFrom, eachFromOK := sa.Param(cards.PKEachFromSource)
	p.EachFromSource = strings.TrimSpace(eachFrom)
	p.ExistingCounter = strings.EqualFold(strings.TrimSpace(p.Kind), "ExistingCounter")
	p.EachExistingCounter = isTrue(sa.ParamStr(cards.PKEachExistingCounter))
	p.PutOnDefined = strings.TrimSpace(rawParamText(sa, "PutOnDefined").Text)
	p.PutOnEachOther = strings.TrimSpace(rawParamText(sa, "PutOnEachOther").Text)
	p.OnlyNewKind = isTrue(rawParamText(sa, "OnlyNewKind").Text)
	perType, perTypeOK := sa.Param(cards.PKCounterTypePerDefined)
	p.CounterTypePerDefined = isTrue(perType)
	random, randomOK := sa.Param(cards.PKRandomType)
	p.RandomType = isTrue(random)
	p.ChooseDifferent = strings.TrimSpace(sa.ParamStr(cards.PKChooseDifferent)) != ""
	perNum, perNumOK := sa.Param(cards.PKCounterNumPerDefined)
	p.CounterNumPerDefined = strings.TrimSpace(perNum)
	p.RememberPut = isTrue(sa.ParamStr(cards.PKRememberPut))
	p.RememberCards = isTrue(sa.ParamStr(cards.PKRememberCards))
	p.RememberAmount = isTrue(sa.ParamStr(cards.PKRememberAmount))
	p.ForColor = isTrue(rawParamText(sa, "ForColor").Text)
	p.UpTo = isTrue(sa.ParamStr(cards.PKUpTo))
	p.UpToMin = rawParamText(sa, "UpToMin")
	tribute, tributeOK := sa.Param(cards.PKTribute)
	p.Tribute = isTrue(tribute)

	p.EntryFoldBlocked = triggeredCounterMapOK || optionalOK || choicesOK || rawParamText(sa, "Divided").Present || divided.Present || dividedRandomlyOK ||
		randomOK || bolsterOK || supportOK || p.Adapt.Present || p.Monstrosity.Present || renownOK ||
		perNumOK || perTypeOK || eachFromOK || rawParamText(sa, "PerDefined").Present || tributeOK

	p.Unread = unreadKeys(sa, putCounterKnownKeys[:])
	return p
}

// rawParamText is one key outside the ParamKey vocabulary read as ParamText
// (Num's raw read shape); every call site spells its key as a literal, so the
// parameter census attributes each read to the caller.
func rawParamText(sa *cards.SA, key string) ParamText {
	v, ok := sa.Params[key]
	return ParamText{Text: v, Present: ok}
}

// paramTextSet is a raw parameter read as ParamText, and whether its trimmed
// text is non-empty.
func paramTextSet(v string, ok bool) (ParamText, bool) {
	return ParamText{Text: v, Present: ok}, strings.TrimSpace(v) != ""
}

// PutCounterKnownKeys is a copy of putCounterKnownKeys, for the census check.
func PutCounterKnownKeys() []string { return slices.Clone(putCounterKnownKeys[:]) }
