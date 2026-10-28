package cards

import "strings"

// manaChainDepth bounds the SubAbility$ walk IsManaAbilitySA performs. Real
// chains are four nodes deep (ChooseColor -> Mana -> Animate -> Cleanup); the
// cap exists so a runtime-built cyclic Sub chain cannot hang a classifier
// that runs on every offer walk.
const manaChainDepth = 16

// IsManaAbilityAPI reports whether api is one of the two supported activated
// mana ability APIs (CR 605.1a): AB$ Mana and AB$ ManaReflected. It is a pure
// function of the ability's API word, so it lives beside the IR rather than in
// the engine; rules' activation walks and trigmatch's ValidSA$ Activated.Mana
// gate share it. A caller that classifies an ABILITY -- not an API word --
// wants IsManaAbilitySA, which also recognises a chain whose head is not the
// Mana API itself.
func IsManaAbilityAPI(api string) bool { return api == "Mana" || api == "ManaReflected" }

// IsManaAbilitySA reports whether sa is an activated mana ability (CR 605.1a):
// the head-word APIs above, or Forge's "AB$ ChooseColor | SubAbility$ DBMana"
// chain (Foraging Wickermaw, Nykthos Shrine to Nyx, Nyx Lotus, Rhystic Cave).
// The chain's head carries the activation cost and the colour ask, and the
// DB$ Mana sub-ability produces the mana, so classifying by the head word
// alone left the whole ability on the ordinary stack path. Every
// mana-vs-ordinary split must share this one classifier, or the chain is
// offered twice (wheel and priority "ability"). Kind must be AB: an SP$ Mana
// is a mana ritual spell (Battle Hymn), never an activated ability.
//
// The chain arm takes any activated head, whatever its API, whose
// SubAbility$ chain reaches a DB$ Mana (CR 605.1a puts no order on the
// effects: Sacellum Godspeaker reveals, then adds {G}). The exclusions below
// keep out the chains that are not mana abilities: Red Death, Shipwrecker's
// goad targets a creature, a planeswalker ability is a loyalty ability (CR
// 605.1b; isLoyaltyScripted), and Priest of Forgotten Gods, Selvala and the
// like move a card to or from a library.
//
// An ability that requires a target anywhere in its SubAbility$ chain is not
// a mana ability (CR 605.1a): Radiant Lotus, The Warring Triad and Jetfire
// declare ValidTgts$ on the head, Witch Engine's ChangeControl and the two
// Chandra loyalty abilities' damage declare it on a sub-ability, and all six
// stay on the ordinary stack path (the corpus population is pinned by
// TestManaAbilityTargetGateMovesTheTargetingCarriers). The walk is the one
// isTriggeredManaAbility makes for a triggered mana ability, bounded by
// manaChainDepth. An ability whose cost or effect moves a card to or from a
// library is not a mana ability either (CR 605.1a; movesLibraryCard).
func IsManaAbilitySA(sa *SA) bool {
	if sa == nil || !sa.IsActivated() {
		return false
	}
	if !IsManaAbilityAPI(sa.API) && (ManaChainProduction(sa) == nil || isLoyaltyScripted(sa)) {
		return false
	}
	return !requiresTarget(sa) && !movesLibraryCard(sa)
}

// isLoyaltyScripted reports whether sa carries the Planeswalker$ True marker
// Forge scripts on every loyalty ability (CR 605.1b). Only the chain arm of
// IsManaAbilitySA reads it; a head-word Mana loyalty ability is filtered by
// the callers that already split on isLoyaltyAbility.
func isLoyaltyScripted(sa *SA) bool { return sa.ParamStr(PKPlaneswalker) == "True" }

// movesLibraryCard reports whether sa's cost or effect moves a card to or
// from a library, which disqualifies a mana ability (CR 605.1a: "its cost and
// effect don't move any card to or from a library"; Forge's
// SpellAbility.isManaAbility makes the same cost-and-chain test). The answer
// is a pure function of the script, never of the board (CR 605.1a disregards
// replacement effects), so the compiled mana list may cache it. The cost is
// the head's own Cost$ (a sub-ability carries none); the effect is every node
// of the SubAbility$ chain, bounded by manaChainDepth. Measured at the base
// pin, the carriers are Chromatic Sphere, the five Eggs and Rainbow Dash
// (DB$ Draw), Chandra, Flameshaper (DB$ Dig) and the four Mill<1> cost
// carriers (TestCR605_1aManaAbilityLibraryGateMovesTheLibraryCarriers).
func movesLibraryCard(sa *SA) bool {
	if costMovesLibraryCard(sa.ParamStr(PKCost)) {
		return true
	}
	for s, d := sa, 0; s != nil && d < manaChainDepth; s, d = s.Sub, d+1 {
		if effectMovesLibraryCard(s) {
			return true
		}
	}
	return false
}

