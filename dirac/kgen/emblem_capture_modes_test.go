package kgen

import "testing"

// emblemCaptureModes: the single-blocked carriers of the Effect-triggers
// RememberObjects$ class (census 11-25), one row each. RememberObjects$ Self
// read only as the body's Defined$ Remembered over SpellCast or DamageDone
// lowers to the emblem's source object, kop.RefEmblemSource (lower_effeottrig.go
// emblemSelfRemember; the kernel queues the emblem trigger at srcVer = ver+1,
// so plain RefSelf names nothing there); Targeted over an end-of-turn Attacks
// line lowers as the emblem's capture (lower_effeottrig.go emblemCapture);
// TriggeredCard over a Phase or ChangesZone trigger lowers as a capture of
// the creating event's card (fireEmblemsAmt / fireEmblemsObj queue it as the
// trigger object); a Targeted player's untap denial lowers as SkipUntap over
// the target seat (lower_untapdenial.go). Every other sub-shape stays refused
// by name.
var emblemCaptureModes = []formCase{
	// Self: lowered when the set is read only as Defined$ Remembered
	{card: "Mountain Titan", form: "Self, SpellCast, PutCounter on Defined$ Remembered", lir: "PutCounter obj=EmblemSource A=1 CS=P1P1"},
	{card: "Dovin, Grand Arbiter", form: "Self, DamageDone (mbcselfrem pool plays it in lockstep)", lir: "PutCounter obj=EmblemSource"},
	// Self: refused where the set is a filter
	{card: "Electric Seaweed", form: "Self, ChangesZone ValidCard$ Creature.!IsRemembered", refused: `RememberObjects$ "Self" and trigger ValidCard$`},
	{card: "Massacre Girl", form: "Self, PumpAll ValidCards$ Creature.!IsRemembered", refused: `RememberObjects$ "Self" and body ValidCards$`},
	// Targeted: lowered over an end-of-turn Attacks line (the kernel's Attacks
	// emblem hook hands the line only the captured attacker, emblemAtkListeners)
	{card: "Maeve, Insidious Singer", form: "Targeted, Attacks ValidCard$ Creature.IsRemembered", lir: "until=EOT remembers=Target0"},
	// Targeted player: lowered as the untap-step restriction the target
	// seat's next untap skips (the nearest accepted form beside Exhaustion)
	{card: "Blinding Beam", form: "Targeted player, Phase Untap replacement", lir: "SkipUntap obj=Target1 C=7"},
	// TriggeredCard: lowered over the Phase event the kernel hands the
	// captured object to; the plane's chaos static still refuses by its own name
	{card: "Agyrem", form: "TriggeredCard, Phase End of Turn", lir: "emblem[0] trigger=1 statics=0 remembers=Triggered once", refused: `Effect StaticAbilities$ "STCantAttack"`},
	// Targeted: refused where the Phase and next-turn DamageDone emblem hooks carry no capture,
	// or the Attacks line keeps a gate the kernel lacks
	{card: "Okoye, Mighty and Adored", form: "Targeted, Attacks Attacked$ Player.isMonarch (no monarch defender gate)", refused: `Attacks Attacked "Player.isMonarch"`},
	{card: "Kang Dynasty", form: "Targeted, DamageDone, Duration$ UntilYourNextTurn", refused: `Effect triggers with RememberObjects$ "Targeted"`},
	{card: "Quenchable Fire", form: "Targeted, OneOff$ Phase beside Abilities$ PayUp", refused: `Effect triggers with RememberObjects$ "Targeted"`},
	{card: "Telekinesis", form: "Targeted object, Phase Untap beside a damage-prevention replacement", refused: `Effect triggers with RememberObjects$ "Targeted"`},
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
	{card: "Lucius the Eternal", form: "TriggeredCard over ChangesZone beside ImprintCards$ (the imprint capture is a separate shape)", refused: `IsImprinted outside a chain that writes the imprinted set`},
}

func TestEmblemCaptureModes(t *testing.T) { checkForms(t, emblemCaptureModes) }
