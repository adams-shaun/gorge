package kop

// StMayPayLife is a StatMayPlayFrom static whose cast pays life equal to the
// card's mana value in place of its mana cost (an Effect-delivered library-top
// grant with MayPlayAltManaCost$ PayLife<ConvertedManaCost>: Gwenom,
// Remorseless; CR 118.9). The mana part is waived exactly as StWithoutManaCost
// waives it; the card's other costs and the cost statics are still owed, and a
// land is played for nothing. StaticFlags is full at 16 bits, so it aliases
// StReduceColor, a ReduceCost-only bit no MayPlayFrom static reads.
const StMayPayLife = StReduceColor
