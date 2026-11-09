package kgen

import (
	"fmt"
	"strings"
	"testing"
)

// effectProg returns every instruction of every ability of the card.
func effectProg(c *lCard) []lInstr {
	var out []lInstr
	for _, a := range c.Abilities {
		out = append(out, a.Prog...)
	}
	return out
}

// TestEffectLongDurations: an Effect whose grant outlives the turn lowers on
// the kernel's existing records. A may-play of the chosen card to the end of
// the controller's next turn is the per-card permission with that lifetime
// (End-Blaze Epiphany); "loses all abilities while the source stays" over the
// one targeted permanent is a host-bound LoseAbilities override (The Wondrous
// Wasp). Every neighbouring spelling keeps its named refusal.
func TestEffectLongDurations(t *testing.T) {
	cs := lowerNamed(t, []string{"End-Blaze Epiphany", "The Wondrous Wasp"})
	if len(cs) != 2 {
		t.Fatalf("lowered %d cards, want 2", len(cs))
	}
	for _, c := range cs {
		for _, r := range c.Refusals {
			t.Errorf("%s: refused: %s", c.Name, r.Reason)
		}
		prog := effectProg(c)
		switch c.Name {
		case "End-Blaze Epiphany":
			found := false
			for _, in := range prog {
				if in.Op == "MayPlay" && in.Obj.Kind == rChosen && in.CS == "EndOfYourNextTurn" {
					found = true
				}
			}
			if !found {
				t.Errorf("%s: no MayPlay of the chosen card to the end of the next turn in %v", c.Name, prog)
			}
		case "The Wondrous Wasp":
			found := false
			for _, in := range prog {
				if in.Op == "Override" && in.Obj.Kind == rTarget && in.CS == "WhileHost" {
					o := c.overrides[in.C]
					found = o.LoseAbilities && o.WhileHost
				}
			}
			if !found {
				t.Errorf("%s: no host-bound LoseAbilities override of the target in %v", c.Name, prog)
			}
		}
	}

	const chosen = "Name:Chosen Probe\nManaCost:1 R\nTypes:Instant\n" +
		"A:SP$ Dig | DigNum$ 2 | ChangeNum$ All | DestinationZone$ Exile | RememberChanged$ True | SubAbility$ Pick\n" +
		"SVar:Pick:DB$ ChooseCard | Amount$ 1 | Mandatory$ True | Choices$ Card.IsRemembered | ChoiceZone$ Exile | SubAbility$ Grant\n" +
		"SVar:Grant:DB$ Effect | StaticAbilities$ Play | %s | SubAbility$ Clean\n" +
		"SVar:Play:Mode$ Continuous | MayPlay$ True | Affected$ %s | AffectedZone$ Exile\n" +
		"SVar:Clean:DB$ Cleanup | ClearChosenCard$ True | ClearRemembered$ True\n"
	const lose = "Name:Lose Probe\nManaCost:1 U\nTypes:Creature Human\nPT:2/1\n" +
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ Go\n" +
		"SVar:Go:DB$ Tap | ValidTgts$ Creature | TargetMin$ 0 | TargetMax$ 1 | SubAbility$ Fx\n" +
		"SVar:Fx:DB$ Effect | StaticAbilities$ Lose | RememberObjects$ Targeted | %s | Duration$ %s\n" +
		"SVar:Lose:Mode$ Continuous | Affected$ Card.IsRemembered | RemoveAllAbilities$ True | AffectedZone$ %s\n"
	for _, tc := range []struct {
		name, src, want string // want "": lowers clean; else a refusal substring
	}{
		{"chosen to the end of the next turn", fmt.Sprintf(chosen, "Duration$ UntilTheEndOfYourNextTurn", "Card.ChosenCard"), ""},
		{"chosen, another affected word", fmt.Sprintf(chosen, "Duration$ UntilTheEndOfYourNextTurn", "Card.nonLand+ChosenCard"), "Effect static"},
		{"chosen, next turn start", fmt.Sprintf(chosen, "Duration$ UntilYourNextTurn", "Card.ChosenCard"), "Effect static"},
		{"lose abilities while the host stays", fmt.Sprintf(lose, "ForgetOnMoved$ Battlefield", "AsLongAsInPlay", "Battlefield"), ""},
		{"lose abilities without the forget rider", fmt.Sprintf(lose, "ConditionDefined$ Targeted", "AsLongAsInPlay", "Battlefield"), "Effect continuous shape"},
		{"lose abilities, another duration", fmt.Sprintf(lose, "ForgetOnMoved$ Battlefield", "Permanent", "Battlefield"), "Effect"},
		{"lose abilities with an extra static key", strings.Replace(fmt.Sprintf(lose, "ForgetOnMoved$ Battlefield", "AsLongAsInPlay", "Battlefield"),
			"RemoveAllAbilities$ True", "RemoveAllAbilities$ True | AddPower$ -3", 1), "unread"},
		{"lose abilities, another zone", fmt.Sprintf(lose, "ForgetOnMoved$ Battlefield", "AsLongAsInPlay", "Graveyard"), "Effect continuous shape"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := keclReason(keclSynth(t, "Probe", tc.src))
			if tc.want == "" && got != "" {
				t.Errorf("refused: %s", got)
			}
			if tc.want != "" && !strings.Contains(got, tc.want) {
				t.Errorf("refusal %q, want it to contain %q", got, tc.want)
			}
		})
	}
}
