package effects

import (
	"slices"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// This file is api:Effect's parameter compiler, on ChangeZone's pattern
// (changezone_params.go). compileEffect is the
// ONLY reader of an Effect ability's own parameters: the resolution
// (effect.go), the rules-side target ask's granted-static scan
// (staticModesFromSVars' StaticAbilities$) and the opening-hand Effect
// (Triggers$, EffectOwner$) all read the compiled EffectParams.
// internal/codeshape's effectParamLeaks ratchet holds that: no parameter read
// in effect.go, and no read of an Effect-only key anywhere else in engine/rules/ or
// effects/.
//
// The conversion is a read move, not a semantic one: every field is the
// value the resolution read in place before, at the spelling (raw or
// trimmed) it read. The Triggers$ bodies an Effect arms are parsed per
// resolution from SVar text; their own parameters are read once per body by
// readEffectTriggerLine below. The shared readers the resolution calls with
// the ability -- RememberObjects$ (effectRemembered,
// effectRememberedPlayers), ForgetOtherRemembered$ (forgetOtherRemembered)
// and the restriction/static-line registration gates -- still read their own
// keys; those are the next tier.

// EffectParams is one Effect ability's parameters, compiled once.
type EffectParams struct {
	// paramBinding is the Params map the struct was compiled from
	// (typed_params.go's identity rule).
	paramBinding

	// Duration is Duration$ as written ("" when absent: the absent-duration
	// rules key on that).
	Duration string
	// StaticAbilities$, Triggers$ and ReplacementEffects$, as written (SVar
	// name lists).
	StaticAbilities    string
	Triggers           string
	ReplacementEffects string
	// TriggerNames is Triggers$ split into its SVar names: Forge writes the
	// list comma-separated (Tawnos's Coffin's `LeavesPlay,Untap`), some
	// scripts space-separated.
	TriggerNames []string
	// NoteCounterDefined$ (Tawnos's Coffin's `Targeted`), trimmed: the
	// objects whose counters the Effect notes for a later WithNotedCounters$.
	NoteCounterDefined string
	// Name$, trimmed.
	Name string
	// NotStackable is Stackable$ False.
	NotStackable bool
	// ForgetOnMoved$, ExileOnMoved$ and ForgetCounter$, trimmed.
	ForgetOnMoved string
	ExileOnMoved  string
	ForgetCounter string
	// ForgetOnCast is ForgetOnCast$, trimmed, with Forge's explicit False
	// folded to "".
	ForgetOnCast string
	// ImprintOnHost$, trimmed, and whether it is True.
	ImprintOnHost     string
	ImprintOnHostTrue bool
	// ForgetOnPhasedIn$ True.
	ForgetOnPhasedIn bool
	// ImprintCards$, trimmed: the capture an api:Effect's staticAbilities$
	// grants read as their own memory (Garland, Royal Kidnapper's
	// `ImprintCards$ Targeted` pairs with the GainControl static's
	// Affected$ Card.IsImprinted). The shared readers resolve the value
	// against this resolution's targets exactly as effectRemembered resolves
	// RememberObjects$.
	ImprintCards string
	// RememberLKI$, SetChosenNumber$ and EffectOwner$, trimmed.
	RememberLKI     string
	SetChosenNumber string
	EffectOwner     string
	Boon            bool
	// RemembersObjects is RememberObjects$ present and non-blank: the
	// Effect's own trigger bodies then read its capture, not the resolving
	// spell's memory.
	RemembersObjects bool

	// Unread are the parameters present on the ability that no Effect
	// reader consumes: the resolution Notes them once.
	Unread []string
}

// effectKnownKeyRows is every parameter key an Effect resolution consumes or
// deliberately ignores (changeZoneKnownKeys' contract): rules'
// TestEffectKnownKeysMatchTheCensus holds the normalized table equal to the
// parameter census's measured api:Effect read set plus its ignored and
// structural keys.
//
// The rows are not trusted to be sorted or duplicate-free: engine/effects/
// *_params.go is merge=union, so rebasing this branch over a main-side key
// addition leaves both rows in the source. normalizeKnownKeys computes the
// union the table is meant to be.
var effectKnownKeyRows = [...]string{
	"AILifeThreshold", "AILogic", "AIMinTgts", "AINoRecursiveCheck", "AIPhyrexianPayment",
	"AITgtOwnCards", "AITgts", "AIXMax", "Activation", "ActivationAfterBlockers", "ActivationFirstCombat",
	"ActivationGameTypes", "ActivationLimit", "ActivationPhases", "ActivationZone",
	"Activator",
	"AdditionalDesc", "AdditionalDescription", "Affected", "AlternateCost",
	"AlternativeCost", "Announce", "AnnounceTitle", "BecomeStartingPlayer", "Boast", "Boon", "BoonAmount",
	"CantCopy", "ChangeNumDesc", "ChangeTypeDesc", "ChangeValidDesc", "CharacteristicDefining", "CheckSVar", "ChoiceDesc", "ChoiceTitle",
	"ChoiceTitleAppend", "ChoiceZone", "Choices", "ChooseFromList", "ClassBand", "ClearImprinted",
	"Condition", "ConditionActivationLimit", "ConditionCheckSVar", "ConditionCompare", "ConditionCompare2",
	"ConditionDefined", "ConditionDescription", "ConditionFirstCombat",
	"ConditionManaNotSpent", "ConditionManaSpent", "ConditionNotPresent", "ConditionOpponentTurn", "ConditionOptionalPaid", "ConditionPhases", "ConditionPlayerContains", "ConditionPlayerDefined", "ConditionPlayerTurn",
	"ConditionPresent", "ConditionPresent2", "ConditionSVarCompare", "ConditionTargetValidTargeting", "ConditionTargetsSingleTarget", "ConditionZone", "CopyCard", "Cost", "CostDesc",
	"Defined", "DefinedCards", "DefinedDesc", "DefinedTarget", "Description", "Duration",
	"EffectOwner", "EffectZone", "Exclude", "ExcludeZone", "Execute", "Exhaust", "ExileOnLost", "ExileOnMoved",
	"FirstCombat", "ForgetCounter", "ForgetOnCast", "ForgetOnMoved", "ForgetOnPhasedIn",
	"ForgetOtherRemembered", "ForgetOtherTargets", "FromDesc", "GameActivationLimit", "GiftDescription", "IfDesc", "Image", "ImprintCards",
	"ImprintOnHost", "ImprintPlayed", "InstantSpeed", "IsCurse",
	"IsPresent", "KW", "Keyword", "KeywordLine", "LockInText", "ManaConversion", "MaxTotalTargetCMC",
	"MaxTotalTargetPower", "Mentor", "ModeCost", "Monstrosity", "Name",
	"NewController", "NoPutDesc", "NoShuffle", "NonBasicSpell", "NoteCounterDefined", "OneOff", "OpponentTurn", "OptionalDecider", "Origin", "Pawprint", "Phase",
	"Planeswalker", "PlayCost", "PlayerTurn", "PowerUp", "PrecostDesc",
	"PresentCompare", "PresentDefined", "PresentZone", "PrimaryPrompt", "RaiseCost", "RandomNumTargets", "ReduceAmount", "ReduceCost",
	"RememberAnimated", "RememberCostMana", "RememberLKI", "RememberObjects", "RememberTargets", "ReplaceColor",
	"ReplaceGraveyard", "ReplaceGraveyardValid", "ReplaceMana", "ReplaceOnly",
	"ReplaceType", "ReplacementEffects", "ReturnAbility", "ReturnValid", "SVarCompare",
	"SelectPrompt", "SetChosenMode", "SetChosenNumber", "SetColor", "ShowCards",
	"SorcerySpeed", "SpellDescription", "StackDescription", "Stackable", "Static",
	"StaticAbilities", "SubAbility", "TargetMax", "TargetMin", "TargetType",
	"TargetUnique", "TargetValidTargeting", "TargetingPlayer",
	"TargetingPlayerControls", "TargetsAtRandom", "TargetsForEachPlayer",
	"TargetsWithControllerProperty", "TargetsWithDefinedController",
	"TargetsWithDifferentCMC", "TargetsWithDifferentControllers",
	"TargetsWithDifferentNames", "TargetsWithEqualToughness",
	"TargetsWithRelatedProperty", "TargetsWithSameCardType", "TargetsWithSameController",
	"TargetsWithSameCreatureType", "TargetsWithSharedCardType",
	"TargetsWithSharedTypes", "TgtPrompt", "TgtZone", "ThisTurn", "TokenScript",
	"TriggerController", "TriggerDescription", "Triggers", "Type", "Ultimate",
	"UnlessAI", "UnlessCost", "UnlessPayer", "UnlessResolveSubs", "UnlessSwitched",
	"ValidAfterStack", "ValidCard", "ValidCards", "ValidCardsDesc", "ValidChoices", "ValidCounterType",
	"ValidDescription", "ValidPlayer", "ValidSource", "ValidTgts", "ValidTgtsDesc", "VoteMessage", "WithoutManaCost", "XColor", "XMax", "XMin",
}

// effectKnownKeys is effectKnownKeyRows as the sorted, duplicate-free set
// unreadKeys binary-searches and the census check compares. The normalization
// is what makes a union merge of two key additions mean their union instead of
// a duplicated key.
var effectKnownKeys = normalizeKnownKeys(effectKnownKeyRows[:])

// normalizeKnownKeys returns the sorted union of keys.
func normalizeKnownKeys(keys []string) []string {
	out := slices.Clone(keys)
	slices.Sort(out)
	return slices.Compact(out)
}

// isEffectSA reports whether sa resolves as api:Effect.
func isEffectSA(sa *cards.SA) bool {
	return sa != nil && (sa.CompiledAPI() == cards.APIEffect || sa.API == "Effect")
}

// EffectOf returns sa's compiled Effect parameters: the configured record's
// when it is bound to sa's Params map, else the front cache's entry for that
// map, else a fresh compile stored in the front cache (ChangeZoneOf's
// contract).
func EffectOf(sa *cards.SA) *EffectParams {
	if f := LoadSAFacts(sa); f != nil {
		if p := f.Effect; p != nil && p.boundTo(sa.Params) {
			return p
		}
	}
	slot := &effectFront[paramMapSlot(sa.Params)]
	if p := slot.Load(); p != nil && p.boundTo(sa.Params) {
		return p
	}
	p := compileEffect(sa)
	if sa.Params != nil {
		slot.Store(p)
	}
	return p
}

// effectFront is EffectOf's direct-mapped front cache (czFront's shape).
var effectFront [1 << 10]atomic.Pointer[EffectParams]

// compileEffect is the one reader of an Effect ability's parameters.
func compileEffect(sa *cards.SA) *EffectParams {
	p := &EffectParams{paramBinding: bindParams(sa)}
	p.Duration = sa.ParamStr(cards.PKDuration)
	p.StaticAbilities = sa.ParamStr(cards.PKStaticAbilities)
	p.Triggers = sa.ParamStr(cards.PKTriggers)
	p.TriggerNames = strings.FieldsFunc(p.Triggers, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})
	p.ReplacementEffects = sa.ParamStr(cards.PKReplacementEffects)
	p.NoteCounterDefined = strings.TrimSpace(sa.ParamStr(cards.PKNoteCounterDefined))
	p.Name = strings.TrimSpace(sa.ParamStr(cards.PKName))
	if stackable, present := sa.Param(cards.PKStackable); present && strings.EqualFold(strings.TrimSpace(stackable), "False") {
		p.NotStackable = true
	}
	p.ForgetOnMoved = strings.TrimSpace(sa.ParamStr(cards.PKForgetOnMoved))
	p.ExileOnMoved = strings.TrimSpace(sa.ParamStr(cards.PKExileOnMoved))
	p.ForgetCounter = strings.TrimSpace(sa.ParamStr(cards.PKForgetCounter))
	p.ForgetOnCast = strings.TrimSpace(sa.ParamStr(cards.PKForgetOnCast))
	if strings.EqualFold(p.ForgetOnCast, "False") {
		p.ForgetOnCast = ""
	}
	p.ImprintOnHost = strings.TrimSpace(sa.ParamStr(cards.PKImprintOnHost))
	p.ImprintOnHostTrue = strings.EqualFold(p.ImprintOnHost, "True")
	p.ImprintCards = strings.TrimSpace(sa.ParamStr(cards.PKImprintCards))
	p.ForgetOnPhasedIn = isTrue(sa.ParamStr(cards.PKForgetOnPhasedIn))
	p.RememberLKI = strings.TrimSpace(sa.ParamStr(cards.PKRememberLKI))
	p.SetChosenNumber = strings.TrimSpace(sa.ParamStr(cards.PKSetChosenNumber))
	p.EffectOwner = strings.TrimSpace(sa.ParamStr(cards.PKEffectOwner))
	p.Boon = isTrue(sa.ParamStr(cards.PKBoon))
	p.RemembersObjects = strings.TrimSpace(sa.ParamStr(cards.PKRememberObjects)) != ""
	// NoShuffle$ is the opt-out twopiles' noShuffleBody forces onto EVERY pile
	// body (a pile split is not a search, so the shared library-origin tail's
	// CR 701.23d shuffle must be suppressed). An Effect body has no library
	// movement to suppress, so the key is meaningless here; the recognition
	// read documents it so the census never flags it unread (Fight or Flight's
	// ChosenPile$ DBEffect is the carrier).
	_ = sa.ParamStr(cards.PKNoShuffle)
	p.Unread = unreadKeys(sa, effectKnownKeys[:])
	return p
}

