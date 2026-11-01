package kgen

import (
	"strings"
	"testing"
)

// TestAttachChoicesLowering: the object-side Attach pick
// `DB$ Attach | Choices$ <filter> | Defined$ <dest>` (no Object$) lowers to a
// ChooseCard with the attach-pick code (CS AttachPick mandatory, AttachPickOpt
// for Optional$ True) followed by an AttachObj of the chosen permanent onto
// the destination. Unexpected Request's DelayedTrigger body (api Unattach)
// stays refused (on RememberAttached$ at the Attach node, before its api
// Unattach body), and the keys the pick does not read refuse by name.
func TestAttachChoicesLowering(t *testing.T) {
	want := map[string]string{
		"Zack Fair":                    "AttachPick",
		"Vow to Erebor":                "AttachPickOpt",
		"Winter Soldier, Icy Assassin": "AttachPickOpt",
	}
	names := []string{"Zack Fair", "Vow to Erebor", "Winter Soldier, Icy Assassin", "Unexpected Request"}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		if c.Name == "Unexpected Request" {
			// the Attach node's RememberAttached$ refuses before the delayed
			// body's api Unattach is reached
			if len(c.Refusals) == 0 {
				t.Errorf("Unexpected Request lowered, want it refused")
			}
			continue
		}
		if len(c.Refusals) != 0 {
			t.Errorf("%s: refused: %v", c.Name, c.Refusals)
			continue
		}
		seen++
		ok := false
		for _, ab := range c.Abilities {
			for i, in := range ab.Prog {
				if in.Op == "ChooseCard" && in.CS == want[c.Name] {
					ok = i+1 < len(ab.Prog) && ab.Prog[i+1].Op == "AttachObj" && ab.Prog[i+1].Obj2.Kind == rChosen
				}
			}
		}
		if !ok {
			t.Errorf("%s: no ChooseCard CS=%s followed by AttachObj of the chosen card", c.Name, want[c.Name])
		}
	}
	if seen != 3 {
		t.Errorf("%d of the 3 carriers lowered", seen)
	}

	const base = "Name:Attach Probe\nManaCost:1 W\nTypes:Sorcery\nA:SP$ Attach | ValidTgts$ Creature.YouCtrl | Choices$ Artifact.YouCtrl | Defined$ Targeted"
	cases := []struct{ name, extra, want string }{
		{"mandatory", "", ""},
		{"optional", " | Optional$ True", ""},
		{"optional false", " | Optional$ False", `Optional$ "False"`},
		{"remember attached", " | RememberAttached$ True", "RememberAttached$"},
		{"chooser", " | Chooser$ Opponent", "Chooser$"},
		{"choice zone", " | ChoiceZone$ Graveyard", "ChoiceZone$"},
		{"player choices", " | PlayerChoices$ Player", "PlayerChoices$"},
	}
	for _, c := range cases {
		got := keclReason(keclSynth(t, "Attach Probe", base+c.extra+"\n"))
		if c.want == "" {
			if got != "" {
				t.Errorf("%s: refused: %s", c.name, got)
			}
		} else if !strings.Contains(got, c.want) {
			t.Errorf("%s: refusals %q, want one containing %q", c.name, got, c.want)
		}
	}
	// a Defined$ the pick does not resolve refuses by name
	got := keclReason(keclSynth(t, "Attach Probe", strings.Replace(base, "Defined$ Targeted", "Defined$ You", 1)+"\n"))
	if !strings.Contains(got, `Defined$ "You"`) {
		t.Errorf("Defined You: refusals %q", got)
	}
}
