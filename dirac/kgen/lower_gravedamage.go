package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// graveDamageKeys are the parameters of a graveyard DamageDone line the
// kernel reads (State.fireGraveDamage): the source filter, the damaged
// player or planeswalker, the combat gate and the body.
var graveDamageKeys = map[string]bool{
	"Mode": true, "ValidSource": true, "ValidTarget": true, "TriggerZones": true, "Execute": true,
	"TriggerDescription": true, "CombatDamage": true,
}

// graveDamageLine reports a Mode$ DamageDone line the kernel's graveyard
// damage walk (dirac/trig_gravedamage.go) reads exactly: no other parameter
// (an IsPresent$ gate, a damage-amount gate, an Optional$ rider) rides it.
func graveDamageLine(t cards.Trigger) bool {
	if t.Mode != "DamageDone" {
		return false
	}
	for k := range t.Params {
		if !graveDamageKeys[k] {
			return false
		}
	}
	return true
}

// dropSpellSourceHead removes the Spell base from a damage source filter's
// clauses that name only instants or sorceries ("Spell.Instant+YouCtrl+Red"):
// an instant or sorcery is never a permanent, so the base (an object on the
// stack) adds nothing to the type word for a damage source, and the kernel's
// spell-source walk matches the card's own characteristics. A Spell clause
// with any other type word keeps the base and refuses.
func dropSpellSourceHead(f *lFilter) error {
	if f == nil {
		return nil
	}
	for i := range f.Clauses {
		c := &f.Clauses[i]
		if !c.SpellBase {
			continue
		}
		if c.Types == 0 || c.Types&^(tInstant|tSorcery) != 0 {
			return refuse("a Spell head over a source that may be a permanent")
		}
		c.SpellBase = false
	}
	return nil
}
