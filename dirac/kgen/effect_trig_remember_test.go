package kgen

import (
	"strings"
	"testing"
)

// An Effect's trigger bound to the set RememberObjects$ names lowers in three
// shapes (lower_effectremember.go): a Remembered count carried to a one-off
// next-upkeep trigger, Targeted over both of the chain's target slots beside
// a this-turn dies trigger, and RememberedLKI over the Effect's own source
// leaving. The real cards are checked by form; the probes below are inline
// stand-ins whose holes are what the refused variants change.
var effectTrigRememberForms = []formCase{
	{card: "The Eagles Are Coming!", form: "the Effect's remembered count is frozen into a one-off emblem", lir: "emblem[0] trigger=1 statics=0 number-as-amount once"},
	{card: "The Eagles Are Coming!", form: "the Effect link reads the resolution's remembered count", lir: "CreateEmblem obj=You A=RememberedCount(ctx)"},
	{card: "The Eagles Are Coming!", form: "the trigger's TokenAmount$ reads the carried amount, not a remembered set", lir: "CreateToken obj=You A=TriggerAmount"},
	{card: "T'Pring, Never Parted", form: "the emblem remembers both targeted fighters", lir: "emblem[0] trigger=1 statics=0 until=EOT remembers=Target0 and Target1"},
	{card: "T'Pring, Never Parted", form: "the Effect is made only while both targets are still creatures", lir: "SkipIf if=And(RefMatches({types=0x2},Target0,op1,1),RefMatches({types=0x2},Target1,op1,1))"},
	{card: "T'Pring, Never Parted", form: "the dies trigger's filter is the type check only (the emblem holds the set)", lir: "trigger Dies F={types=0x2}"},
	{card: "Mysterio, Master of Illusion", form: "the emblem remembers the tokens, ends with the host and fires for it alone", lir: "emblem[0] trigger=1 statics=0 remembers=set source not-here"},
	{card: "Mysterio, Master of Illusion", form: "the leave trigger's body is the exile of the emblem's own set", lir: "ExileEmbRem obj=Self"},
}

