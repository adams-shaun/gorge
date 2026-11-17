package kgen

import "testing"

// triggeredObjectRefForms: Defined$ Triggered* spellings the trigger-object
// table (lower_trigrefs.go) admits, one carrier per (spelling, mode) row,
// plus the modes that keep refusing with the mode named.
var triggeredObjectRefForms = []formCase{
	{card: "Phyrexian Obliterator", form: "DamageDone to Self: TriggeredSourceController (source is the trigger object)", lir: "Sacrifice obj=TriggeredTgtCtrl A=TriggerAmount"},
	{card: "Belltower Sphinx", form: "DamageDone to Self: TriggeredSourceController mills", lir: "Mill obj=TriggeredTgtCtrl A=TriggerAmount"},
	{card: "Charisma", form: "DamageDone from the enchanted creature to a creature: TriggeredTarget", lir: "GainControl obj=Triggered CS=dur=WhileSource"},
	{card: "Sword of Kaldra", form: "DamageDone from the equipped creature to a creature: TriggeredTargetLKICopy", lir: "Exile obj=Triggered CS=from=Battlefield"},
	{card: "Cloud Cover", form: "BecomesTarget watcher: TriggeredTargetLKICopy", lir: "Bounce obj=Triggered CS=from=Battlefield"},
	{card: "Ferocity", form: "Blocks over the enchanted creature: the blocker is the host", lir: "PutCounter obj=Equipped A=1 CS=P1P1"},
	// a source that is not one object keeps the plain trigger, whose trigger
	// object is the source, not the damaged creature: refused, mode named
	{card: "Toxin Sliver", form: "DamageDone from any Sliver: TriggeredTargetLKICopy", refused: `outside a damage trigger the kernel's trigger object or player names (Mode$ DamageDone)`},
	{card: "Doran, Besieged by Time", form: "Blocks of any creature you control: blocker not recorded", refused: `outside a whenever-this-blocks trigger (Mode$ Blocks)`},
	{card: "Enormous Energy Blade", form: "Attached: mode records no target object", refused: `(Mode$ Attached)`},
}

func TestTriggeredObjectRefForms(t *testing.T) {
	checkForms(t, triggeredObjectRefForms)
}
