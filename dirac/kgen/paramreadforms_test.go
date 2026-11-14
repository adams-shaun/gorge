package kgen

import "testing"

// paramreadformsForms: lowerings, one carrier
// per class, checked like filterpredicateformsForms.
var paramreadformsForms = []formCase{
	{card: "Annihilate", form: "Destroy NoRegen$ True (OpDestroy C=1)", lir: "0 Destroy obj=Target0 C=1"},
	{card: "Damnation", form: "DestroyAll NoRegen$ True (OpDestroyAll C&2)", lir: "0 DestroyAll C=2 F={types=0x2}"},
	{card: "Lord of the Pit", form: "Sacrifice RememberSacrificed$ True (OpSacrifice B.N&1)", lir: "0 Sacrifice obj=You A=1 F={types=0x2 Other} // remember sacrificed\n     1 SkipIf if=Value(op5,RememberedCount(ctx),1) skip=1"},
	{card: "Nylea's Colossus", form: "Pump NumDef$ Double (the object's toughness, gorge NumForObject)", lir: "0 PumpUntilEOT obj=Target0 A=PowerOf(Target0) B=ToughnessOf(Target0)"},
	{card: "Impulse", form: "Dig NoReveal$ True (a private look: gorge RevealWin = Reveal && !NoReveal)"},
	{card: "Goblin Guide", form: "Dig LibraryPosition2$ 0 with one card in the window (kop.DigRestTop)", lir: "0 Dig obj=Defending A=1 B=-1 C=2 CS=rest=Library|RestTop|Reveal F={types=0x1}"},
	{card: "Bamboozle", form: "LibraryPosition2$ 0 leaving two on top: gorge asks their order (digTopOrderAsk, kop.DigRestTopAsk)", lir: "CS=rest=Library|RestTop|RestTopAsk|Reveal"},
	{card: "Gray Merchant of Asphodel", form: "Count$Devotion.Black (ExDevotion)", lir: "Devotion"},
	{card: "Fledgling Osprey", form: "enchanted (PredEnchanted)", lir: "Self cmp(10 0 0  )"},
	{card: "Memory Lapse", form: "Counter Destination$ TopOfLibrary (OpCounter C=CtrDestLibTop)", lir: "Counter obj=Target0 C=3"},
	{card: "Jeskai Elder", form: "a trigger's own Cost$ Draw<1/You> (IfPaid kop.PayDraw)", lir: "0 IfPaid obj=You A=1 CS=Draw skip=1"},
	{card: "Shu Cavalry", form: "printed Horsemanship (CardDef.Evasion)", lir: " evasion=1"},
	{card: "Accursed Spirit", form: "printed Intimidate (CardDef.Evasion)", lir: " evasion=2"},
	{card: "Morbid Curiosity", form: "Sacrificed$CardManaCost of a spell's sacrifice cost (ExMVOf RefSacrificed)", lir: "MVOf(Sacrificed)"},
	{card: "Industrial Advancement", form: "Sacrificed$CardManaCost of a trigger's own IfPaid Sacrifice cost (ExRemLKIPT N 3)", lir: "Dig obj=You A=RemLKIPT(n=3)"},
	{card: "Bolg of the North", form: "Spawner>Sacrificed$CardPower in a reflexive trigger (the parent's sacrificed power captured as the trigger's amount; gorge reads LKI)", lir: "TriggerAmount"},
	{card: "Divine Offering", form: "Targeted$CardManaCost of a battlefield target (ExMVOf RefTarget0, kernel targetMV)", lir: "GainLife obj=You A=MVOf(Target0)"},
	{card: "Hisoka, Minamo Sensei", form: "Targeted$CardManaCost of a spell target lowers (kernel targetMV adds the stack X); the card stays refused on Discarded$CardManaCost", refused: "Discarded$CardManaCost"},
	{card: "Murasa Behemoth", form: "static IsPresent$ PresentZone$ Graveyard (a graveyard count condition)", lir: "cond=Value(op1,CountGraveyard({types=0x1 owner=1}),1)"},
	{card: "Dreadlight Monstrosity", form: "activation IsPresent$ PresentZone$ Exile (an exile count condition)", lir: "cond=Value(op1,CountExile({types=0x0 owner=1}),1)"},
	{card: "Malakir Blood-Priest", form: "Count$Party (ExParty)", lir: "Party"},
	{card: "Clear a Path", form: "withDefender (PredHasKeyword, the keyword's bit)", lir: "cmp(11 0 10  )"},
	{card: "Clip Wings", form: "Sacrifice SacMessage$ is prompt text (gorge never reads it)"},
	{card: "Nether Spirit", form: "trigger IsPresent$ PresentZone$ Graveyard (a graveyard count intervening if)", lir: "cond=Value(op3,CountGraveyard({types=0x2 owner=1}),1)"},
	{card: "Armament Master", form: "Attached (PredAttached)", lir: "cmp(12 0 0  )"},
	{card: "Karma", form: "ActivePlayerCtrl (PredActiveCtrl)", lir: "cmp(13 0 0  )"},
	{card: "Dark Confidant", form: "Remembered$CardManaCost (ExMVOf RefRemembered: the remembered set's mana values summed)", lir: "1 LoseLife obj=You A=MVOf(Remembered)"},
	{card: "Frost Lynx", form: "Pump KW$ HIDDEN This card doesn't untap during your next untap step. (OpSkipNextUntap)", lir: "1 SkipNextUntap obj=Target0"},
	{card: "Inspired Fire", form: "Count$YouDrewThisTurn (ExDrawnThisTurn)", lir: "DrawnThisTurn"},
	{card: "Demolisher Spawn", form: "trigger Delirium$ True (ExGraveTypes >= 4)", lir: "cond=Value(op1,GraveTypes,4)"},
	{card: "Grim Flayer", form: "static Condition$ Delirium", lir: "cond=Value(op1,GraveTypes,4) P=2 T=2"},
	{card: "Erratic Explosion", form: "DigUntil RememberFound$ True (DuRememberFound)", lir: "RememberFound"},
	{card: "Boggart Ram-Gang", form: "printed Wither (CardDef.Wither)", lir: " wither"},
	{card: "Geralf's Messenger", form: "K:Undying lowered from gorge's expanded dies trigger", lir: "Return obj=TriggeredNew A=1 C=1 CS=from=Graveyard|counters=P1P1"},
	{card: "Batterskull", form: "K:Living Weapon lowered from gorge's expanded ETB trigger", lir: "CreateToken obj=You A=1 C=4 CS=b_0_0_phyrexian_germ\n     1 Attach obj=Remembered"},
	{card: "Ashling, the Extinguisher", form: "ControlledBy TriggeredTarget in its own combat-damage-to-a-player trigger (PredTrigPlayerCtrl)", lir: "F={types=0x2 cmp(16 0 0  )}"},
	{card: "Chalice of the Void", form: "an {X}{X} cost (ManaCost.X = 2)", lir: "cost={0XX}"},
	{card: "Feral Lightning", form: "Token AtEOT$ Exile (TokExileAtEnd)", lir: "CreateToken obj=You A=3 C=16"},
	{card: "Lowland Oaf", form: "Pump AtEOT$ Sacrifice (OpAtEOT after the pump)", lir: "1 GrantKeyword obj=Target0 CS=Flying\n     2 AtEOT obj=Target0"},
	{card: "All Hallow's Eve", form: "IsPresent$ Card.Self PresentZone$ Exile: the card matcher has no source, kept refused", refused: "of a source-relative filter"},
	{card: "Patrol Signaler", form: "{Q} cost (kop.CostUntapSelf)", lir: "cost={1W} Q"},
	{card: "Forget", form: "Discard RememberDiscarded$ True; Remembered$Amount (the remembered set's size)", lir: "0 Discard obj=Target0 A=2 CS=DiscarderChooses // remember discarded\n     1 Draw obj=Target0 A=RememberedCount(ctx)"},
	{card: "Primal Might", form: "Fight TargetMin$ 0 TargetMax$ 1 (up to one target, CR 601.2c)", lir: "target[1] mode=-1 zone=1 min=0 max=1 F={types=0x2 ctrl=2}"},
}

func TestParamReadForms(t *testing.T) { checkForms(t, paramreadformsForms) }
