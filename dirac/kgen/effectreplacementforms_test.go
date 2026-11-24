package kgen

import "testing"

// effectReplacementForms: Effect replacements whose Duration$ is tied to the source on the battlefield.
var effectReplacementForms = []formCase{
	{card: "Somnophore", form: "AsLongAsInPlay untap lock", lir: "SkipUntap"},
	{card: "Tidebinder Mage", form: "AsLongAsInPlay untap lock", lir: "SkipUntap"},
	{card: "Origin of Iron Man", form: "UntilHostLeavesPlay untap lock (Saga chapter)", lir: "SkipUntap"},
	{card: "Amber Prison", form: "UntilUntaps untap lock (ExileOnMoved$)", lir: "SkipUntap"},
	{card: "Ice Floe", form: "UntilUntaps untap lock (land host)", lir: "SkipUntap"},
	{card: "Rust Tick", form: "UntilUntaps untap lock (artifact target)", lir: "SkipUntap"},
	{card: "Whip Vine", form: "UntilUntaps untap lock (blocked-by-source target)", lir: "SkipUntap"},
	{card: "Sand Squid", form: "UntilUntaps untap lock (creature target)", lir: "SkipUntap"},
	{card: "Godtoucher", form: "prevent all damage to the remembered target this turn", lir: "PreventNext"},
	{card: "Indestructible Aura", form: "prevent all damage to the remembered target this turn", lir: "PreventNext"},
	{card: "Shielded Passage", form: "prevent all damage to the remembered target this turn", lir: "PreventNext"},
	{card: "Gideon, Ally of Zendikar", form: "prevent all damage to the Effect's source, Duration$ UntilHostLeavesPlayOrEOT", lir: "PreventNext obj=Self C=2"},
	{card: "Gideon, the Oathsworn", form: "prevent all damage to the Effect's source, Duration$ UntilHostLeavesPlayOrEOT", lir: "PreventNext obj=Self C=2"},
	{card: "Djeru's Resolve", form: "prevent all damage to the remembered target this turn", lir: "PreventNext"},
	{card: "Take the Bait", form: "scoped prevention with no ActiveZones$ (the spell's own OpponentTurn$/ActivationPhases$ cast window still refuses)", refused: "unread params [ActivationPhases OpponentTurn]"},
	{card: "Ajani Steadfast", form: "Permanent emblem: prevent all but 1 to you and your planeswalkers", lir: "PreventDamageN"},
	{card: "Kurbis, Harvest Celebrant", form: "no ExileOnMoved", refused: "Effect replacement shape"},
}

func TestEffectReplacementForms(t *testing.T) { checkForms(t, effectReplacementForms) }
