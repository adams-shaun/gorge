package cards

import (
	"math/bits"
	"slices"
	"strings"
	"sync/atomic"
	"unsafe"
)

// ParamKey is a compiled parameter key: a dense ordinal over the fixed
// vocabulary below, the keys the rules hot paths read. A node's ParamSet
// answers a ParamKey read with a mask test and a popcount rank into a value
// slice instead of a string-keyed map probe. Keys outside the vocabulary are
// read from Params as before.
//
// The rules parameter census (engine/rules/paramcensus_test.go) counts
// `x.Param(cards.PK<Key>)`, `x.ParamStr(cards.PK<Key>)`, `x.ParamCode(cards.PK<Key>)` and
// `x.HasParam(cards.PK<Key>)` as reads of <Key>, exactly like
// `x.Params["<Key>"]`, so a constant's name MUST be "PK" + the key text.
//
// The type is uint16 and the vocabulary may grow to paramKeyCap keys (the
// corpus names ~1160 distinct parameters in all). A ParamSet's mask is sized
// to the vocabulary actually declared (paramMaskWords), so a new key costs
// nothing until it crosses a 64-key word boundary, and then 10 bytes per
// compiled node.
type ParamKey uint16

const (
	pkNone ParamKey = iota
	// Insert new keys in their sorted position (by key text), never appended:
	// a tail append from parallel branches always conflicts at this block's
	// end, while an insert at a distinct position merges cleanly.
	// TestParamKeysSorted enforces the order.
	PKAILogic
	PKAIManaPref
	PKAbilities
	PKActivate
	PKActivation
	PKActivationAfterBlockers
	PKActivationFirstCombat
	PKActivationGameTypes
	PKActivationLimit
	PKActivationPhases
	PKActivationZone
	PKActivator
	PKActivatorThisTurnCast
	PKActivatorThisTurnCastEach
	PKActivePhases
	PKActiveZones
	PKAdamant
	PKAdapt
	PKAddAbilities
	PKAddAbility
	PKAddAllCreatureTypes
	PKAddColor
	PKAddColors
	PKAddKWs
	PKAddKeyword
	PKAddKeywords
	PKAddPower
	// A Mode$ Continuous static's AddReplacementEffect$ grant (DFT Far Fortune, End
	// Boss; Vnwxt, Verbose Host; append-only).
	PKAddReplacementEffect
	PKAddSVar
	PKAddSVars
	PKAddStaticAbilities
	PKAddStaticAbility
	PKAddToughness
	PKAddTrigger
	PKAddTriggers
	PKAddType
	PKAddTypes
	PKAddsCounters
	PKAddsKeywords
	PKAddsKeywordsUntil
	PKAddsKeywordsValid
	PKAddsNoCounter
	PKAdjustLandPlays
	PKAffected
	PKAffectedZone
	PKAfterPhase
	PKAllCards
	// RemoveCounterAll's AllCounterTypes$ flag (Aether Snap): remove every counter
	// kind from each valid permanent, the corpus spelling of "all counters of all
	// kinds" (CR 122.8).
	PKAllCounterTypes
	PKAllCounters
	PKAllValid
	// Play's AllowRepeats$: a chosen card stays a candidate for the next play.
	PKAllowRepeats
	PKAlone
	PKAlternateCost
	PKAlternativeCost
	PKAlternativeDecider
	PKAlwaysRemember
	PKAmount
	PKAmountByChosenMap
	PKAmountFromVotes
	PKAnnihilator
	PKAnnounce
	PKAnnounceTitle
	PKAnyNumber
	PKAtEOT
	PKAtEOTTrig
	// AtLeastOneInstance$ <cmp> on a DamageDealtOnce trigger (Matoc, Lavamancer):
	// some recipient's share of the batch passes the comparison. Read by rules
	// latchDamageBatch / closeDamageBatch.
	PKAtLeastOneInstance
	PKAtRandom
	PKAttachedTo
	PKAttachedToPlayer
	PKAttacked
	PKAttackedTarget
	PKAttacker
	PKAttacking
	PKAttackingPlayer
	PKAttributes
	PKBecomeStartingPlayer
	PKBeginTurn
	// BidSubAbility$ on a BidLife: the SVar body the winning bid resolves (Forge's
	// BidSubAbility, the DBLoseLife chain). Read by effects' effBidLife.
	PKBidSubAbility
	PKBlessing
	PKBlockAllDefined
	PKBoast
	PKBolster
	// Boon$ True on an Effect and BoonAmount$ on its trigger: a one-time boon ends
	// after its trigger has fired that many times.
	PKBoon
	PKBoonAmount
	PKBranchConditionSVar
	PKBranchConditionSVarCompare
	// A Mode$ Continuous static's CanBlockAmount$ N / CanBlockAny$ True: the
	// affected creature can block N additional / any number of creatures (CR
	// 509.1a). A Pump's CanBlockAny$ True is the same grant until end of turn
	// (effects.registerPumpCanBlockAny).
	PKCanBlockAmount
	PKCanBlockAny
	PKCanRepeatModes
	PKCantHaveKeyword
	PKCaster
	PKChangeColorWord
	PKChangeController
	PKChangeLater
	PKChangeNum
	PKChangeSingleTarget
	PKChangeType
	PKChangeTypeWord
	PKChangeValid
	PKChangeZoneTable
	PKCharacteristicDefining
	PKCharmNum
	PKCheckDefinedPlayer
	PKCheckOnTriggeredCard
	PKCheckSVar
	PKCheckSecondSVar
	PKChoiceAmount
	PKChoiceNum
	PKChoiceOptional
	PKChoicePrompt
	PKChoiceRestriction
	PKChoiceTitle
	PKChoiceZone
	PKChoices
	PKChoose
	PKChooseAmount
	PKChooseCounter
	PKChooseDifferent
	PKChooseEach
	PKChooseFromDefined
	PKChooseFromDefinedCards
	PKChooseFromList
	PKChooseOrder
	PKChooser
	PKChosenPile
	PKChosenSVar
	PKChosenZone
	PKCipherCopy
	PKClassBand
	PKClassLevel
	PKClearChosenCard
	PKClearChosenColor
	PKClearChosenPlayer
	PKClearImprinted
	PKClearNamedCard
	PKClearNotedCardsFor
	PKClearRemembered
	PKClearRememberedBeforeLoop
	PKColor
	PKColorOrType
	PKColors
	PKColorsFrom
	PKCombatDamage
	PKCondition
	PKConditionActivationLimit
	PKConditionCheckSVar
	PKConditionCompare
	PKConditionCompare2
	PKConditionDefined
	PKConditionFirstCombat
	PKConditionNotPresent
	PKConditionPhases
	PKConditionPlayerContains
	// ConditionPlayerDefined$ / ConditionPlayerContains$: the player-group
	// membership gate read by effects' conditionMetCore.
	PKConditionPlayerDefined
	PKConditionPlayerTurn
	PKConditionPresent
	PKConditionPresent2
	PKConditionSVarCompare
	PKConditionZone
	PKControlOpponentsSearchingLibrary
	PKControlledByPlayer
	PKController
	PKControllerUntaps
	PKCopyCard
	PKCost
	PKCounterAmount
	PKCounterNum
	PKCounterNumPerDefined
	PKCounterType
	PKCounterType2
	PKCounterTypeChoice
	PKCounterTypePerDefined
	PKDamageAmount
	PKDamageMap
	PKDamageSource
	PKDamageTarget
	PKDayTime
	PKDefined
	PKDefinedAttacker
	PKDefinedCards
	PKDefinedDamagers
	PKDefinedExiler
	PKDefinedKW
	PKDefinedMagnet
	PKDefinedName
	PKDefinedPiles
	PKDefinedPlayer
	PKDefinedPlayerChooses
	PKDefinedTarget
	PKDelayedTriggerDefinedPlayer
	PKDelirium
	PKDescription
	PKDestAltSVar
	PKDestAltSVarCompare
	PKDestination
	PKDestinationAlternative
	PKDestinationZone
	PKDestinationZone2
	PKDethrone
	PKDifferentNames
	// DigMultiple's distinct choice and remainder grammar (append-only).
	PKDigNum
	PKDiscard
	PKDiscardValid
	PKDiscardValidDesc
	PKDivideEvenly
	PKDividedAsYouChoose
	PKDividedRandomly
	PKDontPlaneswalkAway
	PKDrawLimit
	PKDungeon
	PKDuration
	PKETB
	PKEach
	// EachDifferentPower$ on a ChooseCard resolution: one mandatory pick per
	// distinct power among the Choices$ matches, ascending (Celestial Judgment; CR
	// 608.2c). Read by effects' ChooseCardOf.
	PKEachDifferentPower
	PKEachExistingCounter
	PKEachFromSource
	PKEachToItself
	PKEcho
	PKEffectOnly
	PKEffectOwner
	PKEffectZone
	// Embalm$ True and RemoveCost$ True on a Clone (Vizier of Many Faces's
	// embalm-gated token exceptions). Read by effects' compileClone.
	PKEmbalm
	PKEnduringStory
	PKEvenOddResults
	PKEvolve
	PKExactly
	PKExcessDamage
	PKExcessDamageCondition
	PKExcessSVar
	PKExcessSVarCondition
	PKExclude
	PKExcludeZone
	PKExcludedOrigins
	PKExecute
	PKExhaust
	PKExileFaceDown
	PKExileOnMoved
	// ChangeZone's ExiledWithEffectSource$ rider: the exile is tagged as exiled-with
	// the Effect's source (Forger's Foundry; append-only).
	PKExiledWithEffectSource
	PKExpression
	PKExtraPhase
	PKExtraPhaseDelayedTrigger
	PKExtraPhaseDelayedTriggerExcute
	PKExtraTurn
	PKExtraTurnDelayedTrigger
	PKExtraTurnDelayedTriggerExecute
	PKFaceDown
	PKFaceDownPower
	PKFaceDownSetType
	PKFaceDownToughness
	PKFallbackAbility
	PKFalseSubAbility
	PKFirstAttack
	PKFirstCardInDrawStep
	PKFirstCombat
	PKFirstExtraCardDrawnThisTurn
	PKFirstForetell
	PKFirstTime
	// FirstTimeCrewed$ True and ValidCrewAmount$ N on a Mode$ BecomesCrewed trigger
	// (CR 702.122e; Mindlink Mech, Mighty Servant of Leuk-o). Read by trigmatch's
	// becomesCrewedMatches.
	PKFirstTimeCrewed
	PKFirstTimeSaddled
	PKFirstUpkeep
	PKFlipUntilYouLose
	PKFlipper
	PKFollowedBy
	PKForCost
	PKForEachPlayer
	PKForEachShard
	PKForbiddenNewTypes
	PKForetold
	PKForgetChanged
	PKForgetChosen
	PKForgetCounter
	PKForgetImprinted
	PKForgetObjects
	PKForgetOnCast
	PKForgetOnMoved
	PKForgetOnPhasedIn
	PKForgetOtherRemembered
	PKForgetOtherTargets
	PKForgetPlayed
	PKFound
	PKFoundSearchingLibrary
	PKGainControl
	PKGains
	PKGainsAbilitiesLimitPerTurn
	PKGainsAbilitiesOf
	PKGainsAbilitiesOfDefined
	PKGainsAbilitiesOfZones
	PKGainsTriggerAbsOf
	PKGainsValidAbilities
	PKGameActivationLimit
	PKGoad
	// Haunt$ True on a ChangeZone (the K:Haunt expansion, CR 702.55b): the exiled
	// card haunts the announced creature target. Read by effects' compileChangeZone.
	PKHaunt
	PKHeadsSubAbility
	PKHellbent
	PKHidden
	PKHiddenKeywords
	PKIgnoreFreeze
	PKIgnoreGeneric
	PKImprint
	PKImprintCards
	// ChooseCard's ImprintChosen$ rider records selected card IDs on the source's
	// non-exile imprint association.
	PKImprintChosen
	PKImprintFound
	PKImprintLast
	PKImprintOnHost
	PKImprintPlayed
	PKImprintRest
	PKImprintTokens
	PKIncrement
	PKInstantSpeed
	PKIntoPlayTapped
	PKInvalidTypes
	PKIsCombat
	PKIsDamage
	PKIsPresent
	PKIsPresent2
	PKIsSingleTarget
	PKKW
	PKKWChoice
	// KeepRemembered$ on a TwoPiles resolution: present (Forge's hasParam, any
	// value) the resolution leaves the source's remembered list alone instead of
	// clearing it once both pile bodies ran (Hostile Negotiations' YouChoose keeps
	// it for TheyChoose). Read by effects' twoPilesKeepsRemembered.
	PKKeepRemembered
	PKKeyword
	PKKeywordLine
	PKKeywords
	PKLayer
	PKLeaveBattlefield
	PKLeftRightPile
	// PKLevel is the target level of a Class level-up activator (kw:Class
	// synthesises `AB$ ClassLevelUp | Level$ N`; CR 716.2d sets the designation to
	// N).
	PKLevel
	PKLibraryPosition
	PKLibraryPositionAlternative
	PKLifeAmount
	PKLifeTotal
	PKList
	PKListTitle
	PKLook
	PKLoseControl
	PKLoseSubAbility
	PKManaAmount
	PKManaConversion
	PKManaNotSpent
	// ManaSpent$ / ManaNotSpent$ name the colours whose mana was (or was not) spent
	// to cast the source (Forge's CardTraitBase.meetsRequirements
	// ManaSpent/ManaNotSpent arms; Court Hussar's "sacrifice it unless {W} was spent
	// to cast it", CR 601.2h). Read by triggerConditionHolds against the cast's
	// per-colour spend.
	PKManaSpent
	PKMandatory
	PKMatchedAbility
	PKMax
	PKMaxAttackers
	// MaxNum$ on an S:Mode$ MaxCounter static (Rasputin Dreamweaver's seven dream
	// counters, CR 704.5r). Read by rules' maxCounterSBA.
	PKMaxNum
	PKMaxRevealed
	PKMaxRollsResults
	PKMaxTotalTargetCMC
	PKMaxTotalTargetPower
	PKMayChooseTarget
	PKMayLookAt
	PKMayPlay
	PKMayPlayAltManaCost
	PKMayPlayLimit
	// Shared Fate's MayPlayPlayer$ Exiler rider and the DefinedExiler$ operand that
	// records the exiling player.
	PKMayPlayPlayer
	PKMayPlayText
	PKMayPlayWithFlash
	PKMayPlayWithoutManaCost
	PKMayShuffle
	PKMentor
	PKMetalcraft
	PKMin
	PKMinAmount
	PKMinCharmNum
	PKMinLimit
	PKMinMana
	PKMode
	PKModeCost
	PKMonstrosity
	PKMustAttack
	PKMyriad
	PKName
	PKNatural
	PKNewController
	PKNewCounterAmount
	PKNextRoom
	PKNextTurn
	PKNoCall
	PKNoLonger
	PKNoLooking
	// NoPrevention$ True on DealDamage: this ability's damage can't be prevented (CR
	// 615.12).
	PKNoPrevention
	PKNoRegen
	PKNoResolvingCheck
	PKNoReveal
	PKNoShuffle
	PKNonBasicSpell
	PKNonLegendary
	PKNotFirstCardInDrawStep
	PKNotThisAbility
	PKNoteCards
	PKNoteCardsFor
	// NoteCounterDefined$ on an Effect and WithNotedCounters$ on a ChangeZone
	// (Tawnos's Coffin): the Effect notes the defined objects' counters and the
	// return puts them back. Read by effects' compileEffect and the ChangeZone
	// compiler.
	PKNoteCounterDefined
	PKNoteNumber
	PKNum
	PKNumAtt
	PKNumCards
	PKNumDef
	PKNumDmg
	PKNumLimitEachTurn
	PKNumPhases
	PKNumRandomChoices
	PKNumTurns
	PKNumber
	PKObject
	PKOncePerSource
	PKOneOff
	PKOnlyFirst
	PKOnlyFirstSpell
	PKOpponentTurn
	PKOptionQuestion
	PKOptional
	PKOptionalAbilityPrompt
	PKOptionalDecider
	PKOptionalPrompt
	PKOrColors
	PKOrderDuplicates
	PKOrigin
	PKOriginAlternative
	// OtherBidder$ on a BidLife: the extra bidding participant(s) beyond the
	// resolving controller (Mages' Contest's TargetedController). Read by effects'
	// effBidLife.
	PKOtherBidder
	PKOtherSVar
	PKOtherwiseSubAbility
	PKOverwriteColors
	// Paid$ True/False on a Mode$ PayCumulativeUpkeep trigger (Hibernation's End's
	// True, Heart of Bogardan's and Thought Lash's False): True is the payment arm
	// the settle marker serves (inherent -- the marker exists only when the cost was
	// paid), False is the decline arm, which no marker reaches. Read by trigmatch's
	// payCumulativeUpkeepMatches.
	PKPaid
	PKPawprint
	PKPersistentMana
	PKPersistentUntilEndOfCombat
	PKPhase
	PKPhaseCount
	PKPhaseInOrOut
	PKPhaseout
	PKPhases
	PKPlacer
	PKPlaneswalker
	PKPlayCost
	PKPlayer
	PKPlayerChoices
	PKPlayerTurn
	PKPopulate
	PKPower
	PKPowerUp
	PKPresentCompare
	PKPresentDefined
	PKPresentZone
	PKPrevent
	// A DamageDone replacement's PreventionEffect$ True: the line prevents the
	// damage and its ReplaceWith$ body is the rider (The Mindskinner's mill). CR
	// 615.12 keeps the rider and drops the prevention under can't-prevent.
	PKPreventionEffect
	PKPreventionShield
	PKPreventionSubAbility
	// Meld-specific operands.
	PKPrimary
	PKProduced
	PKPromptToSkipOptionalAbility
	PKPumpDuration
	PKPumpKeywords
	PKPumpZone
	PKRaiseMaxHandSize
	PKRaiseTo
	PKRandom
	PKRandomChange
	PKRandomCompare
	PKRandomCompareSVar
	PKRandomCopied
	PKRandomNum
	PKRandomNumTargets
	PKRandomOrder
	PKRandomTarget
	PKRandomTargetRestriction
	PKRandomType
	// RearrangeTopOfLibrary's RearrangePlayer$ chooser (Tahngarth's Glare's "That
	// player looks at the top three cards of your library"; append-only).
	PKRearrangePlayer
	PKRedistribute
	PKReduceAmount
	// ReduceColor$ on an OptionalCost static (the Defiler cycle): the colour whose
	// one pip the paid optional cost removes.
	PKReduceColor
	PKReduceCost
	PKReflectProperty
	PKRegeneration
	PKRelative
	PKRelativeTarget
	PKRememberAffected
	PKRememberAltered
	PKRememberAmass
	PKRememberAmount
	PKRememberAnimated
	PKRememberAttached
	PKRememberCards
	PKRememberChain
	PKRememberChanged
	PKRememberChosen
	PKRememberClasher
	PKRememberCloaked
	PKRememberControlled
	PKRememberCopies
	PKRememberCostMana
	PKRememberCountered
	PKRememberCounteredCMC
	PKRememberCounteredSA
	PKRememberDamaged
	PKRememberDestroyed
	PKRememberDifference
	PKRememberDiscarded
	PKRememberDiscardingPlayers
	PKRememberDiscovered
	PKRememberDrawn
	PKRememberEach
	PKRememberExchanged
	PKRememberForCounter
	PKRememberFound
	PKRememberGoaded
	PKRememberInvestigatingPlayers
	PKRememberKept
	PKRememberLKI
	PKRememberLoser
	PKRememberManifested
	PKRememberMilled
	PKRememberMoved
	// RememberMovedToZone$ on a Dig: the destination zones (1 = the primary arm, 2 =
	// DestinationZone2$) whose moved cards join Remembered. Read by effects'
	// compileDig.
	PKRememberMovedToZone
	PKRememberNewCard
	PKRememberNumber
	PKRememberObjects
	PKRememberOriginalTokens
	PKRememberOwnLoss
	PKRememberPeeked
	// Play's RememberPlayed$ rider: the card the Play began to play joins the
	// source's remembered set (Planetarium of Wan Shi Tong's "only once each turn";
	// append-only).
	PKRememberPlayed
	PKRememberPumped
	PKRememberPut
	PKRememberRemovedCards
	PKRememberRemovedFromCombat
	PKRememberResult
	PKRememberRevealed
	// ImmediateTrigger's RememberSVarAmount$ rider (New Way Forward): the
	// per-trigger remembered Integer the reflexive ability's
	// Count$TriggerRememberAmount reads.
	PKRememberSVarAmount
	PKRememberSacrificed
	PKRememberSearcher
	// RememberSource$ True on a Token (Saproling Burst): each minted token's memory
	// names the creating source. Read by effects' compileToken.
	PKRememberSource
	PKRememberTapped
	PKRememberTargets
	PKRememberTokens
	PKRememberUntapped
	PKRememberVotedObjects
	PKRememberingAttacker
	PKRemoveAllAbilities
	// RemoveArtifactTypes$ and RemoveEnchantmentTypes$ True: the layer-4 strips of
	// those subtype sets.
	PKRemoveArtifactTypes
	PKRemoveCardTypes
	PKRemoveConditionSVar
	PKRemoveCost
	PKRemoveCreatureTypes
	PKRemoveEnchantmentTypes
	PKRemoveKeyword
	PKRemoveKeywords
	PKRemoveLandTypes
	PKRemoveType
	PKRemoveTypes
	PKRenown
	PKReorder
	PKRepeatCards
	PKRepeatCheckSVar
	PKRepeatCompare
	PKRepeatDefined
	PKRepeatOptional
	PKRepeatOptionalDecider
	PKRepeatOptionalForEachPlayer
	PKRepeatOptionalMessage
	PKRepeatPlayers
	PKRepeatPresent
	PKRepeatSVarCompare
	PKRepeatSpellAbilities
	PKRepeatSubAbility
	PKRepeatTargeted
	PKRepeatTypesFrom
	PKReplaceColor
	PKReplaceDyingDefined
	PKReplaceGraveyard
	PKReplaceGraveyardValid
	PKReplaceMana
	PKReplaceOnly
	PKReplaceType
	PKReplaceWith
	PKReplacementEffects
	PKReplacementResult
	PKReplacements
	PKResolvedLimit
	PKResolvedOnly
	PKRestRandomOrder
	PKRestrictFromValid
	PKRestrictFromZone
	PKRestrictToRemembered
	PKRestrictValid
	PKResult
	PKResultSVar
	PKResultSubAbilities
	// ChangeZone's Duration$ UntilHostLeavesPlay return rider (Lagrella, the
	// Magpie): ReturnAbility$ names the SVar run on each returned card that
	// ReturnValid$ matches.
	PKReturnAbility
	PKReturnValid
	PKReveal
	PKRevealAllValid
	PKRevealDefined
	PKRevealNumber
	PKRevealOptional
	PKRevealType
	PKRevealValid
	PKRevealed
	PKRevolt
	PKRolledToVisitAttractions
	PKRoomName
	PKSVar
	PKSVarCompare
	// SacEachValid$ True on a Sacrifice whose SacValid$ is a " & " list (Tomb of
	// Annihilation's Oubliette room, Catch // Release): True sacrifices one
	// permanent per listed type, part by part. Read by effects' sacrificeParams
	// (sacEachValidParts).
	PKSacEachValid
	PKSacValid
	PKSameNameAsEach
	PKSecondary
	PKSecondaryType
	PKSecretly
	PKSelectPrompt
	PKSeparator
	PKSetChosenMode
	PKSetChosenNumber
	PKSetColor
	PKSetLoyalty
	PKSetMaxHandSize
	PKSetName
	PKSetPower
	PKSetToughness
	PKShareLandType
	// A Mode$ Continuous static's SharedKeywordsZone$/SharedRestrictions$ filter
	// (Eater of Virtue, Cairn Wanderer): the AddKeyword$ list is narrowed to the
	// keywords a card in those zones matching the restriction has.
	PKSharedKeywordsZone
	PKSharedRestrictions
	PKShieldEffectTarget
	PKShowCards
	PKShowMilledCards
	PKShowSacrificedCards
	PKShuffle
	// ShuffleBySelfControlled$ True on a Mode$ Shuffled trigger (Widespread Panic's
	// "causes its controller to shuffle"). Read by trigmatch's shuffledMatches.
	PKShuffleBySelfControlled
	PKShuffleNonMandatory
	PKSkip
	PKSkipUntap
	PKSorcerySpeed
	PKSorcerySpeed2
	PKSource
	PKSpellDescription
	PKStackDescription
	PKStackable
	// StartBidding$ on a BidLife: the opening bid as written ("", a literal number,
	// or Any). Read by effects' effBidLife.
	PKStartBidding
	PKStartingWith
	PKStatic
	PKStaticAbilities
	PKStaticCommandCheckSVar
	PKStaticCommandSVarCompare
	PKStaticEffect
	PKStaticEffectCheckSVar
	PKStaticEffectSVarCompare
	PKStoreVoteNum
	PKStrictAmount
	PKSubAbility
	PKSupport
	PKTailsSubAbility
	PKTapCreaturesForMana
	PKTapped
	PKTapper
	PKTapperController
	PKTarget
	PKTargetMax
	PKTargetMin
	PKTargetRestriction
	PKTargetType
	PKTargetUnique
	PKTargetValidTargeting
	PKTargetingPlayer
	PKTargetingPlayerControls
	PKTargetsAtRandom
	PKTargetsForEachPlayer
	PKTargetsValid
	PKTargetsWithControllerProperty
	PKTargetsWithDefinedController
	PKTargetsWithDifferentCMC
	PKTargetsWithDifferentControllers
	PKTargetsWithDifferentNames
	PKTargetsWithEqualToughness
	PKTargetsWithRelatedProperty
	PKTargetsWithSameCardType
	PKTargetsWithSameController
	PKTargetsWithSameCreatureType
	PKTargetsWithSharedCardType
	PKTargetsWithSharedTypes
	PKTeamwork
	PKTempRemember
	PKTgtPrompt
	PKTgtPrompt2
	PKTgtZone
	PKThisDoor
	PKThisTurn
	PKThreshold
	PKToBottom
	PKToEachOther
	PKTokenAttacking
	// PKTokenBlocking is TokenBlocking$ (CR 509.4a): the Defined$ selector naming
	// the attacking creature a created token enters blocking. Read by effects'
	// compileToken and compileCopyPermanent.
	PKTokenBlocking
	PKTokenOwner
	PKTokenRemembered
	PKTokenScript
	PKTokenTapped
	// TopCardOfLibraryIs$ <filter> on a Mode$ Continuous static: the grant lives
	// while the static's controller's library is non-empty and its top card matches
	// the filter (Mul Daya Channelers, Vampire Nocturnus; CR 611.3a). Read by rules
	// continuousGateHolds.
	PKTopCardOfLibraryIs
	PKToughness
	PKTraining
	PKTransformed
	// PutCounter's Tribute$ marker (CR 702.104's tribute keyword expansion,
	// kw_tribute.go): the put is the tribute offer -- the entering creature's
	// controller picks the opponent, the chosen opponent elects to pay the tribute,
	// and an acceptance folds the events.AlterAttribute "Tributed" designation
	// (append-only).
	PKTribute
	PKTrigger
	PKTriggerController
	PKTriggerDescription
	PKTriggerZones
	// Mode$ AbilityTriggered operands (append-only).
	PKTriggeredOwnAbility
	PKTriggers
	PKTriggersWhenSpent
	PKTrueSubAbility
	PKTwoColors
	PKType
	PKTypeLimit
	PKTypes
	PKUltimate
	PKUnattach
	PKUnchosenPile
	PKUnearth
	PKUnimprint
	PKUnlessCost
	PKUnlessDefender
	PKUnlessPayer
	PKUnlessResolveSubs
	PKUnlessSwitched
	PKUnlessType
	PKUnlessValidTarget
	PKUnmatchedAbility
	PKUntap
	PKUntapType
	PKUntapUpTo
	PKUpTo
	PKUpcomingTurn
	PKUpto
	PKUseAllOriginZones
	PKUseDifferenceBetweenRolls
	PKUseHighestRoll
	PKUseImprinted
	PKValiant
	PKValid
	PKValidActivatingPlayer
	PKValidActivator
	// PKValidAfterStack is ValidAfterStack$ (Keeper of the Dead's activation- time
	// target filter).
	PKValidAfterStack
	PKValidAmountEach
	PKValidAttachment
	PKValidAttacked
	PKValidAttacker
	PKValidAttackers
	PKValidAttackersAmount
	PKValidAttackingPlayer
	PKValidBlocked
	PKValidBlocker
	// Mode$ AttackerBlocked's "blocked by two or more creatures" comparator (Seifer,
	// Balamb Rival; append-only).
	PKValidBlockerAmount
	PKValidCard
	// ValidCardToTarget$ on an S:Mode$ CantAttach line that scopes the ATTACHING
	// object by targeting legality rather than by a ValidCard$ filter (Guardian
	// Beast's "ValidCardToTarget$ Aura.!Attached"). Read by
	// cards.CantAttachAuraShape and rules' cantAttachAuraBlocks.
	PKValidCardToTarget
	PKValidCards
	PKValidCards2
	PKValidCause
	// ValidCauser$ on a Mode$ Destroyed trigger (Karmic Justice's "a spell or
	// ability an opponent controls"): the player controlling the destroying stack
	// object. Read by trigmatch's destroyedMatches.
	PKValidCauser
	PKValidChoices
	PKValidCounterType
	PKValidCreature
	PKValidCrew
	PKValidCrewAmount
	PKValidDefender
	PKValidDefenders
	PKValidDefined
	PKValidDescription
	PKValidDestination
	PKValidEnlisted
	PKValidEntity
	PKValidExplored
	PKValidExplorer
	// ValidKeyword$ <Landwalk:Type> on S:Mode$ IgnoreLandWalk, read by
	// combat.landwalkIgnored.
	PKValidKeyword
	PKValidLKI
	PKValidLoseReason
	PKValidMode
	PKValidNewController
	PKValidObject
	PKValidObjectToSource
	PKValidOriginalController
	PKValidPlayer
	PKValidPlayers
	PKValidResult
	PKValidSA
	PKValidSAonCard
	PKValidSaddled
	PKValidSides
	PKValidSource
	PKValidSpell
	PKValidSpellAbility
	PKValidStepTurnToController
	PKValidSupportedCopy
	PKValidTarget
	PKValidTgts
	// ValidTgtsDesc$: the script's noun for the legal target set; presentation only
	// (append-only).
	PKValidTgtsDesc
	PKValidToken
	PKValidTrigger
	PKValidTurned
	PKValidTypes
	PKValidZone
	PKValue
	PKVarName
	PKVarValue
	PKVerb
	PKVoteCard
	PKVoteMessage
	PKVotePlayer
	PKVoteSubAbility
	PKVoteTiedAbility
	PKWard
	PKWerewolfTransformCondition
	PKWerewolfUntransformCondition
	PKWinSubAbility
	PKWithCountersAmount
	PKWithCountersType
	PKWithDifferentNames
	PKWithMayLook
	PKWithNotedCounters
	PKWithTotalCMC
	PKWithTotalCardTypes
	PKWithoutManaCost
	PKWon
	PKWontPhaseInNormal
	PKXChoice
	// XColor$ names the colour(s) the {X} part of a cost must be paid with (Forge's
	// SpellAbility.getXColor; Consume Spirit, Crimson Hellkite). Read by
	// pay.WithSpellAbilityExtras / AbilityCompiledCost into rules/cost.Cost.XColors.
	PKXColor
	PKXMax
	PKXMin
	PKZone
	PKsVars
	PKstaticAbilities
	paramKeyCount
)

