package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// kdefRefusals lowers a synthetic sorcery whose root is `root` with the
// SubAbility `sub` and returns its refusal reasons.
func kdefRefusals(t *testing.T, root, sub string) []string {
	t.Helper()
	svars := map[string]string{"DBX": sub}
	r := cards.ResolveSVar(map[string]string{"R": root}, "R")
	if r == nil {
		t.Fatalf("root %q did not resolve", root)
	}
	r.Params["SubAbility"] = "DBX"
	r.Sub = cards.ResolveSVar(svars, "DBX")
	if r.Sub == nil {
		t.Fatalf("sub %q did not resolve", sub)
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

func kdefWant(t *testing.T, got []string, want string) {
	t.Helper()
	for _, g := range got {
		if strings.Contains(g, want) {
			return
		}
	}
	t.Errorf("refusals %q lack %q", got, want)
}

// TestDefinedRefusals: every shape of the grammar sweep the
// lowering does not read exactly is refused with a named reason.
func TestDefinedRefusals(t *testing.T) {
	sac := "SP$ Draw | Cost$ B Sac<1/Artifact>"
	t.Run("sacrificed ok", func(t *testing.T) {
		got := kdefRefusals(t, sac, "DB$ Draw | ConditionDefined$ Sacrificed | ConditionPresent$ Card.Legendary")
		if len(got) != 0 {
			t.Errorf("refused: %q", got)
		}
	})
	t.Run("sacrificed state filter", func(t *testing.T) {
		for _, f := range []string{"Card.modified", "Card.YouCtrl", "Card.tapped", "Card.powerGE3", "Card.counters_GE1_P1P1"} {
			got := kdefRefusals(t, sac, "DB$ Draw | ConditionDefined$ Sacrificed | ConditionPresent$ "+f)
			kdefWant(t, got, "reads state, not printed characteristics")
		}
	})
	t.Run("sacrificed without a sacrifice cost", func(t *testing.T) {
		got := kdefRefusals(t, "SP$ Draw | Cost$ B", "DB$ Draw | ConditionDefined$ Sacrificed | ConditionPresent$ Card.Legendary")
		kdefWant(t, got, "without exactly one sacrifice paid")
	})
	t.Run("sacrificed with two picks", func(t *testing.T) {
		// the kernel snapshots only the first picked sacrifice, so a
		// Sac<2/...> cost's group would be read short
		got := kdefRefusals(t, "SP$ Draw | Cost$ B Sac<2/Creature>", "DB$ Draw | ConditionDefined$ Sacrificed | ConditionPresent$ Card")
		kdefWant(t, got, "without exactly one sacrifice paid")
	})
	t.Run("sacrificed with a sacrifice-this cost", func(t *testing.T) {
		// a Sac<1/CARDNAME> (SacSelf) cost's object is not snapshotted on
		// every payment path, so the group cannot be read exactly
		got := kdefRefusals(t, "AB$ Draw | Cost$ Sac<1/CARDNAME>", "DB$ Draw | ConditionDefined$ Sacrificed | ConditionPresent$ Card")
		kdefWant(t, got, "without exactly one sacrifice paid")
	})
	t.Run("sacrificed with one picked cost lowers", func(t *testing.T) {
		// the tightening refuses only the multi-pick and sacrifice-this
		// shapes; a single picked sacrifice on an activated ability still
		// lowers (the snapshot is the first/only pick)
		got := kdefRefusals(t, "AB$ Draw | Cost$ Sac<1/Creature>", "DB$ Draw | ConditionDefined$ Sacrificed | ConditionPresent$ Card")
		if len(got) != 0 {
			t.Errorf("refused: %q", got)
		}
	})
	t.Run("sacrificed as a subject", func(t *testing.T) {
		got := kdefRefusals(t, sac, "DB$ Destroy | Defined$ Sacrificed")
		kdefWant(t, got, `Defined$ "Sacrificed"`)
	})
}

// TestDefinedPlayerSetRefusals: a Player.<q> / Opponent.<q> set the
// lowering does not read exactly is refused with a named reason.
func TestDefinedPlayerSetRefusals(t *testing.T) {
	root := "SP$ Draw | Cost$ B"
	for _, c := range []struct{ api, key, spec, want string }{
		{"Draw", "Defined", "Player.controlsCreature.MultiColor_GE2", "count suffix or conjunction"},
		{"Draw", "Defined", "Player.controlsCreature.MultiColor+withFlying", "count suffix or conjunction"},
		{"Draw", "Defined", "Player.controlsCreature.powerGE3", "reads more than printed colours"},
		{"Draw", "Defined", "Player.controlsCreature.YouCtrl", "reads more than printed colours"},
		{"Draw", "Defined", "Player.withMostLife", "player qualifier"},
		{"Draw", "Defined", "Player.OpponentOf Remembered", "player qualifier"},
		{"Draw", "Defined", "Player.EnchantedBy", "player qualifier"},
		{"Draw", "Defined", "Player.HasCardsInGraveyard", "player qualifier"},
		{"Draw", "Defined", "Player.TappedLandForManaThisTurn", "player qualifier"},
		{"Draw", "Defined", "Opponent.lifeEQ10", "player qualifier"},
		{"Draw", "Defined", "Player.controlsEnchantment,controlsArtifact", "TokenOwner$-only"},
		{"Token", "TokenOwner", "Opponent.controlsEnchantment,controlsArtifact", "TokenOwner$-only seat spelling on Opponent"},
		{"Mill", "Defined", "Player.MaxSpeed", `Defined$ "Player.MaxSpeed"`},
		{"GainLife", "Defined", "Opponent.IsCorrupted", `Defined$ "Opponent.IsCorrupted"`},
		{"Investigate", "Defined", "Player.MaxSpeed", `Defined$ "Player.MaxSpeed"`},
	} {
		t.Run(c.api+" "+c.spec, func(t *testing.T) {
			got := kdefRefusals(t, root, "DB$ "+c.api+" | "+c.key+"$ "+c.spec)
			kdefWant(t, got, c.want)
		})
	}
	t.Run("exact shapes lower", func(t *testing.T) {
		for _, c := range [][2]string{
			{"Draw", "Player.controlsCreature.MultiColor"}, {"LoseLife", "Opponent.IsCorrupted"},
			{"DealDamage", "Player.!MaxSpeed"}, {"Draw", "Player.MaxSpeed"},
		} {
			sub := "DB$ " + c[0] + " | Defined$ " + c[1]
			switch c[0] {
			case "DealDamage":
				sub += " | NumDmg$ 2"
			case "LoseLife":
				sub += " | LifeAmount$ 3"
			}
			if got := kdefRefusals(t, root, sub); len(got) != 0 {
				t.Errorf("%s %s refused: %q", c[0], c[1], got)
			}
		}
	})
}

// TestDefinedValidRefusals: Defined$ Valid <filter> as the subject of Pump,
// PutCounter and MultiplyCounter lowers only the shapes read exactly.
func TestDefinedValidRefusals(t *testing.T) {
	root := "SP$ Draw | Cost$ B"
	for _, c := range []struct{ name, sub, want string }{
		{"pump non-creature filter", "DB$ Pump | Defined$ Valid Land.YouCtrl | NumAtt$ Double", "not a creature-only filter"},
		{"pump double beside a fixed amount", "DB$ Pump | Defined$ Valid Creature.YouCtrl | NumAtt$ Double | NumDef$ 1", "Double beside a fixed amount"},
		{"pump unread param", "DB$ Pump | Defined$ Valid Creature.YouCtrl | NumAtt$ Double | KWChoice$ Flying,Haste", "unread params [KWChoice]"},
		{"pump unread filter property", "DB$ Pump | Defined$ Valid Creature.ChosenCardStrict | NumAtt$ 1", "ChosenCardStrict"},
		{"pump compound", "DB$ Pump | Defined$ Valid Creature.YouCtrl & Valid Land | NumAtt$ 1", "a compound selector"},
		{"pump zone-qualified", "DB$ Pump | Defined$ ValidGraveyard Creature.YouCtrl | NumAtt$ 1", "a zone-qualified Valid selector"},
		{"counters per defined", "DB$ PutCounter | Defined$ Valid Creature.YouCtrl | CounterType$ P1P1 | CounterNumPerDefined$ X", "unread params [CounterNumPerDefined]"},
		{"counters zero", "DB$ PutCounter | Defined$ Valid Creature.YouCtrl | CounterType$ P1P1 | CounterNum$ 0", "not a constant of at least one"},
		{"counters kind list", "DB$ PutCounter | Defined$ Valid Creature.YouCtrl | CounterType$ P1P1,M1M1 | CounterNum$ 1", "CounterType$"},
		{"counters optional", "DB$ PutCounter | Defined$ Valid Creature.YouCtrl | CounterType$ P1P1 | CounterNum$ 1 | Optional$ True", "unread params [Optional]"},
		{"multiply unread multiplier", "DB$ MultiplyCounter | Defined$ Valid Creature.YouCtrl | CounterType$ P1P1 | Multiplier$ 3", "unread params [Multiplier]"},
		{"animate without P/T", "DB$ Animate | Defined$ Valid Creature.YouCtrl | Types$ Artifact | Optional$ True", "Power$ and Toughness$ not both set"},
		{"animate optional not true", "DB$ Animate | Defined$ Valid Creature.YouCtrl | Power$ 1 | Toughness$ 1 | Optional$ False", `Optional$ "False"`},
		{"animate unread param", "DB$ Animate | Defined$ Valid Creature.YouCtrl | Power$ 1 | Toughness$ 1 | AtEOT$ Sacrifice", "unread params [AtEOT]"},
		{"copy permanent num copies", "DB$ CopyPermanent | Defined$ Valid Creature.YouCtrl | NumCopies$ 2", "unread params [NumCopies]"},
		{"copy permanent add svars", "DB$ CopyPermanent | Defined$ Valid Creature.YouCtrl | AddSVars$ EndStepTrig", "unread params [AddSVars]"},
	} {
		t.Run(c.name, func(t *testing.T) { kdefWant(t, kdefRefusals(t, root, c.sub), c.want) })
	}
	t.Run("exact shapes lower", func(t *testing.T) {
		for _, sub := range []string{
			"DB$ Pump | Defined$ Valid Creature.YouCtrl | NumAtt$ Double | NumDef$ Double",
			"DB$ Pump | Defined$ Valid Creature.YouCtrl+StrictlyOther | NumAtt$ Double | KW$ Vigilance",
			"DB$ Pump | Defined$ Valid Creature.YouCtrl | NumAtt$ 2 | NumDef$ 2",
			"DB$ PutCounter | Defined$ Valid Creature.Other+YouCtrl | CounterType$ Indestructible | CounterNum$ 1",
			"DB$ MultiplyCounter | Defined$ Valid Creature.YouCtrl | CounterType$ P1P1",
			"DB$ Animate | Power$ 3 | Toughness$ 3 | Defined$ Valid Creature.YouCtrl+Other | Optional$ True",
			"DB$ CopyPermanent | Defined$ Valid Creature.YouCtrl+!token | NonLegendary$ True",
		} {
			if got := kdefRefusals(t, root, sub); len(got) != 0 {
				t.Errorf("%s refused: %q", sub, got)
			}
		}
	})
}
