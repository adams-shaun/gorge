package trigmatch

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// ExcessDamage (edh-trigB) is Mode$ ExcessDamageAll's singular sibling:
// "Whenever a creature or planeswalker an opponent controls is dealt excess
// noncombat damage" (Toralf, God of Fury). It fires PER Damage event, not
// once per batch, so it shares DamageMatches -- whose ValidTarget$/
// CombatDamage$ arms are exactly this mode's two printed filters -- and the
// per-event excess amount rules' damageTriggerAmount computes for the mode.
// Registered beside its amount plumbing (rules' excessdamageall.go) rather
// than in combat.go's DamageDone group, so the once-per-batch damage modes'
// file stays the batch modes' file.
func init() {
	registerTrigMatcher(func(e Board, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
		return DamageMatches(e, t, source, ev)
	}, "ExcessDamage")
	effects.RegisterNonAPI("trig:ExcessDamage")
}
