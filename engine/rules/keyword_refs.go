package rules

import "github.com/adams-shaun/gorge/engine/cards"

// Keyword heads this package asks a face about by name, compiled once
// (cards.NewKeywordRef) so a check is a mask or bitset test, not a string
// switch and keyword-line scan.
var (
	kwRefAftermath   = cards.NewKeywordRef("Aftermath")
	kwRefAscend      = cards.NewKeywordRef("Ascend")
	kwRefCipher      = cards.NewKeywordRef("Cipher")
	kwRefConspire    = cards.NewKeywordRef("Conspire")
	kwRefDemonstrate = cards.NewKeywordRef("Demonstrate")
	kwRefDethrone    = cards.NewKeywordRef("Dethrone")
	kwRefDisguise    = cards.NewKeywordRef("Disguise")
	kwRefExploit     = cards.NewKeywordRef("Exploit")
	kwRefFlash       = cards.NewKeywordRef("Flash")
	kwRefFuse        = cards.NewKeywordRef("Fuse")
	kwRefGift        = cards.NewKeywordRef("Gift")
	kwRefHarmonize   = cards.NewKeywordRef("Harmonize")
	kwRefMegamorph   = cards.NewKeywordRef("Megamorph")
	kwRefMentor      = cards.NewKeywordRef("Mentor")
	kwRefMorph       = cards.NewKeywordRef("Morph")
	kwRefOffspring   = cards.NewKeywordRef("Offspring")
	kwRefParadigm    = cards.NewKeywordRef("Paradigm")
	kwRefRiot        = cards.NewKeywordRef("Riot")
	kwRefStorm       = cards.NewKeywordRef("Storm")
	kwRefSunburst    = cards.NewKeywordRef("Sunburst")
	kwRefTraining    = cards.NewKeywordRef("Training")
	kwRefUnleash     = cards.NewKeywordRef("Unleash")
)