// paramKeyCap bounds the vocabulary: 2048 keys, room for every parameter
// name the corpus uses. It is a budget, not a representation limit (ParamKey
// is uint16): a ParamSet carries paramMaskWords mask words plus a uint16 rank
// per word, so the vocabulary's size is paid on every compiled node
// (~85k over the corpus). Raising it is a memory decision; see
// TestParamKeyRoomRemains and TestParamSetSizeTracksVocabulary.
const paramKeyCap = 2048

// The vocabulary fits the budget.
const _ = uint(paramKeyCap - int(paramKeyCount))

// paramMaskWords is a ParamMask's width in 64-bit words: just enough for the
// declared vocabulary (ordinals 0..paramKeyCount-1), so a ParamSet is no
// wider than the keys that exist.
//
// The dense mask was chosen over a two-level (summary word plus dense
// non-zero words) layout by measurement over the corpus's 85k compiled
// nodes: the two-level read is one more dependent load and measured about
// twice as slow on a cold sweep, while the dense mask is the same size as the
// uint8-era set until the vocabulary passes 256 keys and only overtakes the
// two-level layout's memory past ~640 keys.
const paramMaskWords = (int(paramKeyCount) + 63) / 64

// paramKeyNames maps each ParamKey to its Forge key text.
var paramKeyNames = [paramKeyCount]string{
	PKAILogic:                          "AILogic",
	PKAIManaPref:                       "AIManaPref",
	PKAbilities:                        "Abilities",
	PKActivate:                         "Activate",
	PKActivation:                       "Activation",
	PKActivationAfterBlockers:          "ActivationAfterBlockers",
	PKActivationFirstCombat:            "ActivationFirstCombat",
	PKActivationGameTypes:              "ActivationGameTypes",
	PKActivationLimit:                  "ActivationLimit",
	PKActivationPhases:                 "ActivationPhases",
	PKActivationZone:                   "ActivationZone",
	PKActivator:                        "Activator",
	PKActivatorThisTurnCast:            "ActivatorThisTurnCast",
	PKActivatorThisTurnCastEach:        "ActivatorThisTurnCastEach",
	PKActivePhases:                     "ActivePhases",
	PKActiveZones:                      "ActiveZones",
	PKAdamant:                          "Adamant",
	PKAdapt:                            "Adapt",
	PKAddAbilities:                     "AddAbilities",
	PKAddAbility:                       "AddAbility",
	PKAddAllCreatureTypes:              "AddAllCreatureTypes",
	PKAddColor:                         "AddColor",
	PKAddColors:                        "AddColors",
	PKAddKWs:                           "AddKWs",
	PKAddKeyword:                       "AddKeyword",
	PKAddKeywords:                      "AddKeywords",
	PKAddPower:                         "AddPower",
	PKAddReplacementEffect:             "AddReplacementEffect",
	PKAddSVar:                          "AddSVar",
	PKAddSVars:                         "AddSVars",
	PKAddStaticAbilities:               "AddStaticAbilities",
	PKAddStaticAbility:                 "AddStaticAbility",
	PKAddToughness:                     "AddToughness",
	PKAddTrigger:                       "AddTrigger",
	PKAddTriggers:                      "AddTriggers",
	PKAddType:                          "AddType",
	PKAddTypes:                         "AddTypes",
	PKAddsCounters:                     "AddsCounters",
	PKAddsKeywords:                     "AddsKeywords",
	PKAddsKeywordsUntil:                "AddsKeywordsUntil",
	PKAddsKeywordsValid:                "AddsKeywordsValid",
	PKAddsNoCounter:                    "AddsNoCounter",
	PKAdjustLandPlays:                  "AdjustLandPlays",
	PKAffected:                         "Affected",
	PKAffectedZone:                     "AffectedZone",
	PKAfterPhase:                       "AfterPhase",
	PKAllCards:                         "AllCards",
	PKAllCounterTypes:                  "AllCounterTypes",
	PKAllCounters:                      "AllCounters",
	PKAllValid:                         "AllValid",
	PKAllowRepeats:                     "AllowRepeats",
	PKAlone:                            "Alone",
	PKAlternateCost:                    "AlternateCost",
	PKAlternativeCost:                  "AlternativeCost",
	PKAlternativeDecider:               "AlternativeDecider",
	PKAlwaysRemember:                   "AlwaysRemember",
	PKAmount:                           "Amount",
	PKAmountByChosenMap:                "AmountByChosenMap",
	PKAmountFromVotes:                  "AmountFromVotes",
	PKAnnihilator:                      "Annihilator",
	PKAnnounce:                         "Announce",
	PKAnnounceTitle:                    "AnnounceTitle",
	PKAnyNumber:                        "AnyNumber",
	PKAtEOT:                            "AtEOT",
	PKAtEOTTrig:                        "AtEOTTrig",
	PKAtLeastOneInstance:               "AtLeastOneInstance",
	PKAtRandom:                         "AtRandom",
	PKAttachedTo:                       "AttachedTo",
	PKAttachedToPlayer:                 "AttachedToPlayer",
	PKAttacked:                         "Attacked",
	PKAttackedTarget:                   "AttackedTarget",
	PKAttacker:                         "Attacker",
	PKAttacking:                        "Attacking",
	PKAttackingPlayer:                  "AttackingPlayer",
	PKAttributes:                       "Attributes",
	PKBecomeStartingPlayer:             "BecomeStartingPlayer",
	PKBeginTurn:                        "BeginTurn",
	PKBidSubAbility:                    "BidSubAbility",
	PKBlessing:                         "Blessing",
	PKBlockAllDefined:                  "BlockAllDefined",
	PKBoast:                            "Boast",
	PKBolster:                          "Bolster",
	PKBoon:                             "Boon",
	PKBoonAmount:                       "BoonAmount",
	PKBranchConditionSVar:              "BranchConditionSVar",
	PKBranchConditionSVarCompare:       "BranchConditionSVarCompare",
	PKCanBlockAmount:                   "CanBlockAmount",
	PKCanBlockAny:                      "CanBlockAny",
	PKCanRepeatModes:                   "CanRepeatModes",
	PKCantHaveKeyword:                  "CantHaveKeyword",
	PKCaster:                           "Caster",
	PKChangeColorWord:                  "ChangeColorWord",
	PKChangeController:                 "ChangeController",
	PKChangeLater:                      "ChangeLater",
	PKChangeNum:                        "ChangeNum",
	PKChangeSingleTarget:               "ChangeSingleTarget",
	PKChangeType:                       "ChangeType",
	PKChangeTypeWord:                   "ChangeTypeWord",
	PKChangeValid:                      "ChangeValid",
	PKChangeZoneTable:                  "ChangeZoneTable",
	PKCharacteristicDefining:           "CharacteristicDefining",
	PKCharmNum:                         "CharmNum",
	PKCheckDefinedPlayer:               "CheckDefinedPlayer",
	PKCheckOnTriggeredCard:             "CheckOnTriggeredCard",
	PKCheckSVar:                        "CheckSVar",
	PKCheckSecondSVar:                  "CheckSecondSVar",
	PKChoiceAmount:                     "ChoiceAmount",
	PKChoiceNum:                        "ChoiceNum",
	PKChoiceOptional:                   "ChoiceOptional",
	PKChoicePrompt:                     "ChoicePrompt",
	PKChoiceRestriction:                "ChoiceRestriction",
	PKChoiceTitle:                      "ChoiceTitle",
	PKChoiceZone:                       "ChoiceZone",
	PKChoices:                          "Choices",
	PKChoose:                           "Choose",
	PKChooseAmount:                     "ChooseAmount",
	PKChooseCounter:                    "ChooseCounter",
	PKChooseDifferent:                  "ChooseDifferent",
	PKChooseEach:                       "ChooseEach",
	PKChooseFromDefined:                "ChooseFromDefined",
	PKChooseFromDefinedCards:           "ChooseFromDefinedCards",
	PKChooseFromList:                   "ChooseFromList",
	PKChooseOrder:                      "ChooseOrder",
	PKChooser:                          "Chooser",
	PKChosenPile:                       "ChosenPile",
	PKChosenSVar:                       "ChosenSVar",
	PKChosenZone:                       "ChosenZone",
	PKCipherCopy:                       "CipherCopy",
	PKClassBand:                        "ClassBand",
	PKClassLevel:                       "ClassLevel",
	PKClearChosenCard:                  "ClearChosenCard",
	PKClearChosenColor:                 "ClearChosenColor",
	PKClearChosenPlayer:                "ClearChosenPlayer",
	PKClearImprinted:                   "ClearImprinted",
	PKClearNamedCard:                   "ClearNamedCard",
	PKClearNotedCardsFor:               "ClearNotedCardsFor",
	PKClearRemembered:                  "ClearRemembered",
	PKClearRememberedBeforeLoop:        "ClearRememberedBeforeLoop",
	PKColor:                            "Color",
	PKColorOrType:                      "ColorOrType",
	PKColors:                           "Colors",
	PKColorsFrom:                       "ColorsFrom",
	PKCombatDamage:                     "CombatDamage",
	PKCondition:                        "Condition",
	PKConditionActivationLimit:         "ConditionActivationLimit",
	PKConditionCheckSVar:               "ConditionCheckSVar",
	PKConditionCompare:                 "ConditionCompare",
	PKConditionCompare2:                "ConditionCompare2",
	PKConditionDefined:                 "ConditionDefined",
	PKConditionFirstCombat:             "ConditionFirstCombat",
	PKConditionNotPresent:              "ConditionNotPresent",
	PKConditionPhases:                  "ConditionPhases",
	PKConditionPlayerContains:          "ConditionPlayerContains",
	PKConditionPlayerDefined:           "ConditionPlayerDefined",
	PKConditionPlayerTurn:              "ConditionPlayerTurn",
	PKConditionPresent:                 "ConditionPresent",
	PKConditionPresent2:                "ConditionPresent2",
	PKConditionSVarCompare:             "ConditionSVarCompare",
	PKConditionZone:                    "ConditionZone",
	PKControlOpponentsSearchingLibrary: "ControlOpponentsSearchingLibrary",
	PKControlledByPlayer:               "ControlledByPlayer",
	PKController:                       "Controller",
	PKControllerUntaps:                 "ControllerUntaps",
	PKCopyCard:                         "CopyCard",
	PKCost:                             "Cost",
	PKCounterAmount:                    "CounterAmount",
	PKCounterNum:                       "CounterNum",
	PKCounterNumPerDefined:             "CounterNumPerDefined",
	PKCounterType:                      "CounterType",
	PKCounterType2:                     "CounterType2",
	PKCounterTypeChoice:                "CounterTypeChoice",
	PKCounterTypePerDefined:            "CounterTypePerDefined",
	PKDamageAmount:                     "DamageAmount",
	PKDamageMap:                        "DamageMap",
	PKDamageSource:                     "DamageSource",
	PKDamageTarget:                     "DamageTarget",
	PKDayTime:                          "DayTime",
	PKDefined:                          "Defined",
	PKDefinedAttacker:                  "DefinedAttacker",
	PKDefinedCards:                     "DefinedCards",
	PKDefinedDamagers:                  "DefinedDamagers",
	PKDefinedExiler:                    "DefinedExiler",
	PKDefinedKW:                        "DefinedKW",
	PKDefinedMagnet:                    "DefinedMagnet",
	PKDefinedName:                      "DefinedName",
	PKDefinedPiles:                     "DefinedPiles",
	PKDefinedPlayer:                    "DefinedPlayer",
	PKDefinedPlayerChooses:             "DefinedPlayerChooses",
	PKDefinedTarget:                    "DefinedTarget",
	PKDelayedTriggerDefinedPlayer:      "DelayedTriggerDefinedPlayer",
	PKDelirium:                         "Delirium",
	PKDescription:                      "Description",
	PKDestAltSVar:                      "DestAltSVar",
	PKDestAltSVarCompare:               "DestAltSVarCompare",
	PKDestination:                      "Destination",
	PKDestinationAlternative:           "DestinationAlternative",
	PKDestinationZone:                  "DestinationZone",
	PKDestinationZone2:                 "DestinationZone2",
	PKDethrone:                         "Dethrone",
	PKDifferentNames:                   "DifferentNames",
	PKDigNum:                           "DigNum",
	PKDiscard:                          "Discard",
	PKDiscardValid:                     "DiscardValid",
	PKDiscardValidDesc:                 "DiscardValidDesc",
	PKDivideEvenly:                     "DivideEvenly",
	PKDividedAsYouChoose:               "DividedAsYouChoose",
	PKDividedRandomly:                  "DividedRandomly",
	PKDontPlaneswalkAway:               "DontPlaneswalkAway",
	PKDrawLimit:                        "DrawLimit",
	PKDungeon:                          "Dungeon",
	PKDuration:                         "Duration",
	PKETB:                              "ETB",
	PKEach:                             "Each",
	PKEachDifferentPower:               "EachDifferentPower",
	PKEachExistingCounter:              "EachExistingCounter",
	PKEachFromSource:                   "EachFromSource",
	PKEachToItself:                     "EachToItself",
	PKEcho:                             "Echo",
	PKEffectOnly:                       "EffectOnly",
	PKEffectOwner:                      "EffectOwner",
	PKEffectZone:                       "EffectZone",
	PKEmbalm:                           "Embalm",
	PKEnduringStory:                    "EnduringStory",
	PKEvenOddResults:                   "EvenOddResults",
	PKEvolve:                           "Evolve",
	PKExactly:                          "Exactly",
	PKExcessDamage:                     "ExcessDamage",
	PKExcessDamageCondition:            "ExcessDamageCondition",
	PKExcessSVar:                       "ExcessSVar",
	PKExcessSVarCondition:              "ExcessSVarCondition",
	PKExclude:                          "Exclude",
	PKExcludeZone:                      "ExcludeZone",
	PKExcludedOrigins:                  "ExcludedOrigins",
	PKExecute:                          "Execute",
	PKExhaust:                          "Exhaust",
	PKExileFaceDown:                    "ExileFaceDown",
	PKExileOnMoved:                     "ExileOnMoved",
	PKExiledWithEffectSource:           "ExiledWithEffectSource",
	PKExpression:                       "Expression",
	PKExtraPhase:                       "ExtraPhase",
	PKExtraPhaseDelayedTrigger:         "ExtraPhaseDelayedTrigger",
	PKExtraPhaseDelayedTriggerExcute:   "ExtraPhaseDelayedTriggerExcute",
	PKExtraTurn:                        "ExtraTurn",
	PKExtraTurnDelayedTrigger:          "ExtraTurnDelayedTrigger",
	PKExtraTurnDelayedTriggerExecute:   "ExtraTurnDelayedTriggerExecute",
	PKFaceDown:                         "FaceDown",
	PKFaceDownPower:                    "FaceDownPower",
	PKFaceDownSetType:                  "FaceDownSetType",
	PKFaceDownToughness:                "FaceDownToughness",
	PKFallbackAbility:                  "FallbackAbility",
	PKFalseSubAbility:                  "FalseSubAbility",
	PKFirstAttack:                      "FirstAttack",
	PKFirstCardInDrawStep:              "FirstCardInDrawStep",
	PKFirstCombat:                      "FirstCombat",
	PKFirstExtraCardDrawnThisTurn:      "FirstExtraCardDrawnThisTurn",
	PKFirstForetell:                    "FirstForetell",
	PKFirstTime:                        "FirstTime",
	PKFirstTimeCrewed:                  "FirstTimeCrewed",
	PKFirstTimeSaddled:                 "FirstTimeSaddled",
	PKFirstUpkeep:                      "FirstUpkeep",
	PKFlipUntilYouLose:                 "FlipUntilYouLose",
	PKFlipper:                          "Flipper",
	PKFollowedBy:                       "FollowedBy",
	PKForCost:                          "ForCost",
	PKForEachPlayer:                    "ForEachPlayer",
	PKForEachShard:                     "ForEachShard",
	PKForbiddenNewTypes:                "ForbiddenNewTypes",
	PKForetold:                         "Foretold",
	PKForgetChanged:                    "ForgetChanged",
	PKForgetChosen:                     "ForgetChosen",
	PKForgetCounter:                    "ForgetCounter",
	PKForgetImprinted:                  "ForgetImprinted",
	PKForgetObjects:                    "ForgetObjects",
	PKForgetOnCast:                     "ForgetOnCast",
	PKForgetOnMoved:                    "ForgetOnMoved",
	PKForgetOnPhasedIn:                 "ForgetOnPhasedIn",
	PKForgetOtherRemembered:            "ForgetOtherRemembered",
	PKForgetOtherTargets:               "ForgetOtherTargets",
	PKForgetPlayed:                     "ForgetPlayed",
	PKFound:                            "Found",
	PKFoundSearchingLibrary:            "FoundSearchingLibrary",
	PKGainControl:                      "GainControl",
	PKGains:                            "Gains",
	PKGainsAbilitiesLimitPerTurn:       "GainsAbilitiesLimitPerTurn",
	PKGainsAbilitiesOf:                 "GainsAbilitiesOf",
	PKGainsAbilitiesOfDefined:          "GainsAbilitiesOfDefined",
	PKGainsAbilitiesOfZones:            "GainsAbilitiesOfZones",
	PKGainsTriggerAbsOf:                "GainsTriggerAbsOf",
	PKGainsValidAbilities:              "GainsValidAbilities",
	PKGameActivationLimit:              "GameActivationLimit",
	PKGoad:                             "Goad",
	PKHaunt:                            "Haunt",
	PKHeadsSubAbility:                  "HeadsSubAbility",
	PKHellbent:                         "Hellbent",
	PKHidden:                           "Hidden",
	PKHiddenKeywords:                   "HiddenKeywords",
	PKIgnoreFreeze:                     "IgnoreFreeze",
	PKIgnoreGeneric:                    "IgnoreGeneric",
	PKImprint:                          "Imprint",
	PKImprintCards:                     "ImprintCards",
	PKImprintChosen:                    "ImprintChosen",
	PKImprintFound:                     "ImprintFound",
	PKImprintLast:                      "ImprintLast",
	PKImprintOnHost:                    "ImprintOnHost",
	PKImprintPlayed:                    "ImprintPlayed",
	PKImprintRest:                      "ImprintRest",
	PKImprintTokens:                    "ImprintTokens",
	PKIncrement:                        "Increment",
	PKInstantSpeed:                     "InstantSpeed",
	PKIntoPlayTapped:                   "IntoPlayTapped",
	PKInvalidTypes:                     "InvalidTypes",
	PKIsCombat:                         "IsCombat",
	PKIsDamage:                         "IsDamage",
	PKIsPresent:                        "IsPresent",
	PKIsPresent2:                       "IsPresent2",
	PKIsSingleTarget:                   "IsSingleTarget",
	PKKW:                               "KW",
	PKKWChoice:                         "KWChoice",
	PKKeepRemembered:                   "KeepRemembered",
	PKKeyword:                          "Keyword",
	PKKeywordLine:                      "KeywordLine",
	PKKeywords:                         "Keywords",
	PKLayer:                            "Layer",
	PKLeaveBattlefield:                 "LeaveBattlefield",
	PKLeftRightPile:                    "LeftRightPile",
	PKLevel:                            "Level",
	PKLibraryPosition:                  "LibraryPosition",
	PKLibraryPositionAlternative:       "LibraryPositionAlternative",
	PKLifeAmount:                       "LifeAmount",
	PKLifeTotal:                        "LifeTotal",
	PKList:                             "List",
	PKListTitle:                        "ListTitle",
	PKLook:                             "Look",
	PKLoseControl:                      "LoseControl",
	PKLoseSubAbility:                   "LoseSubAbility",
	PKManaAmount:                       "ManaAmount",
	PKManaConversion:                   "ManaConversion",
	PKManaNotSpent:                     "ManaNotSpent",
	PKManaSpent:                        "ManaSpent",
	PKMandatory:                        "Mandatory",
	PKMatchedAbility:                   "MatchedAbility",
	PKMax:                              "Max",
	PKMaxAttackers:                     "MaxAttackers",
	PKMaxNum:                           "MaxNum",
	PKMaxRevealed:                      "MaxRevealed",
	PKMaxRollsResults:                  "MaxRollsResults",
	PKMaxTotalTargetCMC:                "MaxTotalTargetCMC",
	PKMaxTotalTargetPower:              "MaxTotalTargetPower",
	PKMayChooseTarget:                  "MayChooseTarget",
	PKMayLookAt:                        "MayLookAt",
	PKMayPlay:                          "MayPlay",
	PKMayPlayAltManaCost:               "MayPlayAltManaCost",
	PKMayPlayLimit:                     "MayPlayLimit",
	PKMayPlayPlayer:                    "MayPlayPlayer",
	PKMayPlayText:                      "MayPlayText",
	PKMayPlayWithFlash:                 "MayPlayWithFlash",
	PKMayPlayWithoutManaCost:           "MayPlayWithoutManaCost",
	PKMayShuffle:                       "MayShuffle",
	PKMentor:                           "Mentor",
	PKMetalcraft:                       "Metalcraft",
	PKMin:                              "Min",
	PKMinAmount:                        "MinAmount",
	PKMinCharmNum:                      "MinCharmNum",
	PKMinLimit:                         "MinLimit",
	PKMinMana:                          "MinMana",
	PKMode:                             "Mode",
	PKModeCost:                         "ModeCost",
	PKMonstrosity:                      "Monstrosity",
	PKMustAttack:                       "MustAttack",
	PKMyriad:                           "Myriad",
	PKName:                             "Name",
	PKNatural:                          "Natural",
	PKNewController:                    "NewController",
	PKNewCounterAmount:                 "NewCounterAmount",
	PKNextRoom:                         "NextRoom",
	PKNextTurn:                         "NextTurn",
	PKNoCall:                           "NoCall",
	PKNoLonger:                         "NoLonger",
	PKNoLooking:                        "NoLooking",
	PKNoPrevention:                     "NoPrevention",
	PKNoRegen:                          "NoRegen",
	PKNoResolvingCheck:                 "NoResolvingCheck",
	PKNoReveal:                         "NoReveal",
	PKNoShuffle:                        "NoShuffle",
	PKNonBasicSpell:                    "NonBasicSpell",
	PKNonLegendary:                     "NonLegendary",
	PKNotFirstCardInDrawStep:           "NotFirstCardInDrawStep",
	PKNotThisAbility:                   "NotThisAbility",
	PKNoteCards:                        "NoteCards",
	PKNoteCardsFor:                     "NoteCardsFor",
	PKNoteCounterDefined:               "NoteCounterDefined",
	PKNoteNumber:                       "NoteNumber",
	PKNum:                              "Num",
	PKNumAtt:                           "NumAtt",
	PKNumCards:                         "NumCards",
	PKNumDef:                           "NumDef",
	PKNumDmg:                           "NumDmg",
	PKNumLimitEachTurn:                 "NumLimitEachTurn",
	PKNumPhases:                        "NumPhases",
	PKNumRandomChoices:                 "NumRandomChoices",
	PKNumTurns:                         "NumTurns",
	PKNumber:                           "Number",
	PKObject:                           "Object",
	PKOncePerSource:                    "OncePerSource",
	PKOneOff:                           "OneOff",
	PKOnlyFirst:                        "OnlyFirst",
	PKOnlyFirstSpell:                   "OnlyFirstSpell",
	PKOpponentTurn:                     "OpponentTurn",
	PKOptionQuestion:                   "OptionQuestion",
	PKOptional:                         "Optional",
	PKOptionalAbilityPrompt:            "OptionalAbilityPrompt",
	PKOptionalDecider:                  "OptionalDecider",
	PKOptionalPrompt:                   "OptionalPrompt",
	PKOrColors:                         "OrColors",
	PKOrderDuplicates:                  "OrderDuplicates",
	PKOrigin:                           "Origin",
	PKOriginAlternative:                "OriginAlternative",
	PKOtherBidder:                      "OtherBidder",
	PKOtherSVar:                        "OtherSVar",
	PKOtherwiseSubAbility:              "OtherwiseSubAbility",
	PKOverwriteColors:                  "OverwriteColors",
	PKPaid:                             "Paid",
	PKPawprint:                         "Pawprint",
	PKPersistentMana:                   "PersistentMana",
	PKPersistentUntilEndOfCombat:       "PersistentUntilEndOfCombat",
	PKPhase:                            "Phase",
	PKPhaseCount:                       "PhaseCount",
	PKPhaseInOrOut:                     "PhaseInOrOut",
	PKPhaseout:                         "Phaseout",
	PKPhases:                           "Phases",
	PKPlacer:                           "Placer",
	PKPlaneswalker:                     "Planeswalker",
	PKPlayCost:                         "PlayCost",
	PKPlayer:                           "Player",
	PKPlayerChoices:                    "PlayerChoices",
	PKPlayerTurn:                       "PlayerTurn",
	PKPopulate:                         "Populate",
	PKPower:                            "Power",
	PKPowerUp:                          "PowerUp",
	PKPresentCompare:                   "PresentCompare",
	PKPresentDefined:                   "PresentDefined",
	PKPresentZone:                      "PresentZone",
	PKPrevent:                          "Prevent",
	PKPreventionEffect:                 "PreventionEffect",
	PKPreventionShield:                 "PreventionShield",
	PKPreventionSubAbility:             "PreventionSubAbility",
	PKPrimary:                          "Primary",
	PKProduced:                         "Produced",
	PKPromptToSkipOptionalAbility:      "PromptToSkipOptionalAbility",
	PKPumpDuration:                     "PumpDuration",
	PKPumpKeywords:                     "PumpKeywords",
	PKPumpZone:                         "PumpZone",
	PKRaiseMaxHandSize:                 "RaiseMaxHandSize",
	PKRaiseTo:                          "RaiseTo",
	PKRandom:                           "Random",
	PKRandomChange:                     "RandomChange",
	PKRandomCompare:                    "RandomCompare",
	PKRandomCompareSVar:                "RandomCompareSVar",
	PKRandomCopied:                     "RandomCopied",
	PKRandomNum:                        "RandomNum",
	PKRandomNumTargets:                 "RandomNumTargets",
	PKRandomOrder:                      "RandomOrder",
	PKRandomTarget:                     "RandomTarget",
	PKRandomTargetRestriction:          "RandomTargetRestriction",
	PKRandomType:                       "RandomType",
	PKRearrangePlayer:                  "RearrangePlayer",
	PKRedistribute:                     "Redistribute",
	PKReduceAmount:                     "ReduceAmount",
	PKReduceColor:                      "ReduceColor",
	PKReduceCost:                       "ReduceCost",
	PKReflectProperty:                  "ReflectProperty",
	PKRegeneration:                     "Regeneration",
	PKRelative:                         "Relative",
	PKRelativeTarget:                   "RelativeTarget",
	PKRememberAffected:                 "RememberAffected",
	PKRememberAltered:                  "RememberAltered",
	PKRememberAmass:                    "RememberAmass",
	PKRememberAmount:                   "RememberAmount",
	PKRememberAnimated:                 "RememberAnimated",
	PKRememberAttached:                 "RememberAttached",
	PKRememberCards:                    "RememberCards",
	PKRememberChain:                    "RememberChain",
	PKRememberChanged:                  "RememberChanged",
	PKRememberChosen:                   "RememberChosen",
	PKRememberClasher:                  "RememberClasher",
	PKRememberCloaked:                  "RememberCloaked",
	PKRememberControlled:               "RememberControlled",
	PKRememberCopies:                   "RememberCopies",
	PKRememberCostMana:                 "RememberCostMana",
	PKRememberCountered:                "RememberCountered",
	PKRememberCounteredCMC:             "RememberCounteredCMC",
	PKRememberCounteredSA:              "RememberCounteredSA",
	PKRememberDamaged:                  "RememberDamaged",
	PKRememberDestroyed:                "RememberDestroyed",
	PKRememberDifference:               "RememberDifference",
	PKRememberDiscarded:                "RememberDiscarded",
	PKRememberDiscardingPlayers:        "RememberDiscardingPlayers",
	PKRememberDiscovered:               "RememberDiscovered",
	PKRememberDrawn:                    "RememberDrawn",
	PKRememberEach:                     "RememberEach",
	PKRememberExchanged:                "RememberExchanged",
	PKRememberForCounter:               "RememberForCounter",
	PKRememberFound:                    "RememberFound",
	PKRememberGoaded:                   "RememberGoaded",
	PKRememberInvestigatingPlayers:     "RememberInvestigatingPlayers",
	PKRememberKept:                     "RememberKept",
	PKRememberLKI:                      "RememberLKI",
	PKRememberLoser:                    "RememberLoser",
	PKRememberManifested:               "RememberManifested",
	PKRememberMilled:                   "RememberMilled",
	PKRememberMoved:                    "RememberMoved",
	PKRememberMovedToZone:              "RememberMovedToZone",
	PKRememberNewCard:                  "RememberNewCard",
	PKRememberNumber:                   "RememberNumber",
	PKRememberObjects:                  "RememberObjects",
	PKRememberOriginalTokens:           "RememberOriginalTokens",
	PKRememberOwnLoss:                  "RememberOwnLoss",
	PKRememberPeeked:                   "RememberPeeked",
	PKRememberPlayed:                   "RememberPlayed",
	PKRememberPumped:                   "RememberPumped",
	PKRememberPut:                      "RememberPut",
	PKRememberRemovedCards:             "RememberRemovedCards",
	PKRememberRemovedFromCombat:        "RememberRemovedFromCombat",
	PKRememberResult:                   "RememberResult",
	PKRememberRevealed:                 "RememberRevealed",
	PKRememberSVarAmount:               "RememberSVarAmount",
	PKRememberSacrificed:               "RememberSacrificed",
	PKRememberSearcher:                 "RememberSearcher",
	PKRememberSource:                   "RememberSource",
	PKRememberTapped:                   "RememberTapped",
	PKRememberTargets:                  "RememberTargets",
	PKRememberTokens:                   "RememberTokens",
	PKRememberUntapped:                 "RememberUntapped",
	PKRememberVotedObjects:             "RememberVotedObjects",
	PKRememberingAttacker:              "RememberingAttacker",
	PKRemoveAllAbilities:               "RemoveAllAbilities",
	PKRemoveArtifactTypes:              "RemoveArtifactTypes",
	PKRemoveCardTypes:                  "RemoveCardTypes",
	PKRemoveConditionSVar:              "RemoveConditionSVar",
	PKRemoveCost:                       "RemoveCost",
	PKRemoveCreatureTypes:              "RemoveCreatureTypes",
	PKRemoveEnchantmentTypes:           "RemoveEnchantmentTypes",
	PKRemoveKeyword:                    "RemoveKeyword",
	PKRemoveKeywords:                   "RemoveKeywords",
	PKRemoveLandTypes:                  "RemoveLandTypes",
	PKRemoveType:                       "RemoveType",
	PKRemoveTypes:                      "RemoveTypes",
	PKRenown:                           "Renown",
	PKReorder:                          "Reorder",
	PKRepeatCards:                      "RepeatCards",
	PKRepeatCheckSVar:                  "RepeatCheckSVar",
	PKRepeatCompare:                    "RepeatCompare",
	PKRepeatDefined:                    "RepeatDefined",
	PKRepeatOptional:                   "RepeatOptional",
	PKRepeatOptionalDecider:            "RepeatOptionalDecider",
	PKRepeatOptionalForEachPlayer:      "RepeatOptionalForEachPlayer",
	PKRepeatOptionalMessage:            "RepeatOptionalMessage",
	PKRepeatPlayers:                    "RepeatPlayers",
	PKRepeatPresent:                    "RepeatPresent",
	PKRepeatSVarCompare:                "RepeatSVarCompare",
	PKRepeatSpellAbilities:             "RepeatSpellAbilities",
	PKRepeatSubAbility:                 "RepeatSubAbility",
	PKRepeatTargeted:                   "RepeatTargeted",
	PKRepeatTypesFrom:                  "RepeatTypesFrom",
	PKReplaceColor:                     "ReplaceColor",
	PKReplaceDyingDefined:              "ReplaceDyingDefined",
	PKReplaceGraveyard:                 "ReplaceGraveyard",
	PKReplaceGraveyardValid:            "ReplaceGraveyardValid",
	PKReplaceMana:                      "ReplaceMana",
	PKReplaceOnly:                      "ReplaceOnly",
	PKReplaceType:                      "ReplaceType",
	PKReplaceWith:                      "ReplaceWith",
	PKReplacementEffects:               "ReplacementEffects",
	PKReplacementResult:                "ReplacementResult",
	PKReplacements:                     "Replacements",
	PKResolvedLimit:                    "ResolvedLimit",
	PKResolvedOnly:                     "ResolvedOnly",
	PKRestRandomOrder:                  "RestRandomOrder",
	PKRestrictFromValid:                "RestrictFromValid",
	PKRestrictFromZone:                 "RestrictFromZone",
	PKRestrictToRemembered:             "RestrictToRemembered",
	PKRestrictValid:                    "RestrictValid",
	PKResult:                           "Result",
	PKResultSVar:                       "ResultSVar",
	PKResultSubAbilities:               "ResultSubAbilities",
	PKReturnAbility:                    "ReturnAbility",
	PKReturnValid:                      "ReturnValid",
	PKReveal:                           "Reveal",
	PKRevealAllValid:                   "RevealAllValid",
	PKRevealDefined:                    "RevealDefined",
	PKRevealNumber:                     "RevealNumber",
	PKRevealOptional:                   "RevealOptional",
	PKRevealType:                       "RevealType",
	PKRevealValid:                      "RevealValid",
	PKRevealed:                         "Revealed",
	PKRevolt:                           "Revolt",
	PKRolledToVisitAttractions:         "RolledToVisitAttractions",
	PKRoomName:                         "RoomName",
	PKSVar:                             "SVar",
	PKSVarCompare:                      "SVarCompare",
	PKSacEachValid:                     "SacEachValid",
	PKSacValid:                         "SacValid",
	PKSameNameAsEach:                   "SameNameAsEach",
	PKSecondary:                        "Secondary",
	PKSecondaryType:                    "SecondaryType",
	PKSecretly:                         "Secretly",
	PKSelectPrompt:                     "SelectPrompt",
	PKSeparator:                        "Separator",
	PKSetChosenMode:                    "SetChosenMode",
	PKSetChosenNumber:                  "SetChosenNumber",
	PKSetColor:                         "SetColor",
	PKSetLoyalty:                       "SetLoyalty",
	PKSetMaxHandSize:                   "SetMaxHandSize",
	PKSetName:                          "SetName",
	PKSetPower:                         "SetPower",
	PKSetToughness:                     "SetToughness",
	PKShareLandType:                    "ShareLandType",
	PKSharedKeywordsZone:               "SharedKeywordsZone",
	PKSharedRestrictions:               "SharedRestrictions",
	PKShieldEffectTarget:               "ShieldEffectTarget",
	PKShowCards:                        "ShowCards",
	PKShowMilledCards:                  "ShowMilledCards",
	PKShowSacrificedCards:              "ShowSacrificedCards",
	PKShuffle:                          "Shuffle",
	PKShuffleBySelfControlled:          "ShuffleBySelfControlled",
	PKShuffleNonMandatory:              "ShuffleNonMandatory",
	PKSkip:                             "Skip",
	PKSkipUntap:                        "SkipUntap",
	PKSorcerySpeed:                     "SorcerySpeed",
	PKSorcerySpeed2:                    "SorcerySpeed2",
	PKSource:                           "Source",
	PKSpellDescription:                 "SpellDescription",
	PKStackDescription:                 "StackDescription",
	PKStackable:                        "Stackable",
	PKStartBidding:                     "StartBidding",
	PKStartingWith:                     "StartingWith",
	PKStatic:                           "Static",
	PKStaticAbilities:                  "StaticAbilities",
	PKStaticCommandCheckSVar:           "StaticCommandCheckSVar",
	PKStaticCommandSVarCompare:         "StaticCommandSVarCompare",
	PKStaticEffect:                     "StaticEffect",
	PKStaticEffectCheckSVar:            "StaticEffectCheckSVar",
	PKStaticEffectSVarCompare:          "StaticEffectSVarCompare",
	PKStoreVoteNum:                     "StoreVoteNum",
	PKStrictAmount:                     "StrictAmount",
	PKSubAbility:                       "SubAbility",
	PKSupport:                          "Support",
	PKTailsSubAbility:                  "TailsSubAbility",
	PKTapCreaturesForMana:              "TapCreaturesForMana",
	PKTapped:                           "Tapped",
	PKTapper:                           "Tapper",
	PKTapperController:                 "TapperController",
	PKTarget:                           "Target",
	PKTargetMax:                        "TargetMax",
	PKTargetMin:                        "TargetMin",
	PKTargetRestriction:                "TargetRestriction",
	PKTargetType:                       "TargetType",
	PKTargetUnique:                     "TargetUnique",
	PKTargetValidTargeting:             "TargetValidTargeting",
	PKTargetingPlayer:                  "TargetingPlayer",
	PKTargetingPlayerControls:          "TargetingPlayerControls",
	PKTargetsAtRandom:                  "TargetsAtRandom",
	PKTargetsForEachPlayer:             "TargetsForEachPlayer",
	PKTargetsValid:                     "TargetsValid",
	PKTargetsWithControllerProperty:    "TargetsWithControllerProperty",
	PKTargetsWithDefinedController:     "TargetsWithDefinedController",
	PKTargetsWithDifferentCMC:          "TargetsWithDifferentCMC",
	PKTargetsWithDifferentControllers:  "TargetsWithDifferentControllers",
	PKTargetsWithDifferentNames:        "TargetsWithDifferentNames",
	PKTargetsWithEqualToughness:        "TargetsWithEqualToughness",
	PKTargetsWithRelatedProperty:       "TargetsWithRelatedProperty",
	PKTargetsWithSameCardType:          "TargetsWithSameCardType",
	PKTargetsWithSameController:        "TargetsWithSameController",
	PKTargetsWithSameCreatureType:      "TargetsWithSameCreatureType",
	PKTargetsWithSharedCardType:        "TargetsWithSharedCardType",
	PKTargetsWithSharedTypes:           "TargetsWithSharedTypes",
	PKTeamwork:                         "Teamwork",
	PKTempRemember:                     "TempRemember",
	PKTgtPrompt:                        "TgtPrompt",
	PKTgtPrompt2:                       "TgtPrompt2",
	PKTgtZone:                          "TgtZone",
	PKThisDoor:                         "ThisDoor",
	PKThisTurn:                         "ThisTurn",
	PKThreshold:                        "Threshold",
	PKToBottom:                         "ToBottom",
	PKToEachOther:                      "ToEachOther",
	PKTokenAttacking:                   "TokenAttacking",
	PKTokenBlocking:                    "TokenBlocking",
	PKTokenOwner:                       "TokenOwner",
	PKTokenRemembered:                  "TokenRemembered",
	PKTokenScript:                      "TokenScript",
	PKTokenTapped:                      "TokenTapped",
	PKTopCardOfLibraryIs:               "TopCardOfLibraryIs",
	PKToughness:                        "Toughness",
	PKTraining:                         "Training",
	PKTransformed:                      "Transformed",
	PKTribute:                          "Tribute",
	PKTrigger:                          "Trigger",
	PKTriggerController:                "TriggerController",
	PKTriggerDescription:               "TriggerDescription",
	PKTriggerZones:                     "TriggerZones",
	PKTriggeredOwnAbility:              "TriggeredOwnAbility",
	PKTriggers:                         "Triggers",
	PKTriggersWhenSpent:                "TriggersWhenSpent",
	PKTrueSubAbility:                   "TrueSubAbility",
	PKTwoColors:                        "TwoColors",
	PKType:                             "Type",
	PKTypeLimit:                        "TypeLimit",
	PKTypes:                            "Types",
	PKUltimate:                         "Ultimate",
	PKUnattach:                         "Unattach",
	PKUnchosenPile:                     "UnchosenPile",
	PKUnearth:                          "Unearth",
	PKUnimprint:                        "Unimprint",
	PKUnlessCost:                       "UnlessCost",
	PKUnlessDefender:                   "UnlessDefender",
	PKUnlessPayer:                      "UnlessPayer",
	PKUnlessResolveSubs:                "UnlessResolveSubs",
	PKUnlessSwitched:                   "UnlessSwitched",
	PKUnlessType:                       "UnlessType",
	PKUnlessValidTarget:                "UnlessValidTarget",
	PKUnmatchedAbility:                 "UnmatchedAbility",
	PKUntap:                            "Untap",
	PKUntapType:                        "UntapType",
	PKUntapUpTo:                        "UntapUpTo",
	PKUpTo:                             "UpTo",
	PKUpcomingTurn:                     "UpcomingTurn",
	PKUpto:                             "Upto",
	PKUseAllOriginZones:                "UseAllOriginZones",
	PKUseDifferenceBetweenRolls:        "UseDifferenceBetweenRolls",
	PKUseHighestRoll:                   "UseHighestRoll",
	PKUseImprinted:                     "UseImprinted",
	PKValiant:                          "Valiant",
	PKValid:                            "Valid",
	PKValidActivatingPlayer:            "ValidActivatingPlayer",
	PKValidActivator:                   "ValidActivator",
	PKValidAfterStack:                  "ValidAfterStack",
	PKValidAmountEach:                  "ValidAmountEach",
	PKValidAttachment:                  "ValidAttachment",
	PKValidAttacked:                    "ValidAttacked",
	PKValidAttacker:                    "ValidAttacker",
	PKValidAttackers:                   "ValidAttackers",
	PKValidAttackersAmount:             "ValidAttackersAmount",
	PKValidAttackingPlayer:             "ValidAttackingPlayer",
	PKValidBlocked:                     "ValidBlocked",
	PKValidBlocker:                     "ValidBlocker",
	PKValidBlockerAmount:               "ValidBlockerAmount",
	PKValidCard:                        "ValidCard",
	PKValidCardToTarget:                "ValidCardToTarget",
	PKValidCards:                       "ValidCards",
	PKValidCards2:                      "ValidCards2",
	PKValidCause:                       "ValidCause",
	PKValidCauser:                      "ValidCauser",
	PKValidChoices:                     "ValidChoices",
	PKValidCounterType:                 "ValidCounterType",
	PKValidCreature:                    "ValidCreature",
	PKValidCrew:                        "ValidCrew",
	PKValidCrewAmount:                  "ValidCrewAmount",
	PKValidDefender:                    "ValidDefender",
	PKValidDefenders:                   "ValidDefenders",
	PKValidDefined:                     "ValidDefined",
	PKValidDescription:                 "ValidDescription",
	PKValidDestination:                 "ValidDestination",
	PKValidEnlisted:                    "ValidEnlisted",
	PKValidEntity:                      "ValidEntity",
	PKValidExplored:                    "ValidExplored",
	PKValidExplorer:                    "ValidExplorer",
	PKValidKeyword:                     "ValidKeyword",
	PKValidLKI:                         "ValidLKI",
	PKValidLoseReason:                  "ValidLoseReason",
	PKValidMode:                        "ValidMode",
	PKValidNewController:               "ValidNewController",
	PKValidObject:                      "ValidObject",
	PKValidObjectToSource:              "ValidObjectToSource",
	PKValidOriginalController:          "ValidOriginalController",
	PKValidPlayer:                      "ValidPlayer",
	PKValidPlayers:                     "ValidPlayers",
	PKValidResult:                      "ValidResult",
	PKValidSA:                          "ValidSA",
	PKValidSAonCard:                    "ValidSAonCard",
	PKValidSaddled:                     "ValidSaddled",
	PKValidSides:                       "ValidSides",
	PKValidSource:                      "ValidSource",
	PKValidSpell:                       "ValidSpell",
	PKValidSpellAbility:                "ValidSpellAbility",
	PKValidStepTurnToController:        "ValidStepTurnToController",
	PKValidSupportedCopy:               "ValidSupportedCopy",
	PKValidTarget:                      "ValidTarget",
	PKValidTgts:                        "ValidTgts",
	PKValidTgtsDesc:                    "ValidTgtsDesc",
	PKValidToken:                       "ValidToken",
	PKValidTrigger:                     "ValidTrigger",
	PKValidTurned:                      "ValidTurned",
	PKValidTypes:                       "ValidTypes",
	PKValidZone:                        "ValidZone",
	PKValue:                            "Value",
	PKVarName:                          "VarName",
	PKVarValue:                         "VarValue",
	PKVerb:                             "Verb",
	PKVoteCard:                         "VoteCard",
	PKVoteMessage:                      "VoteMessage",
	PKVotePlayer:                       "VotePlayer",
	PKVoteSubAbility:                   "VoteSubAbility",
	PKVoteTiedAbility:                  "VoteTiedAbility",
	PKWard:                             "Ward",
	PKWerewolfTransformCondition:       "WerewolfTransformCondition",
	PKWerewolfUntransformCondition:     "WerewolfUntransformCondition",
	PKWinSubAbility:                    "WinSubAbility",
	PKWithCountersAmount:               "WithCountersAmount",
	PKWithCountersType:                 "WithCountersType",
	PKWithDifferentNames:               "WithDifferentNames",
	PKWithMayLook:                      "WithMayLook",
	PKWithNotedCounters:                "WithNotedCounters",
	PKWithTotalCMC:                     "WithTotalCMC",
	PKWithTotalCardTypes:               "WithTotalCardTypes",
	PKWithoutManaCost:                  "WithoutManaCost",
	PKWon:                              "Won",
	PKWontPhaseInNormal:                "WontPhaseInNormal",
	PKXChoice:                          "XChoice",
	PKXColor:                           "XColor",
	PKXMax:                             "XMax",
	PKXMin:                             "XMin",
	PKZone:                             "Zone",
	PKsVars:                            "sVars",
	PKstaticAbilities:                  "staticAbilities",
}

