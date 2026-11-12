package kop

// StatAttackTax is a printed S:Mode$ CantAttackUnless with a generic mana
// Cost$ (Propaganda, Ghostly Prison, Windborn Muse, Baird, Steward of Argive,
// Sphere of Safety; CR 508.1d, 508.1g-i): a creature matching Filter can't
// attack a defender the static's scope admits unless its controller pays A
// (VA under StDynamic, read from the static's source and controller) generic
// mana for it. Several statics on one pair sum. gorge prices it in
// rules/attack_cost.go attackPairCharge and pays it in the declare-attackers
// payment window; the kernel's half is dirac/combat_attacktax.go.
const StatAttackTax StaticKind = 243

// StAtkTaxPlayer and StAtkTaxWalker are a StatAttackTax static's Target$
// scope: an attack on the static's controller (Target$ You) and on a
// planeswalker that player controls (Planeswalker.YouCtrl). Neither set is
// no Target$: every defender is taxed (gorge combat.RestrictionTargetMatches'
// empty spec). StaticFlags is full at 16 bits, so they alias the
// MayPlayFrom-only StOncePerTurn and StAnyManaType: a static's StFlags are
// kind-specific (as StTapOnC aliases StOncePerTurn).
const (
	StAtkTaxPlayer = StOncePerTurn
	StAtkTaxWalker = StAnyManaType
)
