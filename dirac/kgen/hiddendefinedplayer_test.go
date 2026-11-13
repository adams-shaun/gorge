package kgen

import (
	"strings"
	"testing"
)

// The DefinedPlayer$ spellings a hidden pick reads (lower_hiddenpick.go,
// lower_zone.go's plain graveyard pick, lower_definedmove.go's object-path
// inert read): gorge's effHiddenPick walks the fetch players the shared
// selector grammar names, and its object path reads no DefinedPlayer$ once
// the move's objects are already named. Each probe paraphrase carries the
// parameter shape; each out-of-shape edit must refuse by name.

const dpTriggerProbe = `Name:Defined Player Probe
ManaCost:2 U
Types:Creature
PT:2/2
T:Mode$ Phase | Phase$ Upkeep | TriggerZones$ Battlefield | Execute$ TrigBounce | TriggerDescription$ probe
SVar:TrigBounce:DB$ ChangeZone | Origin$ Battlefield | Destination$ Hand | ChangeType$ Permanent | DefinedPlayer$ TriggeredPlayer | ChangeNum$ 1 | Hidden$ True | Mandatory$ True | UnlessCost$ PayLife<2> | UnlessPayer$ TriggeredPlayer
Oracle:probe
`

const dpSpellProbe = `Name:Defined Each Probe
ManaCost:1 U
Types:Instant
A:SP$ ChangeZone | DefinedPlayer$ DP | Hidden$ True | Mandatory$ True | Origin$ Battlefield | Destination$ Hand | ChangeType$ Creature | SpellDescription$ probe
Oracle:probe
`

const dpGraveProbe = `Name:Defined Grave Probe
ManaCost:2 GU GU
Types:Creature
PT:2/4
A:AB$ Mill | Cost$ 2 GU GU | Defined$ You | NumCards$ 2 | SubAbility$ DBReturn | SpellDescription$ probe
SVar:DBReturn:DB$ ChangeZone | Origin$ Graveyard | Destination$ Hand | DefinedPlayer$ You | Chooser$ DPCHOOSER | Hidden$ True | Mandatory$ True | ChangeType$ Card.nonLand+YouOwn | AILogic$ WorstCard
Oracle:probe
`

const dpNamedProbe = `Name:Defined Named Probe
ManaCost:1 W B
Types:Legendary Enchantment Creature God
PT:5/4
T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Creature.YouOwn+Other | TriggerZones$ Battlefield | Execute$ TrigReturn | TriggerDescription$ probe
SVar:TrigReturn:DB$ ChangeZone | Defined$ TriggeredNewCardLKICopy | Origin$ Graveyard | Destination$ Hand | UnlessCost$ PayLife<3> | UnlessPayer$ Targeted | ValidTgts$ Opponent | IsCurse$ True | Hidden$ True | DefinedPlayer$ You
Oracle:probe
`

func dpScript(base, dp, chooser string) string {
	s := strings.Replace(base, "DPCHOOSER", chooser, 1)
	return strings.Replace(s, "DefinedPlayer$ DP", "DefinedPlayer$ "+dp, 1)
}

func dpOps(t *testing.T, name, script string) []string {
	t.Helper()
	c := lowerScriptCard(t, name, script)
	for _, rf := range c.Refusals {
		t.Errorf("%s refused: %s", name, rf.Reason)
	}
	var ops []string
	for _, a := range c.Abilities {
		for i := range a.Prog {
			ops = append(ops, a.Prog[i].Op+"("+a.Prog[i].Obj.String()+","+a.Prog[i].CS+")")
		}
	}
	return ops
}

func TestHiddenDefinedPlayerLowers(t *testing.T) {
	// the trigger's own-zone pick (Blood Clock, Umbilicus, Sunken Hope)
	ops := dpOps(t, "Defined Player Probe", dpScript(dpTriggerProbe, "TriggeredPlayer", "You"))
	if len(ops) == 0 || !strings.Contains(ops[len(ops)-1], "ChooseMove(TriggeredPlayer,") || !strings.Contains(ops[len(ops)-1], "OwnZone") {
		t.Errorf("trigger probe ops %q, want a ChooseMove over TriggeredPlayer's own zone", ops)
	}

	// each player's own-zone pick (Curfew), each opponent's (Sothera)
	for _, tc := range []struct{ dp, loop string }{{"Player", "RepeatEach(EachPlayer"}, {"Opponent", "RepeatEach(EachOpp"}} {
		ops := dpOps(t, "Defined Each Probe", dpScript(dpSpellProbe, tc.dp, "You"))
		joined := strings.Join(ops, ";")
		if !strings.Contains(joined, tc.loop) || !strings.Contains(joined, "ChooseMove(Each,") || !strings.Contains(joined, "OwnZone") {
			t.Errorf("%s probe ops %q, want the %s loop over each picker's own zone", tc.dp, ops, tc.loop)
		}
	}

	// the graveyard pick's chooser (Tasigur)
	ops = dpOps(t, "Defined Grave Probe", dpScript(dpGraveProbe, "You", "Opponent"))
	if len(ops) == 0 || !strings.Contains(ops[len(ops)-1], "ChooseMove(You,") || !strings.Contains(ops[len(ops)-1], "ChooserOpp") {
		t.Errorf("grave probe ops %q, want a ChooseMove over the controller's graveyard answered by an opponent", ops)
	}

	// the object path reads no DefinedPlayer$ (Athreos)
	dpOps(t, "Defined Named Probe", dpScript(dpNamedProbe, "You", "You"))

	// each out-of-shape variant refuses by name
	if rs := lowerScript(t, "Defined Player Probe", strings.Replace(dpTriggerProbe, "DefinedPlayer$ TriggeredPlayer", "DefinedPlayer$ Chooser", 1)); !anyRefusal(rs, "DefinedPlayer$") {
		t.Errorf("Chooser spelling: refusals %q, want one naming it", rs)
	}
	if rs := lowerScript(t, "Defined Each Probe", strings.Replace(dpSpellProbe, "| DefinedPlayer$ Player", "", 1)); !anyRefusal(rs, "DefinedPlayer$ Player inside a RepeatEach") && !anyRefusal(rs, "DefinedPlayer$") {
		t.Errorf("no DefinedPlayer$: refusals %q, want the game-wide pool refusal", rs)
	}
	if rs := lowerScript(t, "Defined Grave Probe", dpScript(dpGraveProbe, "You", "Targeted")); !anyRefusal(rs, "Chooser$") {
		t.Errorf("Chooser$ Targeted: refusals %q, want one naming it", rs)
	}
	if rs := lowerScript(t, "Defined Named Probe", strings.Replace(dpNamedProbe, "Origin$ Graveyard", "Origin$ Library", 1)); !anyRefusal(rs, "DefinedPlayer") {
		t.Errorf("library origin beside DefinedPlayer$: refusals %q, want it refused", rs)
	}
}