// String is the key's Forge text.
func (k ParamKey) String() string { return paramKeyNames[k] }

// paramKeyByName is the load-time inverse of paramKeyNames (never read on a
// hot path: newParamSet runs once per node at load).
var paramKeyByName = func() map[string]ParamKey {
	m := make(map[string]ParamKey, paramKeyCount)
	for k := ParamKey(1); k < paramKeyCount; k++ {
		m[paramKeyNames[k]] = k
	}
	return m
}()

// ParamMask is a set of ParamKeys.
type ParamMask [paramMaskWords]uint64

// ParamMaskOf builds a mask of keys (package-init constant tables).
func ParamMaskOf(keys ...ParamKey) ParamMask {
	var m ParamMask
	for _, k := range keys {
		m[k>>6] |= 1 << (k & 63)
	}
	return m
}

// ParamSet is one Params map compiled against the ParamKey vocabulary: the
// presence mask of the vocabulary keys it holds and their values in key
// order. It is immutable once built and answers only for the exact map it
// was built from (src, n): a node copy whose Params was replaced or resized
// falls back to the map read.
//
// A read is a mask test and a rank: rank[w] is the number of keys present in
// the words below w (a per-word popcount prefix computed once at build), so
// a key's value index is rank[w] plus the popcount of its own word below its
// bit, whichever word it lives in. No loop, no map, no allocation: a range test
// (never taken for a declared key) and the bit test.
type ParamSet struct {
	// src is the map the set was built from. Held as the map itself (not
	// its address) so two identically parsed nodes stay reflect.DeepEqual;
	// bound compares identities.
	src  map[string]string
	n    int
	has  ParamMask
	rank [paramMaskWords]uint16
	vals []string
	// codes is each value's ParamCoder code, parallel to vals (nil when no
	// key the set holds has a coder).
	codes []uint16
}

