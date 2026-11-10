package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestLowerEffectReplStagger: Lightning, Army of One's Stagger (an Effect
// damage replacement, Duration$ UntilYourNextTurn, doubling the damage the
// remembered damaged player and their permanents take) lowers whole to one
// OpExt ExtDmgMark over the trigger's damaged player. Each out-of-shape
// variant still refuses by name.
func TestLowerEffectReplStagger(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Lightning, Army of One")
	if !ok || card == nil {
		t.Fatal("Lightning, Army of One not in the corpus")
	}
	p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
	lc := lowerCard(p, card.Faces[0].Name, card, false)
	if got := keclReason(lc.Refusals); got != "" {
		t.Fatalf("Lightning: refusals %q, want none", got)
	}
	marks := 0
	for _, ab := range lc.Abilities {
		for _, in := range ab.Prog {
			if in.Op == "DmgMark" {
				marks++
				if in.Obj.Kind != rTriggeredPlayer {
					t.Errorf("DmgMark Obj kind %v, want the trigger's player", in.Obj.Kind)
				}
			}
		}
	}
	if marks != 1 {
		t.Fatalf("Lightning: %d DmgMark ops, want 1", marks)
	}

	const base = "Name:Exile Bf Probe\nManaCost:1 R W\nTypes:Legendary Creature Human Soldier\nPT:3/2\n" +
		"T:Mode$ DamageDone | ValidSource$ Card.Self | ValidTarget$ Player | Execute$ TrigEffect | CombatDamage$ True | TriggerDescription$ probe\n" +
		"SVar:TrigEffect:DB$ Effect | ReplacementEffects$ Stagger | RememberObjects$ TriggeredTarget | Duration$ UntilYourNextTurn\n" +
		"SVar:Stagger:Event$ DamageDone | ActiveZones$ Command | ValidSource$ Card,Emblem | ValidTarget$ Player.IsRemembered,Permanent.ControlledBy Player.IsRemembered | ReplaceWith$ DmgTwice | Description$ probe\n" +
		"SVar:DmgTwice:DB$ ReplaceEffect | VarName$ DamageAmount | VarValue$ StaggerX\n" +
		"SVar:StaggerX:ReplaceCount$DamageAmount/Twice\n"
	if r := exileBfSynth(t, base); len(r) != 0 {
		t.Fatalf("the probe script itself refuses: %q", keclReason(r))
	}
	for _, c := range []struct{ name, from, to, want string }{
		{"thrice", "DamageAmount/Twice", "DamageAmount/Thrice", "Effect DamageDone doubling VarValue$"},
		{"var name", "VarName$ DamageAmount", "VarName$ Other", "Effect DamageDone doubling body"},
		{"body extra key", "VarName$ DamageAmount |", "VarName$ DamageAmount | Foo$ Bar |", "Effect DamageDone doubling body"},
		{"player only", "ValidTarget$ Player.IsRemembered,Permanent.ControlledBy Player.IsRemembered", "ValidTarget$ Player.IsRemembered", "Effect DamageDone doubling ValidTarget$"},
		{"creature source", "ValidSource$ Card,Emblem", "ValidSource$ Creature", "Effect DamageDone doubling ValidSource$"},
		{"combat only", "ActiveZones$ Command |", "ActiveZones$ Command | IsCombat$ True |", "Effect DamageDone doubling unread"},
		{"zone", "ActiveZones$ Command", "ActiveZones$ Battlefield", "Effect DamageDone doubling ActiveZones$"},
		{"remember self", "RememberObjects$ TriggeredTarget", "RememberObjects$ Self", "Effect DamageDone doubling RememberObjects"},
		{"other duration", "Duration$ UntilYourNextTurn", "Duration$ UntilHostLeavesPlay", "Effect replacement Duration$"},
	} {
		src := strings.Replace(base, c.from, c.to, 1)
		if src == base {
			t.Fatalf("%s: variant did not change the script", c.name)
		}
		if got := keclReason(exileBfSynth(t, src)); !strings.Contains(got, c.want) {
			t.Errorf("%s: refusals %q, want %q", c.name, got, c.want)
		}
	}
}
