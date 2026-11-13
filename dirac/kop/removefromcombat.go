package kop

// ExtRemoveFromCombat (CR 506.4): the battlefield permanent Obj (one object:
// a target slot, the source, the trigger object or the source's host) is
// removed from combat (gorge effRemoveFromCombat: one events.EndCombatReset
// for it): it stops attacking and blocking; an attacker it blocked stays
// blocked with one blocker fewer (CR 509.1h), and a blocker of it stays
// blocking but blocks nothing. It stays tapped or untapped. An object off
// the battlefield does nothing. No other field is read. Reconnaissance,
// Nivea, Beloved Battlemage, the Gustcloak cycle. dirac/combat_noncreature.go.
const ExtRemoveFromCombat ExtOp = 260

func init() { extOpNames[ExtRemoveFromCombat] = "RemoveFromCombat" }