func mapIdentity(m map[string]string) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&m))
}

func newParamSet(m map[string]string) *ParamSet {
	if m == nil {
		return nil
	}
	ps := &ParamSet{src: m, n: len(m)}
	for key := range m {
		if k, ok := paramKeyByName[key]; ok {
			ps.has[k>>6] |= 1 << (k & 63)
		}
	}
	n := 0
	for w, h := range ps.has {
		ps.rank[w] = uint16(n) // n < paramKeyCap: a uint16 holds any count
		n += bits.OnesCount64(h)
	}
	ps.vals = make([]string, 0, n)
	for w, h := range ps.has {
		for ; h != 0; h &= h - 1 {
			k := ParamKey(w<<6 | bits.TrailingZeros64(h))
			ps.vals = append(ps.vals, m[paramKeyNames[k]])
		}
	}
	paramCodersSealed.Store(true)
	for w, h := range ps.has {
		for i := int(ps.rank[w]); h != 0; h &= h - 1 {
			k := ParamKey(w<<6 | bits.TrailingZeros64(h))
			if c := paramCoders[k]; c != nil {
				if ps.codes == nil {
					ps.codes = make([]uint16, n)
				}
				ps.codes[i] = c(ps.vals[i])
			}
			i++
		}
	}
	return ps
}

