package kgen

import (
	"reflect"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Symbolic names for kop's enum and mask types, so the generated code reads
// as the vocabulary rather than ordinals. Every entry names the constant it
// stands for, so a renamed or removed constant breaks the build here; a
// constant with no entry panics in writeLit rather than emitting an ordinal.

type nv struct {
	name string
	v    uint64
}

var enumNames = map[reflect.Type][]nv{}
var maskNames = map[reflect.Type][]nv{}

func enum[T ~uint8 | ~uint16 | ~int16](pairs ...any) {
	var zero T
	t := reflect.TypeOf(zero)
	for i := 0; i < len(pairs); i += 2 {
		enumNames[t] = append(enumNames[t], nv{pairs[i].(string), uint64(pairs[i+1].(T))})
	}
}

func mask[T ~uint8 | ~uint16 | ~uint32](pairs ...any) {
	var zero T
	t := reflect.TypeOf(zero)
	for i := 0; i < len(pairs); i += 2 {
		maskNames[t] = append(maskNames[t], nv{pairs[i].(string), uint64(pairs[i+1].(T))})
	}
}

func init() {
	for o := kop.OpNop; o < kop.OpHighEnd; o++ { // the frozen list, registered ops, then kop/ops.go's high ops
		if !o.Known() {
			continue
		}
		enumNames[reflect.TypeOf(o)] = append(enumNames[reflect.TypeOf(o)], nv{"kop.Op" + o.String(), uint64(o)})
	}
	enum[kop.Ref]("kop.RefNone", kop.RefNone, "kop.RefTarget0", kop.RefTarget0, "kop.RefTarget1", kop.RefTarget1,
		"kop.RefTarget2", kop.RefTarget2, "kop.RefTarget3", kop.RefTarget3, "kop.RefTarget4", kop.RefTarget4, "kop.RefSelf", kop.RefSelf, "kop.RefController", kop.RefController,
		"kop.RefOpponent", kop.RefOpponent, "kop.RefEachPlayer", kop.RefEachPlayer, "kop.RefEachOpponent", kop.RefEachOpponent,
		"kop.RefTriggerObj", kop.RefTriggerObj, "kop.RefTriggerPlayer", kop.RefTriggerPlayer, "kop.RefAttached", kop.RefAttached,
		"kop.RefLastCreated", kop.RefLastCreated, "kop.RefChosen", kop.RefChosen, "kop.RefTargetController", kop.RefTargetController,
		"kop.RefDefendingPlayer", kop.RefDefendingPlayer, "kop.RefEach", kop.RefEach, "kop.RefTargetOwner", kop.RefTargetOwner,
		"kop.RefRemembered", kop.RefRemembered, "kop.RefDelayedRemembered", kop.RefDelayedRemembered,
		"kop.RefReplaced", kop.RefReplaced, "kop.RefTriggerSource", kop.RefTriggerSource, "kop.RefSacrificed", kop.RefSacrificed,
		"kop.RefHost", kop.RefHost, "kop.RefTargetSlot", kop.RefTargetSlot, "kop.RefTargets01", kop.RefTargets01,
		"kop.RefTargets012", kop.RefTargets012, "kop.RefTargets12", kop.RefTargets12, "kop.RefTargets0123", kop.RefTargets0123,
		"kop.RefRememberedController", kop.RefRememberedController, "kop.RefDiscarded", kop.RefDiscarded, "kop.RefTgtCtrl1", kop.RefTgtCtrl1, "kop.RefTgtCtrl2", kop.RefTgtCtrl2, "kop.RefLibTop", kop.RefLibTop,
		"kop.RefOppNonRemCtrl", kop.RefOppNonRemCtrl, "kop.RefSrcOwner", kop.RefSrcOwner, "kop.RefPromised", kop.RefPromised, "kop.RefTriggerAttackers", kop.RefTriggerAttackers,
		"kop.RefRememberedOwner", kop.RefRememberedOwner, "kop.RefImprinted", kop.RefImprinted, "kop.RefTriggerTgtCtrl", kop.RefTriggerTgtCtrl, "kop.RefAttachedCtrl", kop.RefAttachedCtrl)
	enum[kop.VKind]("kop.VConst", kop.VConst, "kop.VX", kop.VX, "kop.VCountFilter", kop.VCountFilter,
		"kop.VLastDamage", kop.VLastDamage, "kop.VTargetPower", kop.VTargetPower, "kop.VSelfPower", kop.VSelfPower,
		"kop.VCountersSelf", kop.VCountersSelf, "kop.VCardsInHand", kop.VCardsInHand, "kop.VTriggerAmount", kop.VTriggerAmount,
		"kop.VNegX", kop.VNegX, "kop.VLifeGainedTurn", kop.VLifeGainedTurn, "kop.VTargetToughness", kop.VTargetToughness,
		"kop.VSelfToughness", kop.VSelfToughness, "kop.VCountersTarget", kop.VCountersTarget, "kop.VExpr", kop.VExpr)
	enum[kop.TrigEvent]("kop.TrigNone", kop.TrigNone, "kop.TrigETB", kop.TrigETB, "kop.TrigDies", kop.TrigDies,
		"kop.TrigLeaves", kop.TrigLeaves, "kop.TrigToGraveyard", kop.TrigToGraveyard, "kop.TrigAttacks", kop.TrigAttacks,
		"kop.TrigAttackersDeclared", kop.TrigAttackersDeclared, "kop.TrigPhase", kop.TrigPhase, "kop.TrigSpellCast", kop.TrigSpellCast,
		"kop.TrigLifeGained", kop.TrigLifeGained, "kop.TrigLifeLost", kop.TrigLifeLost, "kop.TrigDamageDone", kop.TrigDamageDone,
		"kop.TrigDamageDealtOnce", kop.TrigDamageDealtOnce, "kop.TrigBecomesTarget", kop.TrigBecomesTarget,
		"kop.TrigDrawn", kop.TrigDrawn, "kop.TrigDiscarded", kop.TrigDiscarded, "kop.TrigCounterAddedOnce", kop.TrigCounterAddedOnce,
		"kop.TrigUntaps", kop.TrigUntaps, "kop.TrigBlocks", kop.TrigBlocks, "kop.TrigBecomesBlocked", kop.TrigBecomesBlocked,
		"kop.TrigSacrificed", kop.TrigSacrificed, "kop.TrigManaExpend", kop.TrigManaExpend, "kop.TrigTaps", kop.TrigTaps,
		"kop.TrigGiveGift", kop.TrigGiveGift, "kop.TrigProliferate", kop.TrigProliferate, "kop.TrigFacesDilemma", kop.TrigFacesDilemma, "kop.TrigBecomeMonarch", kop.TrigBecomeMonarch, "kop.TrigTokenCreated", kop.TrigTokenCreated,
		"kop.TrigBecomeMonstrous", kop.TrigBecomeMonstrous, "kop.TrigTurnFaceUp", kop.TrigTurnFaceUp, "kop.TrigTransformed", kop.TrigTransformed, "kop.TrigCommitCrime", kop.TrigCommitCrime, "kop.TrigUnlockDoor", kop.TrigUnlockDoor, "kop.TrigFullyUnlock", kop.TrigFullyUnlock,
		"kop.TrigBecomesPlotted", kop.TrigBecomesPlotted, "kop.TrigBecomesSaddled", kop.TrigBecomesSaddled, "kop.TrigAttached", kop.TrigAttached, "kop.TrigDiscardedAll", kop.TrigDiscardedAll, "kop.TrigAbilityCast", kop.TrigAbilityCast, "kop.TrigScry", kop.TrigScry, "kop.TrigSurveil", kop.TrigSurveil, "kop.TrigExplores", kop.TrigExplores, "kop.TrigCollectEvidence", kop.TrigCollectEvidence,
		"kop.TrigLandPlayed", kop.TrigLandPlayed, "kop.TrigSearchedLibrary", kop.TrigSearchedLibrary, "kop.TrigForage", kop.TrigForage, "kop.TrigDiscover", kop.TrigDiscover,
		"kop.TrigManifestDread", kop.TrigManifestDread, "kop.TrigEvolved", kop.TrigEvolved)
	enum[kop.StaticKind]("kop.StatNone", kop.StatNone, "kop.StatContinuous", kop.StatContinuous,
		"kop.StatReduceCost", kop.StatReduceCost, "kop.StatCantBlock", kop.StatCantBlock, "kop.StatCantBlockBy", kop.StatCantBlockBy,
		"kop.StatAlternativeCost", kop.StatAlternativeCost, "kop.StatCastWithFlash", kop.StatCastWithFlash,
		"kop.StatMustAttack", kop.StatMustAttack, "kop.StatCantAttack", kop.StatCantAttack, "kop.StatIncreaseCost", kop.StatIncreaseCost, "kop.StatCantBeCast", kop.StatCantBeCast,
		"kop.StatCDAPT", kop.StatCDAPT, "kop.StatNoMaxHandSize", kop.StatNoMaxHandSize, "kop.StatExtraLandPlay", kop.StatExtraLandPlay,
		"kop.StatMayPlayFrom", kop.StatMayPlayFrom, "kop.StatOverride", kop.StatOverride,
		"kop.StatBeamGrant", kop.StatBeamGrant, "kop.StatFbGrant", kop.StatFbGrant, "kop.StatWarpGrant", kop.StatWarpGrant, "kop.StatEscapeGrant", kop.StatEscapeGrant, "kop.StatCantBeBeamedUp", kop.StatCantBeBeamedUp, "kop.StatPlayerHexproof", kop.StatPlayerHexproof, "kop.StatControl", kop.StatControl, "kop.StatPanharmonicon", kop.StatPanharmonicon, "kop.StatCanAttackDefender", kop.StatCanAttackDefender, "kop.StatReduceAbility", kop.StatReduceAbility, "kop.StatStackKw", kop.StatStackKw, "kop.StatMaxBlockers", kop.StatMaxBlockers, "kop.StatCantActivate", kop.StatCantActivate, "kop.StatDisableETB", kop.StatDisableETB, "kop.StatUntapOther", kop.StatUntapOther, "kop.StatCantGainLife", kop.StatCantGainLife, "kop.StatCantDraw", kop.StatCantDraw, "kop.StatCombatDmgToughness", kop.StatCombatDmgToughness, "kop.StatCantPutCounter", kop.StatCantPutCounter, "kop.StatTapExtra", kop.StatTapExtra, "kop.StatGoad", kop.StatGoad, "kop.StatCantTarget", kop.StatCantTarget, "kop.StatCantTgtGrave", kop.StatCantTgtGrave, "kop.StatCantSacrifice", kop.StatCantSacrifice, "kop.StatCantPhaseIn", kop.StatCantPhaseIn, "kop.StatCantPreventDamage", kop.StatCantPreventDamage, "kop.StatAnyTypeCreatures", kop.StatAnyTypeCreatures, "kop.StatHexproofOwnColors", kop.StatHexproofOwnColors, "kop.StatAsIfHaste", kop.StatAsIfHaste, "kop.StatCantBeSuspected", kop.StatCantBeSuspected, "kop.StatTapPower", kop.StatTapPower, "kop.StatFlipCoinMod", kop.StatFlipCoinMod, "kop.StatActivationLimit", kop.StatActivationLimit, "kop.StatPlayerProtAll", kop.StatPlayerProtAll, "kop.StatAttackTax", kop.StatAttackTax, "kop.StatLure", kop.StatLure)
	enum[kop.ReplKind]("kop.ReplNone", kop.ReplNone, "kop.ReplEntersTapped", kop.ReplEntersTapped,
		"kop.ReplEntersWithCounters", kop.ReplEntersWithCounters, "kop.ReplCantBeCountered", kop.ReplCantBeCountered,
		"kop.ReplDoesntUntap", kop.ReplDoesntUntap, "kop.ReplEntersAsChoice", kop.ReplEntersAsChoice,
		"kop.ReplEntersProgram", kop.ReplEntersProgram, "kop.ReplOthersEnterProgram", kop.ReplOthersEnterProgram,
		"kop.ReplCantLoseGame", kop.ReplCantLoseGame, "kop.ReplOppsCantWinGame", kop.ReplOppsCantWinGame,
		"kop.ReplDoubleDamageToOpps", kop.ReplDoubleDamageToOpps, "kop.ReplDamagePlus", kop.ReplDamagePlus, "kop.ReplPreventNoncombatToOthers", kop.ReplPreventNoncombatToOthers,
		"kop.ReplDoubleTokens", kop.ReplDoubleTokens, "kop.ReplDoubleCounters", kop.ReplDoubleCounters,
		"kop.ReplGraveToLibrary", kop.ReplGraveToLibrary, "kop.ReplExileOnLeave", kop.ReplExileOnLeave,
		"kop.ReplDoubleCountersOn", kop.ReplDoubleCountersOn, "kop.ReplAddCountersOn", kop.ReplAddCountersOn, "kop.ReplOnePoison", kop.ReplOnePoison, "kop.ReplDoubleCountersYou", kop.ReplDoubleCountersYou,
		"kop.ReplOthersDoesntUntap", kop.ReplOthersDoesntUntap, "kop.ReplLifeDouble", kop.ReplLifeDouble, "kop.ReplLifeLossDouble", kop.ReplLifeLossDouble,
		"kop.ReplLifePlus", kop.ReplLifePlus, "kop.ReplTurnUpProgram", kop.ReplTurnUpProgram, "kop.ReplCantTurnUp", kop.ReplCantTurnUp,
		"kop.ReplSpellsCantBeCountered", kop.ReplSpellsCantBeCountered, "kop.ReplAnyUntap", kop.ReplAnyUntap, "kop.ReplDrawMore", kop.ReplDrawMore, "kop.ReplRewriteTokens", kop.ReplRewriteTokens, "kop.ReplMayNotUntap", kop.ReplMayNotUntap, "kop.ReplDestroyHeal", kop.ReplDestroyHeal, "kop.ReplDiscardOppToBattlefield", kop.ReplDiscardOppToBattlefield, "kop.ReplPayBeforeETB", kop.ReplPayBeforeETB, "kop.ReplPreventMillOpps", kop.ReplPreventMillOpps, "kop.ReplAddToken", kop.ReplAddToken, "kop.ReplPreventDamageCounters", kop.ReplPreventDamageCounters, "kop.ReplDamageHealFirst", kop.ReplDamageHealFirst, "kop.ReplCopyFirstToken", kop.ReplCopyFirstToken)
	enum[kop.AbilityKind]("kop.AbSpell", kop.AbSpell, "kop.AbActivated", kop.AbActivated, "kop.AbMana", kop.AbMana,
		"kop.AbTriggered", kop.AbTriggered, "kop.AbStatic", kop.AbStatic, "kop.AbReplacement", kop.AbReplacement,
		"kop.AbAltCost", kop.AbAltCost)
	enum[kop.Zone]("kop.ZoneNone", kop.ZoneNone, "kop.ZoneLibrary", kop.ZoneLibrary, "kop.ZoneHand", kop.ZoneHand,
		"kop.ZoneBattlefield", kop.ZoneBattlefield, "kop.ZoneGraveyard", kop.ZoneGraveyard, "kop.ZoneExile", kop.ZoneExile,
		"kop.ZoneStack", kop.ZoneStack, "kop.ZoneCommand", kop.ZoneCommand,
		"kop.ZonePhased", kop.ZonePhased, "kop.ZonePhasedInd", kop.ZonePhasedInd, "kop.ZoneSideboard", kop.ZoneSideboard,
		"kop.ZoneAny", kop.ZoneAny)
	enum[kop.Timing]("kop.TimingSorcery", kop.TimingSorcery, "kop.TimingInstant", kop.TimingInstant, "kop.TimingLand", kop.TimingLand)
	enum[kop.TargetKind]("kop.TgtNone", kop.TgtNone, "kop.TgtObject", kop.TgtObject, "kop.TgtPlayer", kop.TgtPlayer,
		"kop.TgtAny", kop.TgtAny, "kop.TgtSpell", kop.TgtSpell, "kop.TgtCard", kop.TgtCard, "kop.TgtStackObj", kop.TgtStackObj, "kop.TgtStackAb", kop.TgtStackAb,
		"kop.TgtPermOrSpell", kop.TgtPermOrSpell, "kop.TgtCopyAb", kop.TgtCopyAb, "kop.TgtCopyTrig", kop.TgtCopyTrig,
		"kop.TgtStackObjTrig", kop.TgtStackObjTrig, "kop.TgtSpellTgting", kop.TgtSpellTgting, "kop.TgtPermOrGraveCard", kop.TgtPermOrGraveCard, "kop.TgtGraveExileCard", kop.TgtGraveExileCard)
	enum[kop.CtrlSel]("kop.CtrlAny", kop.CtrlAny, "kop.CtrlYou", kop.CtrlYou, "kop.CtrlOpp", kop.CtrlOpp, "kop.CtrlEachPl", kop.CtrlEachPl, "kop.CtrlHitBySrc", kop.CtrlHitBySrc)
	enum[kop.Pred]("kop.PredNone", kop.PredNone, "kop.PredPowerLE", kop.PredPowerLE, "kop.PredPowerGE", kop.PredPowerGE,
		"kop.PredToughnessLE", kop.PredToughnessLE, "kop.PredCMCLE", kop.PredCMCLE, "kop.PredCMCGE", kop.PredCMCGE,
		"kop.PredHasKeyword", kop.PredHasKeyword, "kop.PredHasCounter", kop.PredHasCounter, "kop.PredSubtype", kop.PredSubtype,
		"kop.PredNamed", kop.PredNamed, "kop.PredWithPowerLEX", kop.PredWithPowerLEX, "kop.PredCMCEQ", kop.PredCMCEQ,
		"kop.PredCMCLEV", kop.PredCMCLEV, "kop.PredCountersGE", kop.PredCountersGE, "kop.PredPowerGEV", kop.PredPowerGEV,
		"kop.PredToughnessGE", kop.PredToughnessGE, "kop.PredHasXCost", kop.PredHasXCost,
		"kop.PredPowerLTToughness", kop.PredPowerLTToughness,
		"kop.PredMultiColor", kop.PredMultiColor, "kop.PredHistoric", kop.PredHistoric, "kop.PredEquipped", kop.PredEquipped,
		"kop.PredEnchanted", kop.PredEnchanted, "kop.PredAttached", kop.PredAttached, "kop.PredActiveCtrl", kop.PredActiveCtrl,
		"kop.PredTypesAll", kop.PredTypesAll, "kop.PredTypesAllCMCLE", kop.PredTypesAllCMCLE, "kop.PredModified", kop.PredModified, "kop.PredHasCounters", kop.PredHasCounters,
		"kop.PredMonstrous", kop.PredMonstrous, "kop.PredNoCounters", kop.PredNoCounters, "kop.PredLacksKeyword", kop.PredLacksKeyword,
		"kop.PredOutlaw", kop.PredOutlaw, "kop.PredMonoColor", kop.PredMonoColor,
		"kop.PredSameNameT0", kop.PredSameNameT0, "kop.PredCountersLT", kop.PredCountersLT, "kop.PredToughnessLEV", kop.PredToughnessLEV,
		"kop.PredTotalPT", kop.PredTotalPT, "kop.PredBlocked", kop.PredBlocked, "kop.PredSolved", kop.PredSolved,
		"kop.PredAdventureCard", kop.PredAdventureCard, "kop.PredSuspected", kop.PredSuspected, "kop.PredTrigPlayerCtrl", kop.PredTrigPlayerCtrl, "kop.PredExiledWith", kop.PredExiledWith, "kop.PredWasCastEnter", kop.PredWasCastEnter, "kop.PredWasCastSpell", kop.PredWasCastSpell, "kop.PredNotF2", kop.PredNotF2, "kop.PredFaceDown", kop.PredFaceDown, "kop.PredPowerGTBase", kop.PredPowerGTBase, "kop.PredPermMark", kop.PredPermMark, "kop.PredWord7", kop.PredWord7,
		"kop.PredTriggerObj", kop.PredTriggerObj, "kop.PredHasManaAbility", kop.PredHasManaAbility,
		"kop.PredSameNameRem", kop.PredSameNameRem)
	enum[kop.ExprKind]("kop.ExNone", kop.ExNone, "kop.ExConst", kop.ExConst, "kop.ExX", kop.ExX, "kop.ExCount", kop.ExCount,
		"kop.ExCountGraveyard", kop.ExCountGraveyard, "kop.ExSumPower", kop.ExSumPower, "kop.ExDistinctCMC", kop.ExDistinctCMC,
		"kop.ExKicked", kop.ExKicked, "kop.ExMorbid", kop.ExMorbid, "kop.ExAttackedThisTurn", kop.ExAttackedThisTurn,
		"kop.ExPowerOf", kop.ExPowerOf, "kop.ExToughnessOf", kop.ExToughnessOf, "kop.ExLifeOf", kop.ExLifeOf,
		"kop.ExLibrarySizeOf", kop.ExLibrarySizeOf, "kop.ExStartingLife", kop.ExStartingLife, "kop.ExTriggerAmount", kop.ExTriggerAmount,
		"kop.ExLifeGainedThisTurn", kop.ExLifeGainedThisTurn, "kop.ExOppLifeLostThisTurn", kop.ExOppLifeLostThisTurn,
		"kop.ExCountersOn", kop.ExCountersOn, "kop.ExCreaturesDied", kop.ExCreaturesDied, "kop.ExRememberedCount", kop.ExRememberedCount,
		"kop.ExExcessDamage", kop.ExExcessDamage, "kop.ExLastLifeLost", kop.ExLastLifeLost, "kop.ExTokensCreated", kop.ExTokensCreated,
		"kop.ExLifeLostThisTurn", kop.ExLifeLostThisTurn, "kop.ExSum", kop.ExSum, "kop.ExHandSizeOf", kop.ExHandSizeOf,
		"kop.ExCountExile", kop.ExCountExile, "kop.ExOppsLostLife", kop.ExOppsLostLife, "kop.ExMVOf", kop.ExMVOf,
		"kop.ExCountAtLeast", kop.ExCountAtLeast, "kop.ExPromisedGift", kop.ExPromisedGift,
		"kop.ExResolvedThisTurn", kop.ExResolvedThisTurn, "kop.ExThisTurnCast", kop.ExThisTurnCast,
		"kop.ExRollResult", kop.ExRollResult, "kop.ExCastManaSpent", kop.ExCastManaSpent, "kop.ExConverge", kop.ExConverge,
		"kop.ExCompare", kop.ExCompare, "kop.ExBranches", kop.ExBranches, "kop.ExDevotion", kop.ExDevotion,
		"kop.ExParty", kop.ExParty, "kop.ExDrawnThisTurn", kop.ExDrawnThisTurn, "kop.ExGraveTypes", kop.ExGraveTypes,
		"kop.ExValidAgg", kop.ExValidAgg, "kop.ExGraveAgg", kop.ExGraveAgg, "kop.ExYourSpeed", kop.ExYourSpeed,
		"kop.ExHighestOppPoison", kop.ExHighestOppPoison, "kop.ExThisTurnEntered", kop.ExThisTurnEntered,
		"kop.ExThisTurnCastSpec", kop.ExThisTurnCastSpec, "kop.ExCommittedCrime", kop.ExCommittedCrime,
		"kop.ExSelfPrepared", kop.ExSelfPrepared, "kop.ExThisTurnActGrave", kop.ExThisTurnActGrave, "kop.ExThisTurnActExhaust", kop.ExThisTurnActExhaust, "kop.ExActivations", kop.ExActivations, "kop.ExLifeLostLastTurn", kop.ExLifeLostLastTurn, "kop.ExSneaked", kop.ExSneaked, "kop.ExTriggerSpellMVLKI", kop.ExTriggerSpellMVLKI,
		"kop.ExPoisonOf", kop.ExPoisonOf, "kop.ExProduct", kop.ExProduct, "kop.ExAttackersAll", kop.ExAttackersAll, "kop.ExRefCount", kop.ExRefCount, "kop.ExNumColorsOf", kop.ExNumColorsOf, "kop.ExTgtSpent", kop.ExTgtSpent,
		"kop.ExCountHand", kop.ExCountHand, "kop.ExYourTurns", kop.ExYourTurns, "kop.ExLowestLife", kop.ExLowestLife, "kop.ExOppsAttacked", kop.ExOppsAttacked, "kop.ExRedNCDmg", kop.ExRedNCDmg, "kop.ExVoid", kop.ExVoid, "kop.ExYourExtraTurns", kop.ExYourExtraTurns,
		"kop.ExRevolt", kop.ExRevolt, "kop.ExLeftZone", kop.ExLeftZone, "kop.ExCtrAdded", kop.ExCtrAdded,
		"kop.ExCastFromGrave", kop.ExCastFromGrave, "kop.ExFirstCast", kop.ExFirstCast, "kop.ExIfCastOwnMain", kop.ExIfCastOwnMain, "kop.ExExiledWithN", kop.ExExiledWithN,
		"kop.ExSacrificedThisTurn", kop.ExSacrificedThisTurn, "kop.ExDiscardedThisTurn", kop.ExDiscardedThisTurn,
		"kop.ExScriedThisTurn", kop.ExScriedThisTurn, "kop.ExDilemmasFaced", kop.ExDilemmasFaced, "kop.ExMonarch", kop.ExMonarch, "kop.ExRemLKIPT", kop.ExRemLKIPT, "kop.ExCastBattlefield", kop.ExCastBattlefield, "kop.ExUnlockedDoors", kop.ExUnlockedDoors, "kop.ExNotOwnLibMV", kop.ExNotOwnLibMV, "kop.ExChromaOf", kop.ExChromaOf, "kop.ExChosenMode", kop.ExChosenMode, "kop.ExChosenNumber", kop.ExChosenNumber, "kop.ExMod", kop.ExMod, "kop.ExPow", kop.ExPow, "kop.ExAttackedTurn", kop.ExAttackedTurn, "kop.ExPhaseCount", kop.ExPhaseCount, "kop.ExAdamant", kop.ExAdamant, "kop.ExCounterKinds", kop.ExCounterKinds, "kop.ExTargetPlayers", kop.ExTargetPlayers, "kop.ExTapPower", kop.ExTapPower, "kop.ExStepIn", kop.ExStepIn, "kop.ExOppExtreme", kop.ExOppExtreme, "kop.ExDmgAmountReceived", kop.ExDmgAmountReceived, "kop.ExPlExtreme", kop.ExPlExtreme)
	enum[kop.CondKind]("kop.CondAlways", kop.CondAlways, "kop.CondPresent", kop.CondPresent, "kop.CondValue", kop.CondValue,
		"kop.CondKicked", kop.CondKicked, "kop.CondThreshold", kop.CondThreshold, "kop.CondYourTurn", kop.CondYourTurn,
		"kop.CondRefMatches", kop.CondRefMatches, "kop.CondPromised", kop.CondPromised, "kop.CondAnd", kop.CondAnd, "kop.CondEchoOwed", kop.CondEchoOwed,
		"kop.CondSourceZone", kop.CondSourceZone)
	enum[kop.CmpOp]("kop.CmpNone", kop.CmpNone, "kop.CmpGE", kop.CmpGE, "kop.CmpLE", kop.CmpLE, "kop.CmpEQ", kop.CmpEQ,
		"kop.CmpGT", kop.CmpGT, "kop.CmpLT", kop.CmpLT, "kop.CmpNE", kop.CmpNE, "kop.CmpPrime", kop.CmpPrime)
	enum[kop.CounterKind]("kop.CtrP1P1", kop.CtrP1P1, "kop.CtrM1M1", kop.CtrM1M1, "kop.CtrLoyalty", kop.CtrLoyalty,
		"kop.CtrStun", kop.CtrStun, "kop.CtrShield", kop.CtrShield, "kop.CtrFlying", kop.CtrFlying, "kop.CtrOther", kop.CtrOther,
		"kop.CtrIncubation", kop.CtrIncubation, "kop.CtrBait", kop.CtrBait, "kop.CtrSoul", kop.CtrSoul,
		"kop.CtrFellowship", kop.CtrFellowship, "kop.CtrRevival", kop.CtrRevival, "kop.CtrStash", kop.CtrStash, "kop.CtrLevel", kop.CtrLevel,
		"kop.CtrCharge", kop.CtrCharge, "kop.CtrOil", kop.CtrOil, "kop.CtrSpore", kop.CtrSpore, "kop.CtrQuest", kop.CtrQuest,
		"kop.CtrVerse", kop.CtrVerse, "kop.CtrGrowth", kop.CtrGrowth, "kop.CtrFuse", kop.CtrFuse, "kop.CtrStory", kop.CtrStory,
		"kop.CtrFirstStrike", kop.CtrFirstStrike, "kop.CtrDoubleStrike", kop.CtrDoubleStrike,
		"kop.CtrDeathtouch", kop.CtrDeathtouch, "kop.CtrHaste", kop.CtrHaste, "kop.CtrHexproof", kop.CtrHexproof,
		"kop.CtrIndestructible", kop.CtrIndestructible, "kop.CtrLifelink", kop.CtrLifelink,
		"kop.CtrMenace", kop.CtrMenace, "kop.CtrReach", kop.CtrReach, "kop.CtrShadow", kop.CtrShadow,
		"kop.CtrTrample", kop.CtrTrample, "kop.CtrVigilance", kop.CtrVigilance, "kop.CtrSupply", kop.CtrSupply,
		"kop.CtrLore", kop.CtrLore, "kop.CtrLandmark", kop.CtrLandmark, "kop.CtrDread", kop.CtrDread, "kop.CtrMaxSpeed", kop.CtrMaxSpeed, "kop.CtrMode", kop.CtrMode,
		"kop.CtrDefense", kop.CtrDefense, "kop.CtrChorus", kop.CtrChorus, "kop.CtrM0M1", kop.CtrM0M1, "kop.CtrFinality", kop.CtrFinality, "kop.CtrTime", kop.CtrTime, "kop.CtrValor", kop.CtrValor, "kop.CtrNet", kop.CtrNet, "kop.CtrPlan", kop.CtrPlan,
		"kop.CtrInvasion", kop.CtrInvasion, "kop.CtrLoot", kop.CtrLoot, "kop.CtrRev", kop.CtrRev, "kop.CtrPage", kop.CtrPage,
		"kop.CtrIngenuity", kop.CtrIngenuity, "kop.CtrFilm", kop.CtrFilm, "kop.CtrFire", kop.CtrFire, "kop.CtrHone", kop.CtrHone, "kop.CtrCoin", kop.CtrCoin,
		"kop.CtrDivinity", kop.CtrDivinity, "kop.CtrImpostor", kop.CtrImpostor, "kop.CtrBore", kop.CtrBore, "kop.CtrPoint", kop.CtrPoint,
		"kop.CtrBloodstain", kop.CtrBloodstain, "kop.CtrPossession", kop.CtrPossession, "kop.CtrPromotion", kop.CtrPromotion, "kop.CtrPeace", kop.CtrPeace, "kop.CtrWish", kop.CtrWish, "kop.CtrSkewer", kop.CtrSkewer, "kop.CtrNest", kop.CtrNest, "kop.CtrAge", kop.CtrAge, "kop.CtrLuck", kop.CtrLuck, "kop.CtrDepletion", kop.CtrDepletion, "kop.CtrMining", kop.CtrMining, "kop.CtrFade", kop.CtrFade, "kop.CtrBurden", kop.CtrBurden,
		"kop.CtrAll", kop.CtrAll)
	enum[kop.Whose]("kop.WhoseAny", kop.WhoseAny, "kop.WhoseYou", kop.WhoseYou, "kop.WhoseOpp", kop.WhoseOpp,
		"kop.WhoseEnchCtrl", kop.WhoseEnchCtrl, "kop.WhoseNonActive", kop.WhoseNonActive)
	enum[kop.Step]("kop.StepUntap", kop.StepUntap, "kop.StepUpkeep", kop.StepUpkeep, "kop.StepDraw", kop.StepDraw,
		"kop.StepMain1", kop.StepMain1, "kop.StepBeginCombat", kop.StepBeginCombat, "kop.StepAttackers", kop.StepAttackers,
		"kop.StepBlockers", kop.StepBlockers, "kop.StepFirstStrikeDamage", kop.StepFirstStrikeDamage, "kop.StepDamage", kop.StepDamage,
		"kop.StepEndCombat", kop.StepEndCombat, "kop.StepMain2", kop.StepMain2, "kop.StepEnd", kop.StepEnd, "kop.StepCleanup", kop.StepCleanup)

	mask[kop.TypeMask]("kop.TLand", kop.TLand, "kop.TCreature", kop.TCreature, "kop.TArtifact", kop.TArtifact,
		"kop.TEnchantment", kop.TEnchantment, "kop.TPlaneswalker", kop.TPlaneswalker, "kop.TInstant", kop.TInstant,
		"kop.TSorcery", kop.TSorcery, "kop.TBattle", kop.TBattle, "kop.TKindred", kop.TKindred, "kop.TAura", kop.TAura,
		"kop.TEquipment", kop.TEquipment, "kop.TBasic", kop.TBasic, "kop.TLegendary", kop.TLegendary, "kop.TToken", kop.TToken,
		"kop.TVehicle", kop.TVehicle)
	mask[kop.KwMask]("kop.KwFlying", kop.KwFlying, "kop.KwReach", kop.KwReach, "kop.KwFirstStrike", kop.KwFirstStrike,
		"kop.KwDoubleStrike", kop.KwDoubleStrike, "kop.KwDeathtouch", kop.KwDeathtouch, "kop.KwLifelink", kop.KwLifelink,
		"kop.KwTrample", kop.KwTrample, "kop.KwVigilance", kop.KwVigilance, "kop.KwHaste", kop.KwHaste, "kop.KwMenace", kop.KwMenace,
		"kop.KwDefender", kop.KwDefender, "kop.KwFlash", kop.KwFlash, "kop.KwHexproof", kop.KwHexproof,
		"kop.KwIndestructible", kop.KwIndestructible, "kop.KwProwess", kop.KwProwess, "kop.KwProtectionAll", kop.KwProtectionAll,
		"kop.KwShroud", kop.KwShroud, "kop.KwWard", kop.KwWard, "kop.KwUnblockable", kop.KwUnblockable,
		"kop.KwCantBlock", kop.KwCantBlock, "kop.KwWither", kop.KwWither, "kop.KwKicker", kop.KwKicker,
		"kop.KwFlashback", kop.KwFlashback, "kop.KwAffinity", kop.KwAffinity, "kop.KwHexproofFromInstants", kop.KwHexproofFromInstants,
		"kop.KwProtBlack", kop.KwProtBlack, "kop.KwProtRed", kop.KwProtRed, "kop.KwPreventCombatDamageTo", kop.KwPreventCombatDamageTo,
		"kop.KwFlashbackOwnCost", kop.KwFlashbackOwnCost, "kop.KwPreventCombatDamageFrom", kop.KwPreventCombatDamageFrom)
	mask[kop.ColorMask]("kop.ColW", kop.ColW, "kop.ColU", kop.ColU, "kop.ColB", kop.ColB, "kop.ColR", kop.ColR,
		"kop.ColG", kop.ColG, "kop.ColC", kop.ColC)
	mask[kop.CostFlags]("kop.CostTap", kop.CostTap, "kop.CostUntap", kop.CostUntap, "kop.CostSacSelf", kop.CostSacSelf,
		"kop.CostSac", kop.CostSac, "kop.CostSubAmong", kop.CostSubAmong, "kop.CostExileCtrlOrGrave", kop.CostExileCtrlOrGrave, "kop.CostRemoveCtr", kop.CostRemoveCtr, "kop.CostAddCtr", kop.CostAddCtr,
		"kop.CostDiscard", kop.CostDiscard, "kop.CostPayLife", kop.CostPayLife, "kop.CostTapX", kop.CostTapX,
		"kop.CostExileGrave", kop.CostExileGrave, "kop.CostSorcery", kop.CostSorcery, "kop.CostOncePerTurn", kop.CostOncePerTurn,
		"kop.CostLoyalty", kop.CostLoyalty, "kop.CostOncePerGame", kop.CostOncePerGame, "kop.CostTapHost", kop.CostTapHost, "kop.CostDiscardSelf", kop.CostDiscardSelf, "kop.CostForage", kop.CostForage, "kop.CostPayEnergy", kop.CostPayEnergy, "kop.CostReturnSelf", kop.CostReturnSelf, "kop.CostReturn", kop.CostReturn, "kop.CostExileSelf", kop.CostExileSelf, "kop.CostTapPower", kop.CostTapPower, "kop.CostPowerUp", kop.CostPowerUp, "kop.CostBlight", kop.CostBlight, "kop.CostDiscardFiltered", kop.CostDiscardFiltered)
	mask[kop.FilterFlags]("kop.FOther", kop.FOther, "kop.FNontoken", kop.FNontoken, "kop.FToken", kop.FToken,
		"kop.FTapped", kop.FTapped, "kop.FUntapped", kop.FUntapped, "kop.FAttacking", kop.FAttacking, "kop.FBlocking", kop.FBlocking,
		"kop.FAttackingOrBlocking", kop.FAttackingOrBlocking, "kop.FSelf", kop.FSelf, "kop.FPlayer", kop.FPlayer,
		"kop.FCreatureOrPW", kop.FCreatureOrPW, "kop.FNotAttacking", kop.FNotAttacking, "kop.FSpell", kop.FSpell, "kop.FAttachedBySrc", kop.FAttachedBySrc, "kop.FExiledWith", kop.FExiledWith, "kop.FNotAttachedBySrc", kop.FNotAttachedBySrc)
	mask[kop.FilterFlags2]("kop.F2Kicked", kop.F2Kicked, "kop.F2CastByYou", kop.F2CastByYou, "kop.F2EnteredThisTurn", kop.F2EnteredThisTurn, "kop.F2ChosenType", kop.F2ChosenType,
		"kop.F2ChosenColor", kop.F2ChosenColor, "kop.F2Remembered", kop.F2Remembered, "kop.F2NotRemembered", kop.F2NotRemembered,
		"kop.F2ChosenCard", kop.F2ChosenCard, "kop.F2NotChosenCard", kop.F2NotChosenCard, "kop.F2AttachedToT0", kop.F2AttachedToT0,
		"kop.F2TargetedBy", kop.F2TargetedBy, "kop.F2SameNameAsSource", kop.F2SameNameAsSource, "kop.F2WithFlying", kop.F2WithFlying,
		"kop.F2WithoutFlying", kop.F2WithoutFlying, "kop.F2Emblem", kop.F2Emblem, "kop.F2Opponent", kop.F2Opponent,
		"kop.F2You", kop.F2You, "kop.F2Basic", kop.F2Basic, "kop.F2Legendary", kop.F2Legendary, "kop.F2SharesColorRem", kop.F2SharesColorRem,
		"kop.F2NotTriggerObj", kop.F2NotTriggerObj, "kop.F2WasDamaged", kop.F2WasDamaged, "kop.F2Saddled", kop.F2Saddled,
		"kop.F2Sneaked", kop.F2Sneaked, "kop.F2Unblocked", kop.F2Unblocked, "kop.F2Warped", kop.F2Warped,
		"kop.F2TgtPlayerCtrl", kop.F2TgtPlayerCtrl, "kop.F2DefendingCtrl", kop.F2DefendingCtrl)
	mask[kop.TrigFlags]("kop.TfOptional", kop.TfOptional, "kop.TfSelf", kop.TfSelf, "kop.TfCombat", kop.TfCombat,
		"kop.TfToPlayer", kop.TfToPlayer, "kop.TfOncePerTurn", kop.TfOncePerTurn, "kop.TfSecondDraw", kop.TfSecondDraw,
		"kop.TfNontokenSrc", kop.TfNontokenSrc, "kop.TfNotToGrave", kop.TfNotToGrave)
	mask[kop.TrigFlags2]("kop.Tf2ManaFromSelf", kop.Tf2ManaFromSelf, "kop.Tf2FirstTime", kop.Tf2FirstTime, "kop.Tf2YourTurn", kop.Tf2YourTurn, "kop.Tf2OppTurn", kop.Tf2OppTurn,
		"kop.Tf2NonCombat", kop.Tf2NonCombat, "kop.Tf2ToOpponent", kop.Tf2ToOpponent, "kop.Tf2SelfOrFilter", kop.Tf2SelfOrFilter,
		"kop.Tf2SecondDraw", kop.Tf2SecondDraw, "kop.Tf2CastSnapshot", kop.Tf2CastSnapshot, "kop.Tf2NthSpell", kop.Tf2NthSpell, "kop.Tf2BlockedBySelf", kop.Tf2BlockedBySelf, "kop.Tf2TargetsFilter2", kop.Tf2TargetsFilter2, "kop.Tf2Alone", kop.Tf2Alone, "kop.Tf2CastFromHand", kop.Tf2CastFromHand, "kop.Tf2Convoked", kop.Tf2Convoked, "kop.Tf2Convoked", kop.Tf2Convoked,
		"kop.Tf2AtkYou", kop.Tf2AtkYou, "kop.Tf2AtkYourPW", kop.Tf2AtkYourPW, "kop.Tf2AtkOpp", kop.Tf2AtkOpp, "kop.Tf2Batch", kop.Tf2Batch, "kop.Tf2PerPair", kop.Tf2PerPair, "kop.Tf2Cycled", kop.Tf2Cycled, "kop.Tf2ResolvedLimit", kop.Tf2ResolvedLimit, "kop.Tf2CastSpent", kop.Tf2CastSpent, "kop.Tf2ToYou", kop.Tf2ToYou, "kop.Tf2PerReferent", kop.Tf2PerReferent, "kop.Tf2SrcInPlay", kop.Tf2SrcInPlay, "kop.Tf2SpentBound", kop.Tf2SpentBound)
	mask[kop.TrigFlags3]("kop.Tf3ConvergeBound", kop.Tf3ConvergeBound, "kop.Tf3CastConverge", kop.Tf3CastConverge,
		"kop.Tf3ManaFromTreasure", kop.Tf3ManaFromTreasure, "kop.Tf3OrTargets", kop.Tf3OrTargets,
		"kop.Tf3Static", kop.Tf3Static, "kop.Tf3MayPlaySrc", kop.Tf3MayPlaySrc, "kop.Tf3GameOnce", kop.Tf3GameOnce, "kop.Tf3FirstAttack", kop.Tf3FirstAttack)
	mask[kop.StaticFlags]("kop.StDynamic", kop.StDynamic, "kop.StOncePerTurn", kop.StOncePerTurn, "kop.StAnyManaType", kop.StAnyManaType,
		"kop.StWithoutManaCost", kop.StWithoutManaCost, "kop.StFunctionsInGraveyard", kop.StFunctionsInGraveyard,
		"kop.StRemove6Counters", kop.StRemove6Counters, "kop.StTargetsFilter2", kop.StTargetsFilter2, "kop.StYouHexproof", kop.StYouHexproof,
		"kop.StFunctionsInExile", kop.StFunctionsInExile, "kop.StPanhETB", kop.StPanhETB, "kop.StAlsoSideboard", kop.StAlsoSideboard, "kop.StFreeOnly", kop.StFreeOnly,
		"kop.StTapPowerCrew", kop.StTapPowerCrew, "kop.StTapPowerSaddle", kop.StTapPowerSaddle, "kop.StTapPowerStation", kop.StTapPowerStation, "kop.StTapPowerTough", kop.StTapPowerTough,
		"kop.StAtkTaxPlayer", kop.StAtkTaxPlayer, "kop.StAtkTaxWalker", kop.StAtkTaxWalker)
	mask[kop.AbilityFlags]("kop.AfLatent", kop.AfLatent, "kop.AfManaDynamic", kop.AfManaDynamic, "kop.AfManaChosen", kop.AfManaChosen, "kop.AfOptionalCost", kop.AfOptionalCost,
		"kop.AfCheckOnTrigger", kop.AfCheckOnTrigger, "kop.AfEquip", kop.AfEquip, "kop.AfCharmTrigger", kop.AfCharmTrigger, "kop.AfManaEachColor", kop.AfManaEachColor,
		"kop.AfManaChoice", kop.AfManaChoice, "kop.AfGift", kop.AfGift, "kop.AfResolveCount", kop.AfResolveCount, "kop.AfStation", kop.AfStation, "kop.AfManaReflect", kop.AfManaReflect, "kop.AfManaNonSpell", kop.AfManaNonSpell)
	mask[kop.TargetFlags]("kop.TgfDistinct", kop.TgfDistinct, "kop.TgfSameController", kop.TgfSameController,
		"kop.TgfDiffController", kop.TgfDiffController, "kop.TgfKickedRange", kop.TgfKickedRange, "kop.TgfAttachedToT0", kop.TgfAttachedToT0, "kop.TgfDecl", kop.TgfDecl, "kop.TgfGiftRange", kop.TgfGiftRange, "kop.TgfDeclSub", kop.TgfDeclSub, "kop.TgfBoundsV", kop.TgfBoundsV, "kop.TgfSingleTgt", kop.TgfSingleTgt, "kop.TgfActOnly", kop.TgfActOnly)
}
