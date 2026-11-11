package kgen

import "testing"

// TestDmgAmountLowers: Tangled Colony lowers its dies trigger's
// Count$TotalDamageReceivedThisTurn amount to vDmgAmount (the kernel's
// ExDmgAmountReceived), marks the card as a damage-amount reader, and reads
// its token's TokenOwner$ You as the creating player. The card had been
// refused at the count, leaving the token body's parameters unread.
func TestDmgAmountLowers(t *testing.T) {
	c := lowerCorpusCard(t, "Tangled Colony")
	if len(c.Refusals) != 0 {
		t.Fatalf("Tangled Colony refused: %v", c.Refusals)
	}
	if !c.DmgAmt {
		t.Fatal("the card is not marked as a damage-amount reader (CardDef.DmgAmt)")
	}
	var tokens, owner, amount int
	for _, a := range c.Abilities {
		for _, in := range a.Prog {
			if in.Op != "CreateToken" {
				continue
			}
			tokens++
			if in.Obj.Kind == rYou {
				owner++
			}
			if in.A.Kind == vDmgAmount {
				amount++
			}
		}
	}
	if tokens != 1 || owner != 1 || amount != 1 {
		t.Fatalf("CreateToken tokens %d, owner Your %d, amount vDmgAmount %d; want 1 each", tokens, owner, amount)
	}
}
