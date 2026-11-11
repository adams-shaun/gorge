package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

const consumeSpiritFixture = "Name:Consume Spirit\n" +
	"ManaCost:X 1 B\n" +
	"Types:Sorcery\n" +
	"A:SP$ DealDamage | NumDmg$ X | XColor$ Black | ValidTgts$ Any | SubAbility$ DBHeal\n" +
	"SVar:DBHeal:DB$ GainLife | LifeAmount$ X\n" +
	"Oracle:x\n"

// TestConsumeSpiritXColorLowers pins the kgen side of Consume Spirit's
// XColor$ Black: the parameter is read (not left to the unread-params
// refusal) and lands on the card record as one black XPip beside {X}{1}{B}.
func TestConsumeSpiritXColorLowers(t *testing.T) {
	c := lowerScriptCard(t, "Consume Spirit", consumeSpiritFixture)
	if len(c.Refusals) != 0 {
		t.Fatalf("Consume Spirit refuses: %v", c.Refusals)
	}
	if !c.Cost.X || c.Cost.XColor != 'B' || c.Cost.Pips[2] != 1 || c.Cost.Generic != 1 {
		t.Fatalf("cost = %+v, want {generic 1, black 1, X, XColor B}", c.Cost)
	}
	mc, err := manaCostOf(c.Cost)
	if err != nil {
		t.Fatalf("manaCostOf: %v", err)
	}
	if mc.XPip != uint8(kop.B+1) || mc.X != 1 || mc.Pips[kop.B] != 1 || mc.Generic != 1 {
		t.Fatalf("built cost = %+v, want {generic 1, black 1, X 1, XPip black}", mc)
	}
}

// TestConsumeSpiritCorpusLowers is the corpus check behind the synthetic
// fixture: the pinned script also writes a bare NumDmg$/LifeAmount$ X with no
// SVar:X, which gorge resolves as the paid X.
func TestConsumeSpiritCorpusLowers(t *testing.T) {
	c := lowerCorpusCard(t, "Consume Spirit")
	if len(c.Refusals) != 0 {
		t.Fatalf("corpus Consume Spirit refuses: %v", c.Refusals)
	}
	if c.Cost.XColor != 'B' {
		t.Fatalf("corpus XColor = %q, want B", c.Cost.XColor)
	}
}

// TestConsumeSpiritXColorMultiColourRefuses: the kernel models one X colour;
// the black-and-red Soul Burn shape is refused by name, not silently read.
func TestConsumeSpiritXColorMultiColourRefuses(t *testing.T) {
	src := strings.Replace(consumeSpiritFixture, "XColor$ Black", "XColor$ Black,Red", 1)
	c := lowerScriptCard(t, "Soul Burn", src)
	var got string
	for _, r := range c.Refusals {
		got += r.Reason + "\n"
	}
	if !strings.Contains(got, "only a single colour") {
		t.Fatalf("refusals = %q, want the multi-colour XColor refusal", got)
	}
}
