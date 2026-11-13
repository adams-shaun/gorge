package kop

// StatBlockTax is a printed S:Mode$ CantBlockUnless with a generic mana
// Cost$ (Archangel of Tithes, Qal Sisma Behemoth, Myr Prototype, Hipparion,
// Awesome Presence, Cowed by Wisdom, Oppressive Rays; CR 509.1c, 509.1d): a
// creature matching Filter can't block an attacker matching Filter2 unless
// its controller pays A (VA under StDynamic, read from the static's source
// and controller) generic mana for it. Filter2 == 0 is no Attacker$: every
// attacker. The statics matching a (blocker, attacker) pair sum, and Cond2
// is the static's gate (IsPresent$, Condition$, CheckSVar$). gorge prices it
// in rules/attack_cost.go blockPairCharge and pays it in the declare-blockers
// payment window; the kernel's half is dirac/combat_blocktax.go. It is the
// first kind past 255, hence StaticKind's width.
const StatBlockTax StaticKind = 256
