package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// valuestaticformsCarriers is one card per form:
// each must lower with no refusal, and valuestaticformsLIR spot-checks what it lowered to.
var valuestaticformsCarriers = map[string]string{
	"Ball Lightning":            "Sacrifice with no Defined$/SacValid$ = the source",
	"Alexi, Zephyr Mage":        "Discard<2/Card/cards> cost (description part)",
	"Blazing Specter":           "Defined$ TriggeredTarget of combat damage to a player",
	"Dictate of Kruphix":        "Defined$ TriggeredPlayer of a Phase trigger",
	"Curse of Chains":           "Defined$ Enchanted",
	"Bone Sabres":               "Defined$ TriggeredAttackerLKICopy",
	"Artillerize":               "Sac<1/Artifact;Creature> (cost ';' alternation)",
	"Baseball Bat":              "Card.AttachedBy (= EquippedBy)",
	"Crusade":                   "Creature.White (colour property)",
	"Doom Blade":                "Creature.nonBlack (negated colour)",
	"Akki Underling":            "Count$ValidHand Card.YouOwn",
	"Endless Ranks of the Dead": "/HalfDown value modifier", // was Hydroid Krasis: its cast trigger reads the spell's X, refused since kverify-c4
	"Gnaw to the Bone":          "/Times.2 value modifier",
	"Mercy Killing":             "SacrificeAll Defined$ Targeted, no ValidCards$",
	"Goblin Chainwhirler":       "ValidPlayers$ Player.Opponent",
}

// valuestaticformsLIR is a lowered-IR fragment each form must produce.
var valuestaticformsLIR = []string{
	"Sacrifice obj=You A=1 F={types=0x0 Self}", // Ball Lightning
	"cost={0UX} T Discard2",                    // Alexi, Zephyr Mage
	"Discard obj=TriggeredPlayer A=1",          // Blazing Specter
	"Draw obj=TriggeredPlayer A=1",             // Dictate of Kruphix
	"Tap obj=Equipped",                         // Curse of Chains
	"PutCounter obj=Triggered A=4 CS=P1P1",     // Bone Sabres
	"spellcost=Sac1{types=0x4 | types=0x2}",    // Artillerize
	"DamageAll A=1 F={players=2 |",
	"trigger Attacks F={types=0x0 EquippedBy}", // Baseball Bat
	"static Continuous F={types=0x2 color=W}",  // Crusade
	"F={types=0x2 notcolor=B}",
	"cond=Value(op1,HandSizeOf(n=1,You),7)",
	"CreateToken obj=You A=Count({types=0x2 sub=Zombie ctrl=1})/2",
	"SacrificeAll F={types=0x0 TargetedBy}",                    // Mercy Killing                                   // Endless Ranks of the Dead
	"GainLife obj=You A=CountGraveyard({types=0x2 owner=1})*2", // Gnaw to the Bone // Akki Underling                 // Doom Blade                // Goblin Chainwhirler
}

func TestValueAndStaticFormsAccepted(t *testing.T) {
	cardsDir := ratchetCards(t)
	dir := t.TempDir()
	var names []string
	for n := range valuestaticformsCarriers {
		names = append(names, n)
	}
	list := filepath.Join(dir, "cards.txt")
	if err := os.WriteFile(list, []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	var sb strings.Builder
	for _, c := range lw.cards {
		for _, r := range c.Refusals {
			t.Errorf("%s (%s): refused: %s", c.Name, valuestaticformsCarriers[c.Name], r.Reason)
		}
		dumpLIR(&sb, c)
	}
	lir := sb.String()
	for _, want := range valuestaticformsLIR {
		if !strings.Contains(lir, want) {
			t.Errorf("lowered IR lacks %q", want)
		}
	}
}
