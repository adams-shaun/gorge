package kgen

import "testing"

// unreadForms: the unread-params lowerings (MageZero sample
// decks, CARD-CENSUS.md), one carrier per parameter, checked like paramreadformsForms.
var unreadForms = []formCase{
	{card: "World Shaper", form: "Mill Optional$ True: a yes/no before the mill (gorge mill_optional)", lir: "MayAsk skip=1 // you may"},
	{card: "Cori-Steel Cutter", form: "Attach Optional$ True: a yes/no before the attach (gorge attach_optional)", lir: "MayAsk C=1 skip=1 // you may"},
	{card: "Reckless Impulse", form: "Dig Imprint$ True on a spell that never reads it: consumed (the record dies with the spell)", lir: "MayPlay obj=Remembered"},
	{card: "Citadel Gate", form: "ChooseColor Exclude$ white: C=1, the excluded colour bits (gorge chooseColorOptions)", lir: "ChooseColor obj=You C=1"},
	{card: "Ascendant Packleader", form: "etbCounter IsPresent$ on the replacement: a Present condition on the enters program", lir: "cond=Present({types=0x9f ctrl=1 cmp(2 1 4  )},op1,1)"},
	{card: "Confront the Assault", form: "a spell's IsPresent$ is a cast restriction (spell Trig.Cond; gorge cr-spell-ispresent-cast-gate; attackingYou: an attacking creature an opponent controls)", lir: "activate turnonce=false cond=Present({types=0x2 ctrl=2 Attacking},op1,1)"},
	{card: "Brainstorm", form: "hand put-back without ChangeType$: a mandatory hand move of 2 to the library (gorge effChangeZoneHand)", lir: "ChooseMove obj=You A=2 C=4 CS=from=Hand|Mandatory"},
	{card: "Lembas", form: "ChangeNum$ 1 / Mandatory$ on a Defined$ shuffle-in: inert (gorge's object path)", lir: "ToLibrary obj=TriggeredNew CS=TopShuffle|from=Graveyard"},
	{card: "Floodpits Drowner", form: "ThisDefinedAndTgts$ Self on a shuffled move: the target, then this, one shuffle per owner", lir: "ToLibraryPair obj=Target0"},
	{card: "Deep-Cavern Bat", form: "RevealHand Look$ ack, then a hand exile from the targeted player's hand until this leaves", lir: "ChooseMove obj=Target0 A=1 C=5 CS=from=Hand|ChooserYou|UntilLeaves"},
	{card: "Gleeful Demolition", form: "RememberLKI$ + AlwaysRemember$ read by ConditionDefined$ RememberedLKI: the destroy's target", lir: "SkipIf if=RefMatches({types=0x0 ctrl=1},Target0,op1,1) skip=1"},
	{card: "Thirst for Discovery", form: "Discard UnlessType$ Land.Basic: OpDiscardUnless (gorge discard_unless)", lir: "DiscardUnless obj=You A=2"},
	{card: "Snap", form: "Untap UntapType$ Land Amount$ 2 UntapUpTo$: OpUntapChoose (gorge's untap KChoose, Min 0)", lir: "UntapChoose A=2 C=1 F={types=0x1}"},
	{card: "Thraben Charm", form: "TargetMin$ 0 over target players: none targeted moves nothing (gorge cr-changezoneall-zero-targets)", lir: "target[2] mode=2 zone=0 min=0 max=2 F={players=3}"},
	{card: "Glissa Sunslayer", form: "RemoveCounter CounterType$ Any + UpTo$: a kind pick per counter with Done (Ext RemoveCounterAny; gorge cr-removecounter-any-upto)", lir: "RemoveCounterAny obj=Target1 B=3 C=1"},
	{card: "Sharp-Eyed Rookie", form: "Condition$ Evolve: the ETB trigger fires only for a creature with greater power or toughness (gorge evolveConditionHolds)", lir: "PutCounter obj=Self A=1 CS=P1P1"},
}

func TestUnreadForms(t *testing.T) { checkForms(t, unreadForms) }