// ParamCoder classifies one parameter value into a downstream package's
// dense code (a zone word, a condition keyword, ...). It must be a pure
// function of the text.
type ParamCoder func(value string) uint16

// paramCoders is the registered coder per key and paramCoderOwners the
// registering vocabulary's name; paramCodersSealed is set by the first
// ParamSet built, after which a registration would leave earlier sets
// without their codes.
var (
	paramCoders       [paramKeyCount]ParamCoder
	paramCoderOwners  [paramKeyCount]string
	paramCodersSealed atomic.Bool
)

// RegisterParamCoder makes every ParamSet built from now on store key k's
// value classified by f, so a hot read takes the stored code (ParamCode)
// instead of re-classifying the text on every call. owner names the
// vocabulary ("effects.ZoneList"); a second registration for k is allowed
// only under the same owner -- a test binary can link two copies of one
// package, each running its init. Package init only: it panics once a set
// has been built.
func RegisterParamCoder(k ParamKey, owner string, f ParamCoder) {
	if paramCodersSealed.Load() {
		panic("cards: RegisterParamCoder after a ParamSet was built: " + paramKeyNames[k])
	}
	if paramCoders[k] != nil && paramCoderOwners[k] != owner {
		panic("cards: ParamCoder for " + paramKeyNames[k] + " registered by " + paramCoderOwners[k] + " and " + owner)
	}
	paramCoders[k], paramCoderOwners[k] = f, owner
}

