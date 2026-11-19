package kop

// MayGuardCopyParent: the OpMayAsk guard of an Optional$ CopySpellAbility
// Defined$ Parent (Sevinne's Reclamation's "you may copy this spell"; gorge
// effCopySpellAbility's copy_optional election): asked only when the next
// OpCopySpell has the resolving spell still on the stack to copy, else the
// copy is skipped unasked.
const MayGuardCopyParent = 13
