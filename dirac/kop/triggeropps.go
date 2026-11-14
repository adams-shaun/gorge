package kop

// RefTriggerOpponents is a player-set ref: each opponent of the trigger's
// player (gorge Defined$ TriggeredCardOpponent, "each of that player's
// opponents"), read in AliveFrom(controller) order without the trigger player
// (a SpellCast trigger's caster, so the source's own controller is in the set
// when an opponent cast the spell). Standstill's draw. High, clear of the
// appended Ref enum; read only by the player-set expansion (OpDraw).
const RefTriggerOpponents Ref = 247
