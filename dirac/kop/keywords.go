package kop

// KwMask is a set of keyword abilities. Bits are append-only.
type KwMask uint32

// KwID names one keyword. 0..31 are KwMask's bit positions, with no
// constants of their own; the manifest's keywords (dirac/manifest/KwID,
// generated into manifest_gen.go) take 32 and up. KwMask is full, so a new
// keyword is a KwID, granted through the arena rather than a mask bit.
type KwID uint16

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
	StackKwRebound     int16 = 11 // a granted rebound (CR 702.88a), read as the spell resolves: the resolved hand-cast spell is exiled and its upkeep recast runs the carrier's GrantBody latent (dirac kw_rebound.go)
	StackKwWebSling    int16 = 12 // a granted web-slinging (CR 702.188a; Static.GrantCost is its cost, Filter2 the grant's own spell spec): the hand card is offered a web-slinging cast beside the plain one (dirac kw_websling.go webSlingGrant)
	StackKwOffspring   int16 = 14 // a granted offspring (CR 702.175a; Static.GrantCost is its cost): the kicked cast charges it and the ETB copy trigger runs the carrier's GrantBody latent (dirac kw_offspring.go)
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