func (ps *ParamSet) index(k ParamKey) (int, bool) {
	w := int(k) >> 6
	if w >= paramMaskWords {
		return 0, false
	}
	h, b := ps.has[w], uint64(1)<<(k&63)
	if h&b == 0 {
		return 0, false
	}
	return int(ps.rank[w]) + bits.OnesCount64(h&(b-1)), true
}

// paramCode is key k's value code: the code stored at load when ps is bound
// to m, else the registered coder applied to m's value. ok reports the key's
// presence; a key with no coder answers 0.
func paramCode(ps *ParamSet, m map[string]string, k ParamKey) (uint16, bool) {
	if ps.bound(m) {
		i, ok := ps.index(k)
		if !ok {
			return 0, false
		}
		if ps.codes != nil {
			return ps.codes[i], true
		}
		if c := paramCoders[k]; c != nil {
			return c(ps.vals[i]), true
		}
		return 0, true
	}
	v, ok := m[paramKeyNames[k]]
	if !ok {
		return 0, false
	}
	if c := paramCoders[k]; c != nil {
		return c(v), true
	}
	return 0, true
}

// SameParamMap reports whether a and b are the same Params map instance: the
// identity rule a compiled view of a node's parameters -- a ParamSet here, a
// downstream typed parameter struct elsewhere -- uses to answer only for the
// exact map it was compiled from (with the map's size at compile time, which
// the caller keeps, catching an in-place write).
func SameParamMap(a, b map[string]string) bool {
	return mapIdentity(a) == mapIdentity(b)
}

