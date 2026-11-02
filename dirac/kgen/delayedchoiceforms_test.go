package kgen

import (
	"strings"
	"testing"
)

// delayedchoiceformsCarriers is one card per form:
// each must lower with no refusal, and its lowered IR must contain the
// fragment.
var delayedchoiceformsCarriers = map[string]struct{ form, lir string }{
	"Prize Pig":                {"RemoveCounter CounterNum$ All of one kind (the object's count, read first)", "RemoveCounter obj=Self A=CountersOn(Self,RIBBON)"},
	"Clockwork Beetle":         {"DelayedTrigger Phase$ EndCombat (kop.DelayEndCombat)", "DelayedTrigger C=1 CS=NextEndCombat"},
	"Brass Squire":             {"Attach Object$ ParentTarget onto a target creature (OpAttachObj)", "AttachObj obj=Target1 obj2=Target0"},
	"Sigarda's Aid":            {"Attach Object$ TriggeredCardLKICopy onto a target creature", "AttachObj obj=Target0 obj2=Triggered"},
	"Infested Thrinax":         {"an until-end-of-turn Effect trigger with TriggerZones$ Battlefield (gorge never reads it)", "zone=7 step="},
	"Tyvar Kell":               {"a Duration$ Permanent emblem trigger without TriggerZones$", "emblem[0] trigger="},
	"Twiddle":                  {"api:TapOrUntap of one target (OpTapOrUntap)", "TapOrUntap obj=Target0"},
	"Canopy Claws":             {"api:Debuff Keywords$ Flying of a target (an EOT RemoveKw override)", "Override obj=Target0 CS=EOT // override{removekw=Flying}"},
	"Merfolk Branchwalker":     {"api:Explore of the source (OpExplore)", "Explore obj=Self"},
	"Leyline of the Guildpact": {"an override static SetColor$ All (gorge colorLetters)", "override{colors=WUBRG}"},
	"Abyssal Nightstalker":     {"Mode$ AttackerUnblocked ValidCard$ Card.Self (Tf2Unblocked)", "unblocked"},
	"Swamp Mosquito":           {"api:Poison of the defending player on an unblocked attack (OpPoison)", "Poison obj=Defending A=1"},
	"Holy Day":                 {"api:Fog (OpFog)", "Fog"},
	"Giant Caterpillar":        {"end-step DelayedTrigger without RememberObjects$ (a body that reads none)", "0 DelayedTrigger C=1 CS=NextEndStep"},
	"Aetherling":               {"end-step DelayedTrigger RememberObjects$ Remembered in an activated ability", "DelayedTrigger obj=Remembered C=1 CS=NextEndStep"},
	"Dragon Mask":              {"end-step DelayedTrigger RememberObjects$ ParentTarget", "DelayedTrigger obj=Target0 C=1 CS=NextEndStep"},
	"Archmage Emeritus":        {"Mode$ SpellCastOrCopy (Tf2OrCopy)", "or-copy"},
	"Soldier of Fortune":       {"api:Shuffle of a target player (OpShuffle)", "Shuffle obj=Target0"},
	"Blessed Wind":             {"api:SetLife of a target player (OpSetLife)", "SetLife obj=Target0 A=20"},
	"Lava Storm":               {"a spell's GenericChoice of the controller (OpChooseBranch at resolution)", "0 GenericChoice obj=You A=2 C=1\n     1 Branch skip=1"},
	"Tireless Provisioner":     {"a triggered ability's GenericChoice of the controller (placement: a one-mode charm)", "0 Charm skip=1"},
	"Tomb Fortress":            {"a may graveyard pick onto the battlefield (gorge effHiddenPick Min 0)", "ChooseMove obj=You A=1 C=1 CS=from=Graveyard| F="},
	"Arbalest Elite":           {"Effect Duration$ UntilTheEndOfYourNextUntap on itself (OpSkipUntap C=0)", "SkipUntap obj=Self"},
	"Bontu's Last Reckoning":   {"Effect Duration$ UntilTheEndOfYourNextUntap on your lands (OpSkipUntap C=1)", "SkipUntap C=1"},
	"Horn of the Mark":         {"Mode$ AttackersDeclaredOneTarget AttackedTarget$ Player (Tf2DefPlayer)", "defender-player=1"},
	"Honor the Past":           {"a return's StaticEffect$ AddType$ Artifact | RemoveCardTypes$ True (a set-types override)", "Override obj=Target0 CS=Permanent"},
	"Abuelo's Awakening":       {"a return's StaticEffect$ AddType$ Creature & Spirit, SetPower$/SetToughness$", "Override obj=Target0 CS=Permanent"},
	"Daring Discovery":         {"api:Discover Num$ 4 (OpDiscover, the free cast, the tail)", "Discover obj=You A=4"},
	"Weapon Rack":              {"api:MoveCounter Source$ Self onto a target (OpMoveCounter)", "MoveCounter obj=Target0 A=1 CS=P1P1 obj2=Self"},
	"Boon Reflection":          {"Event$ GainLife ReplaceCount$LifeGained/Twice (ReplLifeDouble)", "repl LifeDouble"},
	"Heron of Hope":            {"Event$ GainLife ReplaceCount$LifeGained/Plus.1 (ReplLifePlus)", "repl LifePlus"},
	"Ponder":                   {"api:RearrangeTopOfLibrary NumCards$ 3 MayShuffle$ (OpRearrange C=1)", "Rearrange obj=You A=3 C=1"},
	"Reckless Amplimancer":     {"Pump NumAtt$/NumDef$ Double (the object's current power and toughness)", "B=ToughnessOf(Self)"},
	"Aether Membrane":          {"DelayedTrigger Phase$ EndCombat RememberObjects$ TriggeredAttackerLKICopy", "DelayedTrigger obj=Triggered C=1 CS=NextEndCombat"},
}

