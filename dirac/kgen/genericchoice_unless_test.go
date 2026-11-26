package kgen

// Nimble Hobbit is a triggered controller api:GenericChoice whose every
// choice carries an UnlessCost$. The lowering marks the one-mode charm so the
// kernel's mode offer runs the same payability gate as gorge's
// GenericChoiceAvailable (kop.AfCharmTrigger; dirac/modal_unless.go).

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

func TestNimbleHobbitTriggerCharmMarkedForUnlessGate(t *testing.T) {
	lw := lowerNamedPool(t, []string{"Nimble Hobbit"})
	if len(lw.cards) != 1 {
		t.Fatalf("lowered %d cards, want one Nimble Hobbit", len(lw.cards))
	}
	if n := len(lw.cards[0].Refusals); n != 0 {
		t.Fatalf("Nimble Hobbit refused: %s", lw.cards[0].Refusals[0].Reason)
	}
	marked := false
	for _, a := range lw.cards[0].Abilities {
		if a.GenericChoice {
			marked = true
			if a.Modes < 2 {
				t.Fatalf("marked charm has %d modes, want its two unless-cost choices", a.Modes)
			}
		}
	}
	if !marked {
		t.Fatal("Nimble Hobbit's triggered GenericChoice is not marked for the unless gate")
	}
	_, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	if len(bs) != 1 {
		t.Fatalf("built %d cards, want one", len(bs))
	}
	found := false
	for i := range bs[0].def.Abilities {
		if bs[0].def.Abilities[i].AFlags&kop.AfCharmTrigger != 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("kop.AfCharmTrigger missing from the built Nimble Hobbit")
	}
}
