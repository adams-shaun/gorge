package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestLowerEffectReplUntilUntaps: Amber Prison's Effect (Duration$
// UntilUntaps: "for as long as CARDNAME remains tapped") lowers to OpSkipUntap
// C kop.SkipUntapLockUntapStep, with either ExileOnMoved$ Battlefield or
// ForgetOnMoved$ Battlefield, and IsCurse$ consumed. Each out-of-shape variant
// still refuses by name.
func TestLowerEffectReplUntilUntaps(t *testing.T) {
	const base = "Name:Exile Bf Probe\nManaCost:4\nTypes:Artifact\n" +
		"A:AB$ Tap | Cost$ 4 T | ValidTgts$ Artifact,Creature,Land | SubAbility$ DBLock | SpellDescription$ probe\n" +
		"SVar:DBLock:DB$ Effect | Duration$ UntilUntaps | ExileOnMoved$ Battlefield | RememberObjects$ Targeted | ReplacementEffects$ NoUntap\n" +
		"SVar:NoUntap:Event$ Untap | Layer$ CantHappen | ValidStepTurnToController$ You | ValidCard$ Card.IsRemembered\n"
	for _, c := range []struct{ name, from, to string }{
		{"plain", "", ""},
		{"curse flag", "DB$ Effect | ", "DB$ Effect | IsCurse$ True | "},
		{"forget on moved", "ExileOnMoved$ Battlefield", "ForgetOnMoved$ Battlefield"},
	} {
		src := strings.Replace(base, c.from, c.to, 1)
		if (src == base) != (c.from == "") {
			t.Fatalf("%s: variant did not change the script", c.name)
		}
		lc := untapLockProbe(t, src)
		if got := keclReason(lc.Refusals); got != "" {
			t.Errorf("%s: refusals %q, want none", c.name, got)
		}
		if got := untapLockOps(lc); len(got) != 1 || got[0] != kop.SkipUntapLockUntapStep {
			t.Errorf("%s: SkipUntap C %v, want [%d]", c.name, got, kop.SkipUntapLockUntapStep)
		}
	}
	for _, c := range []struct{ name, from, to, want string }{
		{"no step scope", "ValidStepTurnToController$ You | ", "", "Duration$ UntilUntaps without ValidStepTurnToController$ You"},
		{"exile elsewhere", "ExileOnMoved$ Battlefield", "ExileOnMoved$ Graveyard", "Effect UntilUntaps untap ExileOnMoved$"},
		{"neither", " | ExileOnMoved$ Battlefield", "", "Effect UntilUntaps untap ForgetOnMoved$"},
		{"both", "ExileOnMoved$ Battlefield", "ExileOnMoved$ Battlefield | ForgetOnMoved$ Battlefield", "Effect UntilUntaps untap ExileOnMoved$"},
		{"many targets", "ValidTgts$ Artifact,Creature,Land", "ValidTgts$ Artifact,Creature,Land | TargetMax$ 2", "untap target slot"},
		{"remember valid", "RememberObjects$ Targeted", "RememberObjects$ Valid Artifact.Other", "Effect UntilUntaps untap RememberObjects"},
	} {
		src := strings.Replace(base, c.from, c.to, 1)
		if src == base {
			t.Fatalf("%s: variant did not change the script", c.name)
		}
		if got := keclReason(untapLockProbe(t, src).Refusals); !strings.Contains(got, c.want) {
			t.Errorf("%s: refusals %q, want %q", c.name, got, c.want)
		}
	}
}
