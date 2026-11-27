package kop

// RefEnchantedPlayer is a player ref: the player the source Aura enchants
// (gorge Defined$ / TokenOwner$ EnchantedPlayer, definedSpecEnchantedPlayer:
// the resolving source's own AttachedPlayer). None when the source is attached
// to no player or has left the battlefield (CR 704.5m): the op acts on nobody,
// as gorge's fail-closed owner read does. Overencumbered's enters trigger.
const RefEnchantedPlayer Ref = 48

// WhoseEnchantedOpp restricts a printed battlefield Phase trigger to the turn
// of the player its source Aura enchants when that player is also an opponent
// of the trigger's controller (gorge ValidPlayer$ Opponent.EnchantedBy: the
// Opponent base, then playerSpecQualifierEnchantedBy). Read with the
// listener's slot (defEnchanted); elsewhere it fails closed.
const WhoseEnchantedOpp Whose = 204