// ParamNames returns the ability's parameter keys, sorted (a fresh slice;
// load-time compilers only -- a downstream unread-parameter check walks it
// instead of ranging the map).
func (sa *SA) ParamNames() []string {
	out := make([]string, 0, len(sa.Params))
	for k := range sa.Params {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// ParamMapIdentity is m's identity as an address, for a downstream cache
// keyed by the map instance (compare entries with SameParamMap).
func ParamMapIdentity(m map[string]string) uintptr {
	return uintptr(mapIdentity(m))
}

func (ps *ParamSet) bound(m map[string]string) bool {
	return ps != nil && ps.n == len(m) && mapIdentity(ps.src) == mapIdentity(m)
}

func (ps *ParamSet) get(k ParamKey) (string, bool) {
	w := int(k) >> 6
	if w >= paramMaskWords {
		// Outside the vocabulary: absent, as an unset bit would be. The
		// test also lets the compiler drop the has/rank bounds checks.
		return "", false
	}
	h, b := ps.has[w], uint64(1)<<(k&63)
	if h&b == 0 {
		return "", false
	}
	return ps.vals[int(ps.rank[w])+bits.OnesCount64(h&(b-1))], true
}

func paramGet(ps *ParamSet, m map[string]string, k ParamKey) (string, bool) {
	if ps.bound(m) {
		return ps.get(k)
	}
	v, ok := m[paramKeyNames[k]]
	return v, ok
}

// paramMayHaveAny reports whether m may hold any key of mask: exact when ps
// is bound to m, else conservatively true.
func paramMayHaveAny(ps *ParamSet, m map[string]string, mask ParamMask) bool {
	if ps.bound(m) {
		var x uint64
		for w := range mask {
			x |= ps.has[w] & mask[w]
		}
		return x != 0
	}
	return true
}

// Param is Params[k] with presence, through the compiled set when bound.
func (s Static) Param(k ParamKey) (string, bool) { return paramGet(s.ps, s.Params, k) }

// ParamStr is Params[k] ("" when absent).
func (s Static) ParamStr(k ParamKey) string { v, _ := paramGet(s.ps, s.Params, k); return v }

// ParamCode is key k's value as its registered ParamCoder's code, stored at
// load (RegisterParamCoder); ok reports the key's presence.
func (s Static) ParamCode(k ParamKey) (uint16, bool) { return paramCode(s.ps, s.Params, k) }

// HasParam reports whether key k is present.
func (s Static) HasParam(k ParamKey) bool { _, ok := paramGet(s.ps, s.Params, k); return ok }

// CharacteristicDefining reports whether the static carries Forge's
// CharacteristicDefining$ True marker (CR 604.3). The param-value spelling
// lives here, beside the key that owns it, so rules' CDA predicates read the
// flag through one home.
func (s Static) CharacteristicDefining() bool {
	return s.ParamStr(PKCharacteristicDefining) == "True"
}

// MayHaveAnyParam is false only when the static provably holds no key of mask.
func (s Static) MayHaveAnyParam(mask ParamMask) bool { return paramMayHaveAny(s.ps, s.Params, mask) }

// ParamSetOf is the static's compiled parameter set, for a view that carries
// the Params map onward (nil when unbound).
func (s Static) ParamSetOf() *ParamSet { return s.ps }

// Param is Params[k] with presence, through the compiled set when bound.
func (t Trigger) Param(k ParamKey) (string, bool) { return paramGet(t.ps, t.Params, k) }

// ParamStr is Params[k] ("" when absent).
func (t Trigger) ParamStr(k ParamKey) string { v, _ := paramGet(t.ps, t.Params, k); return v }

// ParamCode is Static.ParamCode for a trigger line.
func (t Trigger) ParamCode(k ParamKey) (uint16, bool) { return paramCode(t.ps, t.Params, k) }

// HasParam reports whether key k is present.
func (t Trigger) HasParam(k ParamKey) bool { _, ok := paramGet(t.ps, t.Params, k); return ok }

// Param is Params[k] with presence, through the compiled set when bound.
func (r Repl) Param(k ParamKey) (string, bool) { return paramGet(r.ps, r.Params, k) }

// ParamStr is Params[k] ("" when absent).
func (r Repl) ParamStr(k ParamKey) string { v, _ := paramGet(r.ps, r.Params, k); return v }

// ParamCode is Static.ParamCode for a replacement line.
func (r Repl) ParamCode(k ParamKey) (uint16, bool) { return paramCode(r.ps, r.Params, k) }

// HasParam reports whether key k is present.
func (r Repl) HasParam(k ParamKey) bool { _, ok := paramGet(r.ps, r.Params, k); return ok }

// OptionalValue reports the compiled Optional$ True replacement election flag.
// Parsed faces bind it once; an unbound synthetic Repl reads its raw parameter
// exactly, matching the script's canonical spelling.
func (r Repl) OptionalValue() bool {
	if r.optionalBound {
		return r.optional
	}
	v, ok := r.Param(PKOptional)
	return ok && v == "True"
}

// ParamSetParam reads key k of m through ps (a view carrying a node's Params
// map and its ParamSet side by side).
func ParamSetParam(ps *ParamSet, m map[string]string, k ParamKey) (string, bool) {
	return paramGet(ps, m, k)
}

// ParamSetMayHaveAny is MayHaveAnyParam for a view's (ps, map) pair.
// ParamSetCode is ParamCode through a caller-held set (a view that keeps a
// node's ParamSet and Params apart).
func ParamSetCode(ps *ParamSet, m map[string]string, k ParamKey) (uint16, bool) {
	return paramCode(ps, m, k)
}

func ParamSetMayHaveAny(ps *ParamSet, m map[string]string, mask ParamMask) bool {
	return paramMayHaveAny(ps, m, mask)
}

// Param is Params[k] with presence, through the compiled set when bound.
func (sa *SA) Param(k ParamKey) (string, bool) { return paramGet(sa.ps, sa.Params, k) }

// ParamStr is Params[k] ("" when absent).
func (sa *SA) ParamStr(k ParamKey) string { v, _ := paramGet(sa.ps, sa.Params, k); return v }

// ParamCode is Static.ParamCode for an ability.
func (sa *SA) ParamCode(k ParamKey) (uint16, bool) { return paramCode(sa.ps, sa.Params, k) }

// HasParam reports whether key k is present.
func (sa *SA) HasParam(k ParamKey) bool { _, ok := paramGet(sa.ps, sa.Params, k); return ok }

// MayHaveAnyParam is false only when the ability provably holds no key of
// mask.
func (sa *SA) MayHaveAnyParam(mask ParamMask) bool { return paramMayHaveAny(sa.ps, sa.Params, mask) }

// HasAnyParam reports whether the ability holds a key of mask, exactly: an
// unbound ability walks the mask's keys.
func (sa *SA) HasAnyParam(mask ParamMask) bool {
	if sa.ps.bound(sa.Params) {
		return paramMayHaveAny(sa.ps, sa.Params, mask)
	}
	for w, keys := range mask {
		for ; keys != 0; keys &= keys - 1 {
			if _, ok := sa.Params[paramKeyNames[w<<6|bits.TrailingZeros64(keys)]]; ok {
				return true
			}
		}
	}
	return false
}

// ExhaustValue reports the compiled Exhaust$ True activation qualifier.
// Parsed abilities bind it once; synthetic abilities read their parameter.
func (sa *SA) ExhaustValue() bool {
	if sa.exhaustBound {
		return sa.exhaust
	}
	v, ok := sa.Param(PKExhaust)
	return ok && strings.EqualFold(strings.TrimSpace(v), "True")
}

// SetParam writes key k into the ability's own Params map (allocating it
// when nil). The compiled set no longer describes the map, so reads go to
// the map from here on. Write only to a node the caller owns -- a copy or a
// node it built -- never to a shared parsed node.
func (sa *SA) SetParam(k ParamKey, v string) {
	if sa.Params == nil {
		sa.Params = map[string]string{}
	}
	sa.Params[paramKeyNames[k]] = v
	sa.ps = nil
	if k == PKExhaust {
		// The compiled Exhaust$ fact would otherwise shadow the write.
		sa.exhaustBound = false
	}
}

// SetParam is SA.SetParam for a replacement line.
func (r *Repl) SetParam(k ParamKey, v string) {
	if r.Params == nil {
		r.Params = map[string]string{}
	}
	r.Params[paramKeyNames[k]] = v
	r.ps = nil
}

// deriveParamSets binds each printed static's, trigger's and ability's
// ParamSet (every ability reachable from the face: its Abilities, their
// SubAbility$ chains, trigger Execute$ bodies and replacement bodies). It
// runs at load (derive, and again at the end of link, which re-resolves
// bodies); a node built later stays unbound and reads its map.
func (f *Face) deriveParamSets() {
	for i := range f.Statics {
		st := &f.Statics[i]
		st.ps = newParamSet(st.Params)
		st.mode, st.modeBound = StaticModeOf(st.Mode), true
	}
	for i := range f.Triggers {
		t := &f.Triggers[i]
		t.ps = newParamSet(t.Params)
		t.mode, t.modeBound = TriggerModeOf(t.Mode), true
		t.bindOffBattlefieldTarget()
		t.bindTrailingIf()
	}
	for i := range f.Repls {
		r := &f.Repls[i]
		r.ps = newParamSet(r.Params)
		r.event, r.eventBound = ReplEventOf(r.Event), true
		r.optional = strings.EqualFold(strings.TrimSpace(r.Params[paramKeyNames[PKOptional]]), "True")
		r.optionalBound = true
	}
	bindSA := func(sa *SA) {
		for d := 0; sa != nil && d <= maxSVarDepth+1; d++ {
			sa.ps = newParamSet(sa.Params)
			sa.api, sa.apiBound = APICodeForName(sa.API), true
			sa.exhaust = strings.EqualFold(strings.TrimSpace(sa.Params[paramKeyNames[PKExhaust]]), "True")
			sa.exhaustBound = true
			if sa.extSlot == nil {
				sa.extSlot = &ExtSlot{}
			}
			if sa.costSlot == nil {
				sa.costSlot = &Slot{}
			}
			sa = sa.Sub
		}
	}
	for _, a := range f.Abilities {
		bindSA(a)
	}
	for i := range f.Triggers {
		bindSA(f.Triggers[i].Effect)
	}
	for i := range f.Repls {
		bindSA(f.Repls[i].With)
	}
}
