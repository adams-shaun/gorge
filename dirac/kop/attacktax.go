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

// StCantAtkPlayer and StCantAtkWalker are a StatCantAttack static's Target$
// scope, the same grammar as the attack tax's StAtkTaxPlayer and
// StAtkTaxWalker: an attack on the static's controller (Target$ You) and on a
// planeswalker that player controls (Planeswalker.YouCtrl). A battle is
// neither target, so no battle pair is blocked (gorge
// combat.RestrictionTargetMatches: a player part is read only when nothing
// permanent is attacked, the walker clause needs a Planeswalker. head).
// Neither set is no Target$: every attack is forbidden. StaticFlags is full
// at 16 bits, so they alias the MayPlayFrom-only StOncePerTurn and
// StAnyManaType, which StatCantAttack never reads (as StAtkTaxPlayer does).
const (
	StCantAtkPlayer = StOncePerTurn
	StCantAtkWalker = StAnyManaType
)

// RefTaxedAttacker is the creature an attack-tax charge is being priced for
// (Nils, Discipline Enforcer's RememberingAttacker$): atkTaxCharge binds it
// beside each (attacker, defender) pair it prices, the charge
// Remembered$CardCounters.ALL reads the priced creature's live counters off
// it. None outside a charge (the uncounted convention: 0 reads none).
const RefTaxedAttacker Ref = 38
