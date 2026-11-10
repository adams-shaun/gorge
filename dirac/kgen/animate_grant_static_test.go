package kgen

import (
	"strings"
	"testing"
)

// TestLowerAnimateGrantStatic: an Animate whose single staticAbilities$ body
// is a Continuous static over other permanents lowers that body as an ordinary
// source static gated by the override record the animation applies
// (kop.Static.OvGate): Tenth District Hero's indestructible anthem and The
// Irencrag's equipped pump. With the Animate's Name$, its Equip:N keyword
// (with or without RemoveAllAbilities$) and OptionQuestion$ read, both cards
// lower whole; a body that is not an anthem, keyword grant or equipped pump
// still refuses by name.
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
				"SVar:ChooseCT:DB$ ChooseType | Type$ Creature | Defined$ You | SpellDescription$ probe pick a type\n" +
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
		{
			name: "chooser Name$ is not a rename",
			body: "A:AB$ Animate | Cost$ 1 | Defined$ Self | Name$ ChosenName | Duration$ Permanent | SpellDescription$ test\n",
			want: `Animate Name$ "ChosenName"`,
		},
		{
			name: "literal Name$ renames",
			body: "A:AB$ Animate | Cost$ 1 | Defined$ Self | Name$ Probe Renamed | Types$ Legendary | Duration$ Permanent | SpellDescription$ test\n",
		},
		{
			name: "Fortify keyword still refuses",
			body: "A:AB$ Animate | Cost$ 1 | Defined$ Self | Keywords$ Fortify:2 | Duration$ Permanent | SpellDescription$ test\n",
			want: `keyword "Fortify:2"`,
		},
		{
			name: "Equip beside a plain keyword",
			body: "A:AB$ Animate | Cost$ 1 | Defined$ Self | RemoveAllAbilities$ True | Keywords$ Equip:3 & Vigilance | Duration$ Permanent | SpellDescription$ test\n",
		},
		{
			name: "Equip without the loss of abilities",
			body: "A:AB$ Animate | Cost$ 1 | Defined$ Self | Keywords$ Equip:2 | Duration$ Permanent | SpellDescription$ test\n",
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

	// The first two corpus cards lower whole: the granted static, the rename,
	// the granted equip ability.
	all, _ := keclBuild(t, []string{"Tenth District Hero", "The Irencrag"})
	for _, tc := range []struct{ name string }{
		{"Tenth District Hero"},
		{"The Irencrag"},
	} {
		i := keclIndex(t, all, tc.name)
		if reason := keclReason(lowerRefusals(all[i])); reason != "" {
			t.Errorf("%s: refused: %s", tc.name, reason)
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
			if !strings.Contains(ir, "Activated Equip") || !strings.Contains(ir, "grant=") || !strings.Contains(ir, "loseabilities") {
				t.Errorf("The Irencrag: lowered IR lacks the override-granted Equip beside loseabilities:\n%s", ir)
			}
		}
	}

	// Domesticated Watercourse lowers whole too: its self-granted Equip:2
	// beside no RemoveAllAbilities$ (the kernel hosts it on the animated
	// permanent itself).
	wc, _ := keclBuild(t, []string{"Domesticated Watercourse"})
	wi := keclIndex(t, wc, "Domesticated Watercourse")
	if reason := keclReason(lowerRefusals(wc[wi])); reason != "" {
		t.Errorf("Domesticated Watercourse: refused: %s", reason)
	}
	var wb strings.Builder
	dumpLIR(&wb, wc[wi])
	if ir := wb.String(); !strings.Contains(ir, "Activated Equip") || !strings.Contains(ir, "grant=") {
		t.Errorf("Domesticated Watercourse: lowered IR lacks the override-granted Equip:\n%s", ir)
	}
}