// effectTriggerLine is one Triggers$ body's parameters as the Effect
// resolution arms it (read once per parsed body, on the stack).
type effectTriggerLine struct {
	// Execute$, OptionalDecider$ and ThisTurn$, trimmed.
	Execute         string
	OptionalDecider string
	ThisTurn        string
	// OneOff is OneOff$ True; Static is Static$ True.
	OneOff bool
	Static bool
	// Phase is Phase$ as written; ValidPlayer$ trimmed.
	Phase       string
	ValidPlayer string
	// FirstCombat is FirstCombat$ True on a Phase trigger.
	FirstCombat bool
	// CheckSVar$ and SVarCompare$, trimmed: a Phase trigger's "if" condition.
	CheckSVar   string
	SVarCompare string
	// BoonAmount is BoonAmount$ as a literal: how many firings a boon's trigger
	// lasts. Zero means once.
	BoonAmount int
	// IsPresent$ / PresentZone$ / PresentCompare$, trimmed: a Phase
	// registration's presence condition, carried into the registration Text
	// as the |IP=/|PZ=/|PC= suffixes rules' delayed scan decodes (Scars of
	// the Veteran's IsPresent$ Creature.IsRemembered).
	IsPresent      string
	PresentZone    string
	PresentCompare string
	// ValidSource is ValidSource$, trimmed: who a damage-mode body names as
	// the dealer.
	ValidSource string
	// SourceLeave is a ChangesZone trigger watching the Effect's own source
	// leave the battlefield (`Origin$ Battlefield`, `ValidCard$
	// Card.EffectSource`): the one-shot delayed promise CR 603.7b describes,
	// whose trigger event is the departure that would also end a
	// source-bounded Duration$ Permanent registration.
	SourceLeave bool
}