// costMovesLibraryCard reports whether a raw Cost$ carries a part that mills
// (Mill<N>), draws (Draw<N/...>), exiles from the top of a library
// (ExileFromTop<N/...>) or puts a card into a library (the four
// PutCardToLibFrom<Zone> heads; Forge's Cost.parseCostPart spelling). The
// cost is split on spaces and each part's head (the text before '<') is
// switched on, so the walk neither allocates nor calls strings.
func costMovesLibraryCard(cost string) bool {
	for cost != "" {
		end := 0
		for end < len(cost) && cost[end] != ' ' {
			end++
		}
		part := cost[:end]
		if end < len(cost) {
			end++
		}
		cost = cost[end:]
		lt := 0
		for lt < len(part) && part[lt] != '<' {
			lt++
		}
		if lt == len(part) {
			continue // a bare token ({T}, a mana symbol) never moves a card
		}
		switch part[:lt] {
		case "Mill", "Draw", "ExileFromTop",
			"PutCardToLibFromHand", "PutCardToLibFromGrave", "PutCardToLibFromSameGrave", "PutCardToLibFromBattlefield":
			return true
		}
	}
	return false
}

// effectMovesLibraryCard reports whether one chain node's effect moves a card
// to or from a library: an API that always does (draw, mill, surveil, dig,
// explore, connive, discover, learn, heist, seek, manifest dread), a zone
// change whose Origin$ or Destination$ lists Library (or All), a Play whose
// ValidZone$ lists Library, and a Manifest/Cloak unless its ChoiceZone$
// names a zone list without the library.
func effectMovesLibraryCard(s *SA) bool {
	switch s.API {
	case "Draw", "Mill", "Surveil", "Dig", "DigMultiple", "DigUntil", "Explore", "Connive",
		"Discover", "Learn", "Heist", "Seek", "ManifestDread":
		return true
	case "Manifest", "Cloak":
		z := s.ParamStr(PKChoiceZone)
		return z == "" || zoneListHasLibrary(z)
	case "ChangeZone", "ChangeZoneAll":
		return zoneListHasLibrary(s.ParamStr(PKOrigin)) || zoneListHasLibrary(s.ParamStr(PKDestination))
	case "Play":
		return zoneListHasLibrary(s.ParamStr(PKValidZone))
	}
	return false
}

// zoneListHasLibrary reports whether a comma-separated zone list names the
// library, either as Library or as All (which expands to every zone). Like
// costMovesLibraryCard it splits by hand: no allocation, no strings call.
func zoneListHasLibrary(list string) bool {
	for list != "" {
		end := 0
		for end < len(list) && list[end] != ',' {
			end++
		}
		z := list[:end]
		for z != "" && z[0] == ' ' {
			z = z[1:]
		}
		for z != "" && z[len(z)-1] == ' ' {
			z = z[:len(z)-1]
		}
		if z == "Library" || z == "All" {
			return true
		}
		if end < len(list) {
			end++
		}
		list = list[end:]
	}
	return false
}

// requiresTarget reports whether sa, or any SubAbility$ below it, declares
// ValidTgts$ (the read effects.TargetsOf(sa).Targeted() makes, applied down
// the chain like rules' isTriggeredManaAbility; CR 605.1a). The walk stops at
// manaChainDepth so a cyclic runtime-built chain cannot hang it.
func requiresTarget(sa *SA) bool {
	for s, d := sa, 0; s != nil && d < manaChainDepth; s, d = s.Sub, d+1 {
		if strings.TrimSpace(s.ParamStr(PKValidTgts)) != "" {
			return true
		}
	}
	return false
}

// ManaChainProduction returns the first DB$ Mana sub-ability reachable through
// sa's SubAbility$ chain, or nil when none is. It is the one chain walk behind
// IsManaAbilitySA and the label/payability readers that must name the
// production the head itself does not carry.
func ManaChainProduction(sa *SA) *SA {
	if sa == nil {
		return nil
	}
	for sub, d := sa.Sub, 0; sub != nil && d < manaChainDepth; sub, d = sub.Sub, d+1 {
		if sub.Kind == "DB" && sub.API == "Mana" {
			return sub
		}
	}
	return nil
}

// manaAbilityListEntry is the Face.ManaAbilities list membership: an AB$ Mana
// head, or a chain that reaches a DB$ Mana. ManaReflected is excluded by the
// list's contract -- Face.ManaReflectedAbilities is its own list, and the
// activation walk has a separate reflected pass, so listing a reflected
// ability here would offer it twice.
func manaAbilityListEntry(sa *SA) bool {
	return sa != nil && sa.API != "ManaReflected" && IsManaAbilitySA(sa)
}
