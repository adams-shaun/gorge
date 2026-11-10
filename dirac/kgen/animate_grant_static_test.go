package kgen

import (
	"strings"
	"testing"
)

// TestLowerAnimateGrantStatic: an Animate whose single staticAbilities$ body
// is a Continuous static over other permanents lowers that body as an ordinary
// source static gated by the override record the animation applies
// (kop.Static.OvGate): Tenth District Hero's indestructible anthem and The
// Irencrag's equipped pump. Each card then names its next refusal from another
// class (the Animate's Name$ unread, its parameterised Equip keyword); a body
// that is not an anthem, keyword grant or equipped pump still refuses by name.
func TestLowerAnimateGrantStatic(t *testing.T) {
	const base = "Name:Animate Grant Probe\nManaCost:1\nTypes:Creature Human\nPT:1/1\n"
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "keyword anthem",
			body: "A:AB$ Animate | Cost$ 1 | Duration$ Permanent | staticAbilities$ Static | SpellDescription$ test\n" +
				"SVar:Static:Mode$ Continuous | Affected$ Creature.Other+YouCtrl | AddKeyword$ Indestructible | Description$ test\n",
		},
		{
			name: "equipped pump",
			body: "A:AB$ Animate | Cost$ 1 | Defined$ Self | Duration$ Permanent | staticAbilities$ Static | SpellDescription$ test\n" +
				"SVar:Static:Mode$ Continuous | AffectedDefined$ Equipped | Affected$ Creature | AddPower$ 3 | AddToughness$ 3 | Description$ test\n",
		},
		{
			name: "duration not permanent",
			body: "A:AB$ Animate | Cost$ 1 | Duration$ UntilYourNextTurn | staticAbilities$ Static | SpellDescription$ test\n" +
				"SVar:Static:Mode$ Continuous | Affected$ Creature.Other+YouCtrl | AddKeyword$ Indestructible | Description$ test\n",
			want: "beside a duration other than Permanent on Self",
		},
		{
			name: "no anthem or pump",
			body: "A:AB$ Animate | Cost$ 1 | Duration$ Permanent | staticAbilities$ Static | SpellDescription$ test\n" +
				"SVar:Static:Mode$ Continuous | Affected$ Creature.Other+YouCtrl | Description$ test\n",
			want: "shape",
		},
		{
			name: "player hexproof reads no gate",
			body: "A:AB$ Animate | Cost$ 1 | Duration$ Permanent | staticAbilities$ Static | SpellDescription$ test\n" +
				"SVar:Static:Mode$ Continuous | Affected$ You | AddKeyword$ Hexproof | Description$ test\n",
			want: "ungated static kind PlayerHexproof",
		},
		{
			name: "stack keyword reads no gate",
			body: "A:AB$ Animate | Cost$ 1 | Duration$ Permanent | staticAbilities$ Static | SpellDescription$ test\n" +
				"SVar:Static:Mode$ Continuous | Affected$ Card.wasCastByYou | AffectedZone$ Stack | AddKeyword$ Cascade | Description$ test\n",
			want: "ungated static kind StackKw",
		},
		{
			name: "equipped pump granting an ability reads no gate",
			body: "A:AB$ Animate | Cost$ 1 | Defined$ Self | Duration$ Permanent | staticAbilities$ Static | SpellDescription$ test\n" +
				"SVar:Static:Mode$ Continuous | AffectedDefined$ Equipped | Affected$ Creature | AddPower$ 1 | AddAbility$ ABP | Description$ test\n" +
				"SVar:ABP:AB$ Pump | Cost$ T | Defined$ Self | NumAtt$ +1 | SpellDescription$ test\n",
			want: "grants an ability the kernel does not gate",
		},
		{
			name: "anthem granting a trigger reads no gate",
			body: "A:AB$ Animate | Cost$ 1 | Duration$ Permanent | staticAbilities$ Static | SpellDescription$ test\n" +
				"SVar:Static:Mode$ Continuous | Affected$ Creature.Other+YouCtrl | AddKeyword$ Haste | AddTrigger$ TrigP | Description$ test\n" +
				"SVar:TrigP:Mode$ Attacks | ValidCard$ Card.Self | Execute$ ProbePump | TriggerDescription$ probe grant trigger\n" +
				"SVar:ProbePump:DB$ Pump | Defined$ Self | NumAtt$ +1 | NumDef$ +0\n",
			want: "grants an ability the kernel does not gate",
		},
		{
			name: "anthem granting a subtype reads no gate",
			body: "A:AB$ Animate | Cost$ 1 | Duration$ Permanent | staticAbilities$ Static | SpellDescription$ test\n" +
				"SVar:Static:Mode$ Continuous | Affected$ Creature.Other+YouCtrl | AddKeyword$ Haste | AddType$ Soldier | Description$ test\n",
			want: "grants a subtype the kernel does not gate",
		},
		{
			name: "anthem granting the chosen type reads no gate",
			body: "K:ETBReplacement:Copy:ChooseCT\n" +
				"SVar:ChooseCT:DB$ ChooseType | Defined$ You | Type$ Creature | AILogic$ MostProminentInComputerDeck | SpellDescription$ test\n" +
				"A:AB$ Animate | Cost$ 1 | Duration$ Permanent | staticAbilities$ Static | SpellDescription$ test\n" +
				"SVar:Static:Mode$ Continuous | Affected$ Creature.Other+YouCtrl | AddPower$ 1 | AddType$ ChosenType | Description$ test\n",
			want: "grants a subtype the kernel does not gate",
		},
		{
			name: "comma list stays a permission shape",
			body: "A:AB$ Animate | Cost$ 1 | Defined$ Self | Duration$ Permanent | staticAbilities$ A,B | SpellDescription$ test\n" +
				"SVar:A:Mode$ Continuous | Affected$ Creature.Other+YouCtrl | AddKeyword$ Indestructible | Description$ test\n" +
				"SVar:B:Mode$ Continuous | Affected$ Creature.Other+YouCtrl | AddKeyword$ Vigilance | Description$ test\n",
			want: "shape",
		},
	} {
		got := keclReason(keclSynth(t, "Animate Grant Probe", base+tc.body))
		if tc.want == "" {
			if got != "" {
				t.Errorf("%s: refused: %s", tc.name, got)
			}
			continue
		}
		if !strings.Contains(got, tc.want) {
			t.Errorf("%s: refusals %q, want one containing %q", tc.name, got, tc.want)
		}
	}

	// Both corpus cards lower the granted static; their remaining refusals are
	// the next class, never the staticAbilities$ grant.
	all, _ := keclBuild(t, []string{"Tenth District Hero", "The Irencrag"})
	for _, tc := range []struct {
		name, next string
	}{
		{"Tenth District Hero", "unread params [Name]"},
		{"The Irencrag", `keyword "Equip:3"`},
	} {
		i := keclIndex(t, all, tc.name)
		reason := keclReason(lowerRefusals(all[i]))
		if strings.Contains(reason, "staticAbilities$") {
			t.Errorf("%s: the granted-static class still refuses: %s", tc.name, reason)
		}
		if !strings.Contains(reason, tc.next) {
			t.Errorf("%s: refusals %q, want the next class %q", tc.name, reason, tc.next)
		}
		var sb strings.Builder
		dumpLIR(&sb, all[i])
		ir := sb.String()
		if !strings.Contains(ir, "ovgate=1") {
			t.Errorf("%s: lowered IR lacks the source-scoped granted static:\n%s", tc.name, ir)
		}
		switch tc.name {
		case "Tenth District Hero":
			if !strings.Contains(ir, "kw=[Indestructible]") || !strings.Contains(ir, "grantstatics=1") {
				t.Errorf("Tenth District Hero: lowered IR lacks the gated indestructible anthem:\n%s", ir)
			}
		case "The Irencrag":
			if !strings.Contains(ir, "EquippedBy") || !strings.Contains(ir, "P=3 T=3") {
				t.Errorf("The Irencrag: lowered IR lacks the gated equipped pump:\n%s", ir)
			}
		}
	}
}
