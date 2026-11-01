package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// kremw4Lowered lowers a synthetic sorcery whose root is SVar R of svars and
// returns its refusal reasons.
func kremw4Lowered(t *testing.T, svars map[string]string) []string {
	t.Helper()
	r := cards.ResolveSVar(svars, "R")
	if r == nil {
		t.Fatalf("root %q did not resolve", svars["R"])
	}
	face := &cards.Face{Name: "Synthetic", ManaCost: "B", Types: []string{"Sorcery"}, SVars: svars, Abilities: []*cards.SA{r}}
	card := &cards.Card{Faces: []*cards.Face{face}}
	p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	var out []string
	for _, rf := range lowerCard(p, "Synthetic", card, false).Refusals {
		out = append(out, rf.Reason)
	}
	return out
}

// TestReflexiveRememberRefusals: the shapes kgen does not read
// exactly refuse with a named reason; Curse of the Werefox's and
// Graceful Takedown's shapes lower.
func TestReflexiveRememberRefusals(t *testing.T) {
	fight := "DB$ Fight | Defined$ DelayTriggerRemembered | ValidTgts$ Creature.YouDontCtrl | TargetMin$ 0 | TargetMax$ 1"
	reflex := "DB$ ImmediateTrigger | RememberObjects$ Targeted | Execute$ TrigFight"
	for _, c := range []struct {
		name  string
		svars map[string]string
		want  string
	}{
		{"Pump RememberObjects Imprinted", map[string]string{
			"R": "SP$ Pump | ValidTgts$ Creature | NumAtt$ +1 | RememberObjects$ Imprinted"}, `Pump RememberObjects$ "Imprinted"`},
		{"Back for More", map[string]string{
			"R":         "SP$ ChangeZone | Origin$ Graveyard | Destination$ Battlefield | ValidTgts$ Creature.YouOwn | SubAbility$ DBImm",
			"DBImm":     reflex,
			"TrigFight": fight}, "ImmediateTrigger RememberObjects$ Targeted of a non-battlefield target"},
		{"Attach RememberAttached False", map[string]string{
			"R": "SP$ Attach | ValidTgts$ Creature.YouCtrl | RememberAttached$ False"}, `Attach RememberAttached$ "False"`},
		{"Fight DelayTriggerRemembered outside a capturing reflexive", map[string]string{
			"R": "SP$ Fight | Defined$ DelayTriggerRemembered | ValidTgts$ Creature.YouDontCtrl | TargetMin$ 0 | TargetMax$ 1"}, "DelayTriggerRemembered"},
	} {
		t.Run(c.name, func(t *testing.T) {
			kdefWant(t, kremw4Lowered(t, c.svars), c.want)
		})
	}
	t.Run("Graceful Takedown lowers", func(t *testing.T) {
		got := kremw4Lowered(t, map[string]string{
			"R":   "SP$ Pump | ValidTgts$ Creature.YouCtrl+enchanted | TargetMin$ 0 | TargetMax$ X | RememberObjects$ ThisTargetedCard | SubAbility$ DBP",
			"DBP": "DB$ Pump | ValidTgts$ Creature.YouCtrl | TargetUnique$ True | TargetMin$ 0 | TargetMax$ 1 | RememberObjects$ ThisTargetedCard | SubAbility$ DBE",
			"DBE": "DB$ EachDamage | ValidTgts$ Creature.YouDontCtrl | DefinedDamagers$ Remembered | NumDmg$ Count$CardPower | SubAbility$ DBC",
			"DBC": "DB$ Cleanup | ClearRemembered$ True",
			"X":   "Count$Valid Creature.YouCtrl+enchanted"})
		if len(got) != 0 {
			t.Fatalf("refused: %q", got)
		}
	})
	t.Run("Curse of the Werefox lowers", func(t *testing.T) {
		got := kremw4Lowered(t, map[string]string{
			"R":         "SP$ Pump | ValidTgts$ Creature.YouCtrl | NumAtt$ +1 | SubAbility$ DBImm",
			"DBImm":     reflex,
			"TrigFight": fight})
		if len(got) != 0 {
			t.Fatalf("refused: %q", got)
		}
	})
}
