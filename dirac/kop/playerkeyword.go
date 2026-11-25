package kop

// StatPlayerKeyword's Static.A: the player keyword the static grants its
// controller (`Affected$ You | AddKeyword$ ...`). 0 is no keyword; the two
// below are the grants gorge reads (engine/rules/mana.go PayLifeInsteadOfB,
// engine/rules/playerkeywords.go defenderAssignsAttackerDamage).
const (
	// PlKwPayLifeInsteadOfB: "For each {B} in a cost, you may pay 2 life
	// rather than pay that mana" (K'rrik, Son of Yawgmoth).
	PlKwPayLifeInsteadOfB = 1
	// PlKwDefenderAssigns: "You assign combat damage of each creature
	// attacking you" (Defensive Formation, CR 510.1c as the card rewrites it).
	PlKwDefenderAssigns = 2
)
