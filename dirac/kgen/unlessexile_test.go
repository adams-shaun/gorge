package kgen

import (
	"strings"
	"testing"
)

// ExileFromGrave<N/Spec> as an UnlessCost$ lowers
// to the unless instruction's ExileGrave payment (gorge pay.AdvanceUnless's
// exilecost part); a latent (delayed) body stays refused.
var unlessExileForms = []formCase{
	{card: "Trystan, Callous Cultivator", form: "front face lowers (its back face carries the unless cost)"},
	{card: "Heated Argument", form: "UnlessCost$ ExileFromGrave<1/Card> | UnlessSwitched$ True", lir: "UnlessSwitched obj=You A=1 CS=ExileGrave"},
	{card: "Rotting Giant", form: "UnlessCost$ ExileFromGrave<1/Card> (paid)", lir: "UnlessPaid obj=You A=1 CS=ExileGrave"},
}

func TestUnlessExileGrave(t *testing.T) {
	checkForms(t, unlessExileForms)
	// Trystan's back face: UnlessSwitched, A 1, the Elf filter.
	found := false
	for _, c := range lowerNamedPool(t, []string{"Trystan, Callous Cultivator"}).tokens {
		if c.Name != faceKey("Trystan, Callous Cultivator", 1) {
			continue
		}
		found = true
		var sb strings.Builder
		dumpLIR(&sb, c)
		if !strings.Contains(sb.String(), "UnlessSwitched obj=You A=1 CS=ExileGrave F={types=0x0 sub=Elf") {
			t.Errorf("Trystan back face lacks the Elf ExileGrave UnlessSwitched:\n%s", sb.String())
		}
	}
	if !found {
		t.Error("Trystan back face not lowered")
	}
}
