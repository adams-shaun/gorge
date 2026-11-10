package kgen

import "testing"

// effectStaticTriggerPairForms: a DB$ Effect that remembers its targeted
// creature and carries a static beside a trigger lowers when the pair is one
// gorge applies exactly.
var effectStaticTriggerPairForms = []formCase{
	// Long River Lurker: an unblockable record on the target, and a capturing
	// emblem whose DamageDealtOnce trigger flickers the damage source
	{card: "Long River Lurker", form: "the static is the unblockable record on the Effect's target", lir: "GrantKeyword obj=Target0 CS=Unblockable"},
	{card: "Long River Lurker", form: "the emblem remembers that target and lapses when it leaves", lir: "emblem[0] trigger=1 statics=0 until=EOT remembers=Target0 lapses"},
	{card: "Long River Lurker", form: "the trigger is the optional combat DamageDealtOnce of a creature, any recipient", lir: "trigger DamageDealtOnce F={types=0x2} F2=nil whose=0 zone=7 step= combat=1 opt=true"},
	{card: "Long River Lurker", form: "the body's first Remembered read is the damage source", lir: "Exile obj=TriggeredSource CS=Remember|from=Battlefield"},
	{card: "Long River Lurker", form: "the return reads the set the exile wrote", lir: "Return obj=Remembered C=1 CS=from=Exile"},
	// The Pride of Hull Clade: gorge registers nothing for the Effect's
	// Card.Self copy of the trigger (the Effect never deals damage), so the
	// Effect is the capturing CanAttackDefender emblem alone
	{card: "The Pride of Hull Clade", form: "the Effect is a capturing emblem of one static", lir: "emblem[0] trigger=0 statics=1 until=EOT remembers=Target0 lapses"},
	{card: "The Pride of Hull Clade", form: "the Animate grant stays the one DamageDone trigger", lir: "trigger DamageDone F={types=0x0 Self} F2={players=3}"},
}

func TestEffectStaticTriggerPair(t *testing.T) { checkForms(t, effectStaticTriggerPairForms) }
