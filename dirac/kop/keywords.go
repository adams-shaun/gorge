package kop

// KwMask is a set of keyword abilities. Bits are append-only.
type KwMask uint32

const (
	KwFlying KwMask = 1 << iota
	KwReach
	KwFirstStrike
	KwDoubleStrike
	KwDeathtouch
	KwLifelink
	KwTrample
	KwVigilance
	KwHaste
	KwMenace
	KwDefender
	KwFlash
	KwHexproof
	KwIndestructible
	KwProwess
	KwProtectionAll // protection from everything
	KwShroud
	KwWard        // ward {N}: CardDef.WardN
	KwUnblockable // can't be blocked
	KwCantBlock   // this creature can't block
	KwWither      // wither (CR 702.80) when GRANTED (Pump KW$, static AddKeyword$, Animate); printed wither stays CardDef.Wither. Was the reserved KwShadowNone, never set
	KwKicker      // the card has a kicker cost (CardDef.Kicker)
	KwFlashback   // the card has a flashback cost (CardDef.AltCost)
	KwAffinity    // affinity for artifacts
)

// CardDef.Evasion bits.
const (
	EvHorsemanship uint8 = 1 << iota // CR 702.31b: blocked only by creatures with horsemanship
	EvIntimidate                     // CR 702.13a: blocked only by artifact creatures and/or creatures sharing a colour
	EvSkulk                          // CR 702.118a: not blocked by creatures with greater power
)

// The StatStackKw keyword codes (Static.A).
const (
	StackKwImprovise   int16 = 1
	StackKwCasualty    int16 = 2
	StackKwCascade     int16 = 3
	StackKwLifelink    int16 = 4
	StackKwStorm       int16 = 5
	StackKwSplitSecond int16 = 6  // read off each spell on the stack: while one holds it, no spell is cast and no non-mana ability is activated (CR 702.61a)
	StackKwConvoke     int16 = 7  // read wherever the printed convoke is (tapPays): the cast announces tapped creatures toward its mana cost (CR 702.51a)
	StackKwConspire    int16 = 8  // a granted conspire (CR 702.78a): the conspired cast taps two creatures sharing a colour with the spell and copies it (dirac kw_conspire.go)
	StackKwDemonstrate int16 = 10 // a granted demonstrate (CR 702.144a), read as the spell is cast like Storm: the cast trigger offers the copy and the opponent's (dirac kw_demonstrate.go)
	StackKwDelve       int16 = 9  // read as the spell is cast: the cast announces graveyard cards to exile toward its generic cost (CR 702.66a; kernel kw_delve.go)
)

// Appended keyword bits.
const (
	KwHexproofFromInstants    KwMask = KwAffinity << (1 + iota) // hexproof from instants
	KwProtBlack                                                 // protection from black
	KwProtRed                                                   // protection from red
	KwPreventCombatDamageTo                                     // prevent all combat damage that would be dealt to it
	KwFlashbackOwnCost                                          // flashback with its mana cost (a graveyard card)
	KwPreventCombatDamageFrom                                   // prevent all combat damage that would be dealt by it (an Effect's Prevent$ ValidSource$ IsRemembered)
	KwFear                                                      // fear (CR 702.36): blocked only by artifact and/or black creatures
	KwShadow                                                    // shadow (CR 702.28): blocks and is blocked only by shadow creatures
)
