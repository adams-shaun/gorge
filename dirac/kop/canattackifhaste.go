package kop

// StatCanAttackIfHaste is a printed S:Mode$ CanAttackIfHaste (Frenzied
// Saddlebrute, Instill Energy, Chaos Lord): a summoning-sick creature
// matching Filter (nil: every creature, gorge's ValidCard$ default "Card") may
// be declared as an attacker as though it had haste (CR 302.6; the exemption
// is for attacking only). With StHasteOppDef (ValidTarget$ Opponent,
// Planeswalker.OppCtrl) the exemption holds only against a defender who is an
// opponent of the static's controller: the player, a planeswalker or a battle
// that player controls, the attacked player being the defender in every pair
// (gorge combat.CanAttackIfHastePair). The creature is still offered as an
// attacker when some pair is exempt; the kernel's half is
// dirac/static_canattackifhaste.go.
const StatCanAttackIfHaste StaticKind = 257

// StHasteOppDef (StatCanAttackIfHaste only): the defender must be an
// opponent of the static's controller.
const StHasteOppDef StaticFlags = 1 << 0
