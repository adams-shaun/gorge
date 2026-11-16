package kop

// Repl.C bits of ReplDamageRedirect (dirac/repl_staticredirect.go): a
// permanent's printed static "all damage that would be dealt to <recipient>
// is dealt to <X> instead" (Pariah, Veteran Bodyguard, Treacherous Link, Harsh
// Judgment; gorge replacement_replace.go's ReplaceEffect VarName$ Affected
// rewrite). The lower three select the recipient the event must hit, the next
// four the recipient it is dealt to instead, the last five the gates.
const (
	RdrToYou        int16 = 1 << iota // ValidTarget$ You: the recipient is this source's controller (a player)
	RdrToYouPerms                     // ...or a permanent they control, this source's not among it (Permanent.Other+YouCtrl)
	RdrToEnchanted                    // ...or the object this source is attached to (Card.EnchantedBy)
	RdrAsSelf                         // VarValue$ Self: the damage is dealt to this source instead
	RdrAsAttached                     // ...to the object it is attached to (Enchanted, Equipped)
	RdrAsSrcCtrl                      // ...to the damage source's controller (ReplacedSourceController)
	RdrAsTgtCtrl                      // ...to the current recipient's controller (ReplacedTargetController)
	RdrGateUntapped                   // only while this source is untapped (IsPresent$ Card.Self+untapped)
	RdrGateEquipped                   // only while this source is attached to something (IsPresent$ Card.equipping PresentDefined$ Self)
	RdrCombatOnly                     // combat damage only (IsCombat$ True)
	RdrSrcUnblocked                   // only an unblocked attacking creature (ValidSource$ Creature.unblocked)
	RdrSrcChosen                      // a spell source of this source's chosen colour (ValidSource$ Instant.ChosenColor,Sorcery.ChosenColor)
)
