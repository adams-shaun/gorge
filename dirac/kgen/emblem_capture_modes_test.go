package kgen

import "testing"

// emblemCaptureModes: the single-blocked carriers of the Effect-triggers
// RememberObjects$ class (census 11-25), one row each. RememberObjects$ Self
// read only as the body's Defined$ Remembered over SpellCast lowers (lower_effectremself.go:
// the emblem's source object), and Targeted over an end-of-turn Attacks line
// lowers as the emblem's capture (lower_effeottrig.go emblemCapture); every
// other sub-shape stays refused by name.
var emblemCaptureModes = []formCase{
	// Self: lowered when the set is read only as Defined$ Remembered
	{card: "Mountain Titan", form: "Self, SpellCast, PutCounter on Defined$ Remembered", lir: "PutCounter obj=Self"},
	{card: "Dovin, Grand Arbiter", form: "Self, DamageDone (diverges on simultaneous combat damage)", refused: `RememberObjects$ "Self" over Mode$ DamageDone`},
	// Self: refused where the set is a filter
	{card: "Electric Seaweed", form: "Self, ChangesZone ValidCard$ Creature.!IsRemembered", refused: `RememberObjects$ "Self" and trigger ValidCard$`},
	{card: "Massacre Girl", form: "Self, PumpAll ValidCards$ Creature.!IsRemembered", refused: `RememberObjects$ "Self" and body ValidCards$`},
	// Targeted: lowered over an end-of-turn Attacks line (the kernel's Attacks
	// emblem hook hands the line only the captured attacker, emblemAtkListeners)
	{card: "Maeve, Insidious Singer", form: "Targeted, Attacks ValidCard$ Creature.IsRemembered", lir: "until=EOT remembers=Target0"},
	// Targeted: refused where the Phase and next-turn DamageDone emblem hooks carry no capture,
	// or the Attacks line keeps a gate the kernel lacks
	{card: "Okoye, Mighty and Adored", form: "Targeted, Attacks Attacked$ Player.isMonarch (no monarch defender gate)", refused: `Attacks Attacked "Player.isMonarch"`},
	{card: "Kang Dynasty", form: "Targeted, DamageDone, Duration$ UntilYourNextTurn", refused: `Effect triggers with RememberObjects$ "Targeted"`},
	{card: "Blinding Beam", form: "Targeted player, Phase Untap beside a replacement", refused: `Effect triggers with RememberObjects$ "Targeted"`},
	{card: "Quenchable Fire", form: "Targeted, OneOff$ Phase beside Abilities$ PayUp", refused: `Effect triggers with RememberObjects$ "Targeted"`},
	{card: "Telekinesis", form: "Targeted, Phase Untap beside a replacement", refused: `Effect triggers with RememberObjects$ "Targeted"`},
	{card: "Trench Behemoth", form: "Targeted, Phase BeginCombat must-attack", refused: `Effect triggers with RememberObjects$ "Targeted"`},
	{card: "Sizzling Soloist", form: "Targeted, Phase BeginCombat must-attack", refused: `Effect triggers with RememberObjects$ "Targeted"`},
	// the other values
	{card: "Carom", form: "ParentTarget beside a redirect replacement", refused: `Effect triggers with RememberObjects$ "ParentTarget"`},
	{card: "Razia, Boros Archangel", form: "ParentTarget beside a redirect replacement", refused: `Effect triggers with RememberObjects$ "ParentTarget"`},
	{card: "Ogre Geargrabber", form: "ParentTarget, ChangesController", refused: `Effect triggers with RememberObjects$ "ParentTarget"`},
	{card: "Nafs Asp", form: "TriggeredTarget player, OneOff$ Phase beside Abilities$ PayUp", refused: `Effect triggers with RememberObjects$ "TriggeredTarget"`},
	{card: "Glass Asp", form: "TriggeredTarget player, OneOff$ Phase beside Abilities$ PayUp", refused: `Effect triggers with RememberObjects$ "TriggeredTarget"`},
	{card: "Sabertooth Cobra", form: "TriggeredTarget player, OneOff$ Phase beside Abilities$ PayUp", refused: `Effect triggers with RememberObjects$ "TriggeredTarget"`},
	{card: "Hallow", form: "TargetedSource, a spell on the stack", refused: `Effect triggers with RememberObjects$ "TargetedSource"`},
	{card: "Horn of Plenty", form: "TriggeredActivator, OneOff$ end step", refused: `Effect triggers with RememberObjects$ "TriggeredActivator"`},
	{card: "Lucius the Eternal", form: "TriggeredCard beside ImprintCards$", refused: `Effect triggers with RememberObjects$ "TriggeredCard"`},
}

func TestEmblemCaptureModes(t *testing.T) { checkForms(t, emblemCaptureModes) }
