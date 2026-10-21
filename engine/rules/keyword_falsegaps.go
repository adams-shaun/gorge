package rules

import "github.com/adams-shaun/gorge/engine/effects"

// Registration false gaps (CR audit S08a#15, S08b D14): each keyword below
// is implemented and has a behaviour test, but its kw: head was never
// registered, so the coverage walk counted every printed carrier as
// unplayable. Registering the head is the whole fix; the behaviour lives at
// the named site and is proved by the named test (pinned in
// TestRegisteredKeywordsAreHonoured).
func init() {
	effects.RegisterNonAPI(
		// CR 702.112 Renown: engine/cards/kw_renown.go's combat-damage trigger,
		// effects' PutCounter Renown$ marker (TestConstableOfTheRealmRenownPutsTwoCountersOnce).
		"kw:Renown",
		// CR 702.18 Shroud: Engine.shroudBlocksTarget / playerShroudBlocksTarget
		// read printed and granted heads (TestPrintedShroudWithholdsCastWhenSoleTarget).
		"kw:Shroud",
		// CR 702.26a Phasing: the untap-step toggle in Engine.finishUntapStep
		// (TestCR702PhasingKeywordTogglesAtUntapStep).
		"kw:Phasing",
		// CR 702.63 Vanishing: engine/cards/kw_vanishing.go's entry counters and
		// upkeep/last-counter triggers (TestVanishingDeepForestHermitUpkeepAndLastCounter).
		"kw:Vanishing",
		// CR 702.88 Rebound: the hand-cast exile in Engine.payCast and
		// moveResolvedOffStack's delayed recast (TestReboundCorpusCarrierExilesAndOffers).
		"kw:Rebound",
		// CR 702.150 Compleated: Phyrexian life paid lowers the entry loyalty
		// (TestCompleatedLoyaltyFollowsPhyrexianLifePaid).
		"kw:Compleated",
		// CR 702.153 Casualty: the optional cast-time sacrifice and copy
		// (Engine.casualtySpec; TestCasualtyPrintedSpellSacrificeCopiesTarget).
		"kw:Casualty",
	)
}