// readEffectTriggerLine reads one parsed Triggers$ body's parameters.
func readEffectTriggerLine(tr *cards.Trigger) effectTriggerLine {
	boonAmount, _ := strconv.Atoi(strings.TrimSpace(tr.ParamStr(cards.PKBoonAmount)))
	return effectTriggerLine{
		BoonAmount:      boonAmount,
		Execute:         strings.TrimSpace(tr.ParamStr(cards.PKExecute)),
		OptionalDecider: strings.TrimSpace(tr.ParamStr(cards.PKOptionalDecider)),
		ThisTurn:        strings.TrimSpace(tr.ParamStr(cards.PKThisTurn)),
		OneOff:          isTrue(tr.ParamStr(cards.PKOneOff)),
		Static:          isTrue(tr.ParamStr(cards.PKStatic)),
		Phase:           tr.ParamStr(cards.PKPhase),
		ValidPlayer:     strings.TrimSpace(tr.ParamStr(cards.PKValidPlayer)),
		FirstCombat:     isTrue(tr.ParamStr(cards.PKFirstCombat)),
		CheckSVar:       strings.TrimSpace(tr.ParamStr(cards.PKCheckSVar)),
		SVarCompare:     strings.TrimSpace(tr.ParamStr(cards.PKSVarCompare)),
		IsPresent:       strings.TrimSpace(tr.ParamStr(cards.PKIsPresent)),
		PresentZone:     strings.TrimSpace(tr.ParamStr(cards.PKPresentZone)),
		PresentCompare:  strings.TrimSpace(tr.ParamStr(cards.PKPresentCompare)),
		ValidSource:     strings.TrimSpace(tr.ParamStr(cards.PKValidSource)),
		SourceLeave:     WatchesEffectSourceLeave(tr),
	}
}