// delayedchoiceformsRefused is one card per form left refused, with a
// fragment of the expected refusal reason.
var delayedchoiceformsRefused = map[string]string{
	"Armory Automaton":            "target slot 0 takes several objects", // "any number of target Equipment"
	"Auriok Survivors":            "with Optional$",
	"Shelkin Brownie":             "RemoveKeyword \"Bands with other\"",
	"Shifting Sky":                "SetColor \"ChosenColor\"",
	"Saltskitter":                 "in a triggered ability (the trigger capture)",
	"Crucias, Titan of the Waves": "GenericChoice of the controller below a triggered ability's head",
	"Cinder Marsh":                "in a mana ability",
	"Stoic Ephemera":              "delayed end-of-combat trigger ValidPlayer \"Opponent\"",
}

func TestDelayedAndChoiceFormsAccepted(t *testing.T) {
	var names []string
	for n := range delayedchoiceformsCarriers {
		names = append(names, n)
	}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		want, ok := delayedchoiceformsCarriers[c.Name]
		if !ok {
			continue
		}
		seen++
		for _, r := range c.Refusals {
			t.Errorf("%s (%s): refused: %s", c.Name, want.form, r.Reason)
		}
		var sb strings.Builder
		dumpLIR(&sb, c)
		if !strings.Contains(sb.String(), want.lir) {
			t.Errorf("%s (%s): lowered IR lacks %q:\n%s", c.Name, want.form, want.lir, sb.String())
		}
	}
	if seen != len(delayedchoiceformsCarriers) {
		t.Errorf("lowered %d of %d carriers", seen, len(delayedchoiceformsCarriers))
	}
}

func TestDelayedAndChoiceFormsRefused(t *testing.T) {
	var names []string
	for n := range delayedchoiceformsRefused {
		names = append(names, n)
	}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		want, ok := delayedchoiceformsRefused[c.Name]
		if !ok {
			continue
		}
		seen++
		found := false
		for _, r := range c.Refusals {
			if strings.Contains(r.Reason, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: no refusal containing %q (refusals %v)", c.Name, want, c.Refusals)
		}
	}
	if seen != len(delayedchoiceformsRefused) {
		t.Errorf("lowered %d of %d refused carriers", seen, len(delayedchoiceformsRefused))
	}
}
