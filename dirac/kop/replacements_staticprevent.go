package kop

// ReplPreventDamageFilt and ReplPreventDamageN: a permanent's printed static
// prevention of damage (gorge R:Event$ DamageDone). A full one is
// Prevent$ True; a prevent-N shield carries DB$ ReplaceDamage | Amount$ N,
// which prevents min(N, the damage left) of each damage event it matches
// (CR 615.7). Both carry the recipient and the source they apply to:
//
//   - Repl.Filter matches a permanent recipient, relative to the host (its
//     controller, EnchantedBy / EquippedBy); with no filter only
//     ReplSPAnyPerm covers a permanent. ReplSPYou / ReplSPOpp cover the host
//     controller / its opponents as player recipients.
//   - Repl.Filter2 matches the source (a permanent or a card on the stack);
//     0 is any source.
//   - Repl.Cond is the card's CheckSVar$ / IsPresent$ gate, read with the
//     host as the source.
//
// Repl.C is a set of the ReplSP* bits. The amount of a prevent-N shield is
// Repl.A, or Repl.V under ReplSPAmtV.
const (
	ReplSPYou       = 1 << iota // a player recipient: the host's controller
	ReplSPOpp                   // a player recipient: an opponent of the host's controller
	ReplSPAnyPerm               // every permanent is a recipient (no Repl.Filter)
	ReplSPCombat                // combat damage only (IsCombat$ True)
	ReplSPNonCombat             // noncombat damage only (IsCombat$ False)
	ReplSPYourTurn              // only while the host's controller is the active player (PlayerTurn$ True)
	ReplSPSrcInPlay             // the source is a permanent on the battlefield (ValidSource$ ...inZoneBattlefield)
	ReplSPDmgLE                 // only a damage event of at most Repl.A (DamageAmount$ LEn)
	ReplSPAmtV                  // ReplPreventDamageN: the amount is Repl.V
	ReplSPAllBut                // ReplPreventDamageN: prevents all but Repl.A of the damage (ReplaceCount$DamageAmount/Minus.A)
	ReplSPLimitCtr              // ReplPreventDamageN: prevents the damage up to the host's count of the counter kind Repl.A (Rock Hydra)
	ReplSPOptional              // Optional$ True: its controller may decline each time (Battletide Alchemist)
	ReplSPRecipSnow             // the recipient permanent is a Snow card (Creature.Snow: Goblin Furrier); the printed supertype only
	ReplSPSrcSpell              // the source is the resolving spell (ValidSource$ Spell: Plated Pegasus), not an ability or a permanent
)
