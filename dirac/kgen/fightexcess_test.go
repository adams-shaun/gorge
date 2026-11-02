package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// kfxLowered lowers a synthetic sorcery whose root ability is svars["R"] and
// returns its refusal reasons and each emitted instruction's String().
func kfxLowered(t *testing.T, svars map[string]string) (refusals, prog []string) {
	t.Helper()
	r := cards.ResolveSVar(svars, "R")
	if r == nil {
		t.Fatalf("root %q did not resolve", svars["R"])
	}
	face := &cards.Face{Name: "Synthetic", ManaCost: "R G", Types: []string{"Sorcery"}, SVars: svars, Abilities: []*cards.SA{r}}
	card := &cards.Card{Faces: []*cards.Face{face}}
	p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lc := lowerCard(p, "Synthetic", card, false)
	for _, rf := range lc.Refusals {
		refusals = append(refusals, rf.Reason)
	}
	for _, a := range lc.Abilities {
		for _, in := range a.Prog {
			prog = append(prog, in.String())
		}
	}
	return
}

func kfxHas(lines []string, want string) bool {
	for _, l := range lines {
		if strings.Contains(l, want) {
			return true
		}
	}
	return false
}

// TestFightexcessLowers: Fight ExcessSVar$ is lowered for exactly Rhino's
// Rampage's shape and refused for every other spelling; the Effect
// UnspentMana Red is lowered for a spell and refused for every other shape.
func TestFightexcessLowers(t *testing.T) {
	rhino := func(cond string) map[string]string {
		return map[string]string{
			"R":           "SP$ Pump | Cost$ R G | ValidTgts$ Creature.YouCtrl | NumAtt$ +1 | SubAbility$ DBFight",
			"DBFight":     "DB$ Fight | Defined$ ParentTarget | ValidTgts$ Creature.OppCtrl | ExcessSVar$ Excess | ExcessSVarCondition$ " + cond + " | SubAbility$ DBImm",
			"DBImm":       "DB$ ImmediateTrigger | ConditionCheckSVar$ Excess | Execute$ TrigDestroy",
			"TrigDestroy": "DB$ Destroy | ValidTgts$ Artifact.nonCreature+cmcLE3 | TargetMin$ 0 | TargetMax$ 1",
		}
	}
	t.Run("rhino shape lowers", func(t *testing.T) {
		rs, prog := kfxLowered(t, rhino("Creature.targetedBy+OppCtrl"))
		if len(rs) != 0 {
			t.Fatalf("refused: %v", rs)
		}
		fi := -1
		for i, l := range prog {
			if strings.HasPrefix(l, "Fight") {
				fi = i
			}
		}
		if fi < 0 || !strings.Contains(prog[fi], "with=Target1|TrackExcess") {
			t.Fatalf("no tracked Fight in %v", prog)
		}
		if !kfxHas(prog[fi+1:], "SkipIf") || !kfxHas(prog[fi+1:], "ReflexiveTrigger") {
			t.Errorf("no SkipIf/ReflexiveTrigger after the Fight: %v", prog)
		}
	})
	t.Run("other condition refuses", func(t *testing.T) {
		rs, _ := kfxLowered(t, rhino("Card.targetedBy"))
		if !kfxHas(rs, "Fight ExcessSVar$ shape") {
			t.Fatalf("want a Fight ExcessSVar$ shape refusal, got %v", rs)
		}
	})
	t.Run("other target filter refuses", func(t *testing.T) {
		sv := rhino("Creature.targetedBy+OppCtrl")
		sv["DBFight"] = strings.Replace(sv["DBFight"], "ValidTgts$ Creature.OppCtrl", "ValidTgts$ Creature.Other", 1)
		rs, _ := kfxLowered(t, sv)
		if !kfxHas(rs, "Fight ExcessSVar$ shape") {
			t.Fatalf("want a Fight ExcessSVar$ shape refusal, got %v", rs)
		}
	})

	unspent := func(root, static string) map[string]string {
		return map[string]string{"R": root, "Unspent": static}
	}
	const red = "Mode$ UnspentMana | ValidPlayer$ You | ManaType$ Red"
	t.Run("unspent red lowers", func(t *testing.T) {
		rs, prog := kfxLowered(t, unspent("SP$ Effect | Cost$ R G | StaticAbilities$ Unspent", red))
		if len(rs) != 0 {
			t.Fatalf("refused: %v", rs)
		}
		if !kfxHas(prog, "AddMana obj=You A=0 B=3 CS=R") {
			t.Fatalf("no AddMana obj=You A=0 B=3 CS=R in %v", prog)
		}
	})
	for _, c := range []struct{ name, root, static string }{
		{"green", "SP$ Effect | Cost$ R G | StaticAbilities$ Unspent", "Mode$ UnspentMana | ValidPlayer$ You | ManaType$ Green"},
		{"permanent duration", "SP$ Effect | Cost$ R G | Duration$ Permanent | StaticAbilities$ Unspent", red},
		{"activated source", "AB$ Effect | Cost$ T | StaticAbilities$ Unspent", red},
		{"opponent", "SP$ Effect | Cost$ R G | StaticAbilities$ Unspent", "Mode$ UnspentMana | ValidPlayer$ Opponent | ManaType$ Red"},
	} {
		t.Run("unspent "+c.name+" refuses", func(t *testing.T) {
			rs, prog := kfxLowered(t, unspent(c.root, c.static))
			if len(rs) == 0 {
				t.Fatalf("lowered: %v", prog)
			}
			if kfxHas(prog, "B=3") {
				t.Errorf("emitted the UnspentMana mark: %v", prog)
			}
		})
	}
}