// WatchesEffectSourceLeave reports whether the trigger watches its own
// Effect source leave the battlefield: Forge's
// `Mode$ ChangesZone | Origin$ Battlefield | Destination$ Any | ValidCard$
// Card.EffectSource`. An api:Effect Triggers$ body of this shape is the
// CR 603.7 delayed "when EFFECTSOURCE leaves" promise, which triggers once on
// that departure (Mysterio, Master of Illusion; Stangg).
func WatchesEffectSourceLeave(t *cards.Trigger) bool {
	if t == nil || t.ModeKind() != cards.TriggerChangesZone {
		return false
	}
	z, ok := ParseZoneWord(t.ParamStr(cards.PKOrigin))
	return ok && z == state.ZBattlefield && strings.Contains(t.ParamStr(cards.PKValidCard), "Card.EffectSource")
}

// EffectKnownKeys is a copy of effectKnownKeys, for the census check.
func EffectKnownKeys() []string { return slices.Clone(effectKnownKeys[:]) }

// isEmblemEffectName reports whether an api:Effect's Name$ names an emblem
// (CR 114): the corpus spells every emblem-creating Effect
// `Name$ Emblem — <name>` (115 raw lines, all Duration$ Permanent).
func isEmblemEffectName(name string) bool {
	return strings.HasPrefix(name, "Emblem")
}

// effectStaticSuffix is the bare "|ST" flag a Static$ True Effect Triggers$
// body carries on its DelayedRegister text, so events.Apply sets
// DelayedTrigger.Static and rules resolves it inline (CR 611.2a duration
// bookkeeping, never a stack object). Every arm places it before
// |BN=/|EF/|RN= and before |CN=/|CC=: the decode strips those first, and a
// |ST after them would be swallowed by their LastIndex.
func effectStaticSuffix(tl effectTriggerLine) string {
	if tl.Static {
		return "|ST"
	}
	return ""
}

// effectPhaseFlags is a Phase Triggers$ body's bare flags: "|1C" for
// FirstCombat$ True, then effectStaticSuffix's "|ST". The decode strips |ST
// before |1C, so the order is fixed.
func effectPhaseFlags(tl effectTriggerLine) string {
	flags := ""
	if tl.FirstCombat {
		flags = "|1C"
	}
	return flags + effectStaticSuffix(tl)
}
