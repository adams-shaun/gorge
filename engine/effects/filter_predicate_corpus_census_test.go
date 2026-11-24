package effects

import (
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// filterPredicateSpecKeys are the Forge parameters whose value is a card
// filter spec read through the filter grammar (matchesObjectText). A key here
// is a spec the matcher splits at ',' and '+'; an unknown word in one fails
// that alternative closed, which turns a gate or intervening-if into a silent
// no-op (Rasputin Dreamweaver's Card.Self+startedTheTurnUntapped).
var filterPredicateSpecKeys = map[string]bool{
	"IsPresent": true, "IsPresent2": true,
	"ConditionPresent": true, "ConditionPresent2": true,
	"ValidCard": true, "ValidCards": true, "ValidTgts": true,
	"ValidSource": true, "ValidTarget": true, "ValidDamageSource": true,
	"ValidCause": true, "ValidActivatingPlayer": false,
	"Affected": true, "ChangeType": true, "Valid": true,
}

// filterPredicateCensus walks every compiled corpus card's abilities,
// triggers, statics, replacements and SVars and returns, for each predicate
// word UnknownPredicates rejects, the sorted card names that carry it.
func filterPredicateCensus(t *testing.T) (carriers map[string]map[string]bool, scanned int) {
	reg := testutil.CorpusRegistry(t)
	carriers = map[string]map[string]bool{}
	visitParams := func(card string, params map[string]string) {
		names := make([]string, 0, len(params))
		for k := range params {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, k := range names {
			if !filterPredicateSpecKeys[k] {
				continue
			}
			scanned++
			for _, w := range UnknownPredicates(strings.TrimSpace(params[k])) {
				if carriers[w] == nil {
					carriers[w] = map[string]bool{}
				}
				carriers[w][card] = true
			}
		}
	}
	var visitSA func(card string, sa *cards.SA)
	visitSA = func(card string, sa *cards.SA) {
		for seen := 0; sa != nil && seen < 64; sa, seen = sa.Sub, seen+1 {
			visitParams(card, sa.Params)
		}
	}
	for _, card := range reg.AllCards() {
		for _, f := range card.Faces {
			for _, sa := range f.Abilities {
				visitSA(f.Name, sa)
			}
			for _, tr := range f.Triggers {
				visitParams(f.Name, tr.Params)
				visitSA(f.Name, tr.Effect)
			}
			for _, st := range f.Statics {
				visitParams(f.Name, st.Params)
			}
			for _, r := range f.Repls {
				visitParams(f.Name, r.Params)
				visitSA(f.Name, r.With)
			}
			names := make([]string, 0, len(f.SVars))
			for name := range f.SVars {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				visitSA(f.Name, cards.ResolveSVar(f.SVars, name))
			}
		}
	}
	return carriers, scanned
}

// TestFilterPredicateCorpusCensusOnlyShrinks ratchets, both ways, the filter
// predicate words in corpus filter specs that UnknownPredicates rejects.
// Classification is UnknownPredicates itself (the matcher's own classifier
// and splitter), so the census cannot drift from the matcher. A new unknown
// word fails with an example carrier; a table word the build now
// recognises fails as stale. Implementing a word means deleting its row.
func TestFilterPredicateCorpusCensusOnlyShrinks(t *testing.T) {
	t.Parallel()
	carriers, scanned := filterPredicateCensus(t)
	if scanned < 1000 {
		t.Fatalf("census read %d filter specs: the scan is not reading the corpus", scanned)
	}
	words := make([]string, 0, len(carriers))
	for w := range carriers {
		words = append(words, w)
	}
	sort.Strings(words)
	want := map[string]bool{}
	for _, w := range knownUnknownFilterPredicates {
		want[w] = true
	}
	if os.Getenv("FILTER_PREDICATE_CENSUS_DUMP") != "" {
		t.Logf("measured set:\n%s", strings.Join(words, "\n"))
	}
	for _, w := range words {
		if want[w] {
			continue
		}
		names := make([]string, 0, len(carriers[w]))
		for n := range carriers[w] {
			names = append(names, n)
		}
		sort.Strings(names)
		if len(names) > 3 {
			names = append(names[:3], "...")
		}
		t.Errorf("new unknown filter predicate %q (e.g. %s): it never matches, so a gate carrying it is a silent no-op; implement it, or triage it into knownUnknownFilterPredicates", w, strings.Join(names, "; "))
	}
	for w := range want {
		if carriers[w] == nil {
			t.Errorf("knownUnknownFilterPredicates word %q is stale: no corpus filter spec carries it as unknown any more; delete the row", w)
		}
	}
}

// knownUnknownFilterPredicates is the ratchet table: the predicate words
// UnknownPredicates rejects in some corpus filter spec today. It only
// shrinks: implement a word, delete its row.
var knownUnknownFilterPredicates = []string{
	"!AllColors",
	"!CardOwner",
	"!CastSaSource",
	"!ManaAbility",
	"!SharesColorWith Valid Creature.YouCtrl",
	"!TriggeredActivator",
	"!TriggeredCardController",
	"!dealtCombatDamageThisTurn Creature",
	"!numColorsEQ2",
	"!sharesCreatureTypeWith Valid Creature.YouCtrl",
	"!sharesCreatureTypeWith ValidGraveyard Creature.YouOwn",
	"!sharesCreatureTypeWith ValidLibrary Creature.YouOwn",
	"!sharesLandTypeWith Valid Land.YouCtrl",
	"Above",
	"Active",
	"AllColors",
	"AnyChosenColor",
	"Backup",
	"BorderColorBlack",
	"BorderColorSilver",
	"BorderColorWhite",
	"BottomGraveyard",
	"BottomLibrary_Creature",
	"CanEnchantRemembered",
	"CanEnchantSource",
	"CanPayManaCost",
	"CastSa Spell.OppCtrl",
	"Chosen",
	"ChosenSector",
	"ControlledBy ChoosingPlayer",
	"ControlledBy DelayTriggerRemembered",
	"ControlledBy Opponent.IsCorrupted",
	"ControlledBy Opponent.castSpellThisTurn",
	"ControlledBy Player.targetedBy",
	"ControlledBy Player.wasDealtCombatDamageThisCombatBy ParentTarget",
	"ControlledBy Player.wasDealtCombatDamageThisTurnBy Creature.Pirate GE3",
	"ControlledBy TriggeredTargetController",
	"ControlledBy You.castSpellThisTurn",
	"ControllerControlsAtLeastAsManyLand",
	"ControllerControlsIsland",
	"ControllerControlsLand_LE3",
	"ControllerControlsMoreLand",
	"CostsHybridMana",
	"CostsPhyrexianMana",
	"Cycling",
	"Damaged",
	"DamagedBy Equipped",
	"DamagedByGiant.YouCtrl;Wizard.YouCtrl;Spell.YouCtrl",
	"DefendingPlayerCtrl",
	"DirectlyAbove",
	"DiscardedThisTurn",
	"EnchantedPlayerCtr",
	"EnteredSinceYourLastTurn",
	"EnteredUnder You",
	"EquippedByTargeted",
	"EquippedByValid Equipment.Legendary",
	"ExiledWith",
	"FoughtThisTurn",
	"HasDevoured",
	"Imprinted",
	"IsUnearthed",
	"LostLifeThisTurn",
	"ManaCost0",
	"ManaCost1",
	"ManaCost=Equipped",
	"ManaCost=Imprinted",
	"ManaSpent EQ0",
	"Modular",
	"MostProminentColor black",
	"MostProminentColor blue",
	"MostProminentColor green",
	"MostProminentColor red",
	"MostProminentColor white",
	"NameNotEnchantingEnchantedPlayer",
	"NotDefinedParentTarget",
	"NotDefinedReplacedSimultaneousETB",
	"NotedColor",
	"NotedGuessPhantasm",
	"NotedNameAetherSearcher",
	"NotedNameNobleBanneret",
	"NotedNameSmugglerCaptain",
	"NotedType",
	"NotedTypes",
	"OpponentToActive",
	"Other.YouOwn",
	"OwnedBy Opponent",
	"OwnedBy Opponent.NonActive",
	"OwnerDoesntControl",
	"Party",
	"ProtectedBy TriggeredTarget",
	"Sacrificed",
	"SecondSpellCastThisTurn",
	"SharesCMCWith Discarded",
	"SharesCMCWith Imprinted",
	"SharesCMCWith Imprinted.ExiledWithSource",
	"SharesCMCWith TriggeredCard",
	"SharesColorWith",
	"SharesColorWith Equipped",
	"SharesColorWith Imprinted.ExiledWithSource",
	"SharesColorWith LastCastThisTurn",
	"SharesColorWith TopOfLibrary",
	"ThisTurnCast",
	"ThisTurnEnteredYourHand",
	"TopGraveyard",
	"TopGraveyard2",
	"TopGraveyardCreature",
	"Transformed",
	"TriggeredCrew",
	"TriggeredDefendingPlayer",
	"Untapped",
	"ValidGraveyard Card.YouOwn",
	"ValidGraveyard Creature.Legendary",
	"activated",
	"attackedBattleThisTurn",
	"attackedLastTurn",
	"attackedThisCombat GE3",
	"attackedWithCreaturesThisTurn",
	"attackedYouTheirLastTurn",
	"attackedYouThisTurn",
	"attackersBandedWith",
	"attacking ChosenPlayer",
	"attacking EnchantedPlayer",
	"attacking TriggeredAttackedTarget",
	"attacking TriggeredDefender",
	"attacking Valid Planeswalker.OppCtrl",
	"attackingYouOrYourPW",
	"baseLoyaltyEQX",
	"blockedOrBeenBlockedSinceYourLastUpkeep",
	"blockedThisCombat",
	"blockingCreatureYouCtrl",
	"blockingEnchanted",
	"blockingEquipped",
	"blockingRemembered",
	"blockingSelf",
	"blockingValid Creature.EquippedBy",
	"blockingValid Goblin",
	"blockingValid Orc",
	"canProduceManaColor Colorless",
	"canProduceSameManaTypeWith",
	"controlsLand.counters\\_GE1\\_CONTESTED",
	"couldAttackButNotAttacking",
	"countersReceivedThisTurn_GE1_P1P1_You",
	"dealtCombatDamageThisCombat",
	"dealtCombatDamageThisTurn Player",
	"dealtDamageToOppThisTurn",
	"dealtDamageToYouThisTurn",
	"doesNotShareNameWith Remembered",
	"equipping",
	"exploited",
	"greatestRememberedCMC",
	"hasAbility Activated.otherAbility",
	"hasFewerCreaturesInYardThanActive",
	"hasMoreCardsInHandThanActive",
	"hasMoreLifeThanActive",
	"hasNonManaActivatedAbility",
	"isBlockedByRemembered",
	"isDamaged",
	"leastPower",
	"lowestRememberedCMC",
	"madness",
	"milledThisTurn",
	"numColorsEQ2",
	"numColorsEQ3",
	"numColorsEQX",
	"setARN",
	"setATQ",
	"setHML",
	"setWAR",
	"sharesBlockingAssignmentWith",
	"sharesCardTypeWith",
	"sharesCardTypeWith EachTopLibrary",
	"sharesCardTypeWith Exiled",
	"sharesCardTypeWith Imprinted.ExiledWithSource",
	"sharesCardTypeWith ReplacedSource",
	"sharesCardTypeWith ValidExile Card.ExiledWithSource",
	"sharesControllerWith",
	"sharesCreatureTypeWith Equipped",
	"sharesCreatureTypeWith Sacrificed",
	"sharesCreatureTypeWith Valid Creature.YouCtrl",
	"sharesCreatureTypeWithAll Tapped",
	"sharesNameWith",
	"sharesNameWith Battlefield",
	"sharesNameWith ChosenCard",
	"sharesNameWith ExiledWith",
	"sharesNameWith Imprinted",
	"sharesNameWith NonToken",
	"sharesNameWith Remembered.ExiledWithSource",
	"sharesNameWith Sacrificed",
	"sharesNameWith ThisTurnCast",
	"sharesNameWith ValidExile Card.ExiledWithSource",
	"sharesNameWith YourGraveyard",
	"sharesOwnerWith TriggeredCard",
	"sharesPermanentTypeWith",
	"spectacle",
	"stickered",
	"stickeredWith Art",
	"stickeredWith Name",
	"stickeredWith PT",
	"surveilledThisTurn",
	"turnedFaceUpThisTurn",
	"wasCastFromCommand",
	"wasCastFromGraveyardByYou",
	"wasDealtCombatDamageThisTurnBySource",
	"wasDealtCombatDamageThisTurnBySourceTimes EQ1",
	"wasDealtDamageThisTurnBySource",
	"wasDealtDamageThisTurnBySource GE10",
	"webSlinged",
	"withAugment",
	"withAwaken",
	"withEmbalm",
	"withEternalize",
	"withFading",
	"withFreerunning",
	"withLevel up",
	"withLowestLife",
	"withModular",
	"withMoreCreaturesThanActive",
	"withMoreLandsThanActive",
	"withMoreLandsThanYou",
	"withMostPermanentInPlay",
	"withPartner",
	"withProtection",
	"withTypeCycling",
	"withoutAt the beginning of your upkeep",
	"withoutCumulative upkeep",
	"withoutRampage",
}
