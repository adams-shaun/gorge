package kgen

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAttacksPerAttackerAccepted: an Attacks trigger whose ValidCard
// can match several attackers lowers Defined$ TriggeredAttacker[LKICopy]
// (gorge queues one trigger per matching attacker, binding
// TriggerCard to it, as the kernel does). Kragma Warcaller's Minotaur.YouCtrl
// was the refusal; Bone Sabres' Creature.EquippedBy stays accepted.
func TestAttacksPerAttackerAccepted(t *testing.T) {
	cardsDir := ratchetCards(t)
	dir := t.TempDir()
	list := filepath.Join(dir, "cards.txt")
	if err := os.WriteFile(list, []byte("Kragma Warcaller\nBone Sabres\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	seen := 0
	for _, c := range lw.cards {
		if c.Name == "Kragma Warcaller" || c.Name == "Bone Sabres" {
			seen++
			if len(c.Refusals) != 0 {
				t.Errorf("%s: refused %v, want accepted", c.Name, c.Refusals)
			}
		}
	}
	if seen != 2 {
		t.Fatalf("lowered %d of the 2 carriers", seen)
	}
}