func TestLowerEffectTrigRemember(t *testing.T) {
	checkForms(t, effectTrigRememberForms)

	const count = `Name:Count Probe
ManaCost:1 W
Types:Instant
A:SP$ ChangeZone | Origin$ Battlefield | Destination$ Hand | ValidTgts$ Creature.YouOwn | TargetMin$ 1 | TargetMax$ 2 | RememberChanged$ True | SubAbility$ DBDelay
SVar:DBDelay:DB$ Effect | Triggers$ UpTrig | RememberObjects$ Remembered DURATION| SubAbility$ DBEnd
SVar:UpTrig:Mode$ Phase | Phase$ PHASE | Execute$ TrigDraw ONEOFF
SVar:TrigDraw:DB$ Draw | NumCards$ DrawX
SVar:DBEnd:DB$ Cleanup | ClearRemembered$ True
SVar:DrawX:Remembered$Amount
Oracle:probe
`
	const pair = `Name:Pair Probe
ManaCost:3 G U
Types:Creature Human
PT:3/3
T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigPick | TriggerDescription$ probe
SVar:TrigPick:DB$ Pump | ValidTgts$ Creature.YouCtrl+Other | SubAbility$ DBFight
SVar:DBFight:DB$ Fight | Defined$ ParentTarget | ValidTgts$ Creature.OppCtrl | SubAbility$ DBDelay
SVar:DBDelay:DB$ Effect | Triggers$ Dies | RememberObjects$ Targeted | ConditionDefined$ Targeted | ConditionPresent$ Creature | ConditionCompare$ CMP
SVar:Dies:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ VALID | TriggerZones$ Command | Execute$ TrigDraw
SVar:TrigDraw:DB$ Draw | Defined$ You | NumCards$ 1
Oracle:probe
`
	const list = `Name:List Probe
ManaCost:3 U
Types:Creature Human
PT:3/3
T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigTok | TriggerDescription$ probe
SVar:TrigTok:DB$ Token | TokenAmount$ 2 | TokenScript$ w_1_1_ally | TokenOwner$ You | RememberTokens$ True | SubAbility$ DBEffect
SVar:DBEffect:DB$ Effect | RememberObjects$ RememberedLKI | Triggers$ TrigExile | Duration$ Permanent | ForgetOnMoved$ FORGET | SubAbility$ DBEnd
SVar:DBEnd:DB$ Cleanup | ClearRemembered$ True
SVar:TrigExile:Mode$ ChangesZone | Origin$ Battlefield | Destination$ DEST | ValidCard$ Card.EffectSource | Execute$ ExileSet
SVar:ExileSet:DB$ ChangeZone | Defined$ Remembered | Origin$ Battlefield | Destination$ Exile
Oracle:probe
`
	fill := func(script string, kv ...string) string {
		return strings.NewReplacer(kv...).Replace(script)
	}
	refusal := func(script, name string) string {
		return strings.Join(lowerScript(t, name, script), "\n")
	}

	// the probes themselves lower
	for _, c := range []struct{ name, script string }{
		{"Count Probe", fill(count, "DURATION", "| Duration$ Permanent ", "PHASE", "Upkeep", "ONEOFF", "| OneOff$ True")},
		{"Pair Probe", fill(pair, "CMP", "EQ2", "VALID", "Creature.IsRemembered")},
		{"List Probe", fill(list, "FORGET", "Battlefield", "DEST", "Any")},
	} {
		if r := refusal(c.script, c.name); r != "" {
			t.Errorf("%s: refused: %s", c.name, r)
		}
	}

	// every variant outside a shape refuses by name
	for _, c := range []struct {
		name, script, want string
	}{
		{"Count Probe", fill(count, "DURATION", "", "PHASE", "Upkeep", "ONEOFF", "| OneOff$ True"), "Effect triggers without a permanent duration"},
		{"Count Probe", fill(count, "DURATION", "| Duration$ Permanent ", "PHASE", "End of Turn", "ONEOFF", "| OneOff$ True"), "the trigger is not a one-off Phase$ Upkeep"},
		{"Count Probe", fill(count, "DURATION", "| Duration$ Permanent ", "PHASE", "Upkeep", "ONEOFF", ""), "the trigger is not a one-off Phase$ Upkeep"},
		{"Pair Probe", fill(pair, "CMP", "EQ2", "VALID", "Creature.YouCtrl"), "beside a trigger that is not ValidCard$ ...IsRemembered"},
		{"Pair Probe", fill(pair, "CMP", "EQ1", "VALID", "Creature.IsRemembered"), "Defined$ Targeted over 2 target slots"},
		{"Count Probe", strings.Replace(fill(count, "DURATION", "| Duration$ Permanent ", "PHASE", "Upkeep", "ONEOFF", "| OneOff$ True"), " | RememberChanged$ True", "", 1), "the set is not one this resolution wrote"},
		{"Pair Probe", strings.Replace(fill(pair, "CMP", "EQ2", "VALID", "Creature.IsRemembered"), "Fight | Defined$ ParentTarget | ValidTgts$ Creature.OppCtrl | SubAbility$ DBDelay", "Pump | Defined$ ParentTarget | NumAtt$ 1 | SubAbility$ DBDelay", 1), "the chain does not declare exactly two target slots"},
		{"Pair Probe", strings.Replace(fill(pair, "CMP", "EQ2", "VALID", "Creature.IsRemembered"), "Triggers$ Dies |", "Triggers$ Dies | ValidTgts$ Creature |", 1), "the Effect has a ValidTgts$ of its own"},
		{"List Probe", strings.Replace(fill(list, "FORGET", "Battlefield", "DEST", "Any"), " | RememberTokens$ True", "", 1), "the set is not the tokens this resolution made"},
		{"List Probe", fill(list, "FORGET", "Exile", "DEST", "Any"), "no ForgetOnMoved$ Battlefield"},
		{"List Probe", fill(list, "FORGET", "Battlefield", "DEST", "Graveyard"), "the trigger is not a leave of Card.EffectSource"},
	} {
		if r := refusal(c.script, c.name); !strings.Contains(r, c.want) {
			t.Errorf("%s variant: refusals %q, want %q", c.name, r, c.want)
		}
	}
}
