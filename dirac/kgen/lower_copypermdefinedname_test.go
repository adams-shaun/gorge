package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// cnCard parses and links one inline card script.
func cnCard(t *testing.T, src string) *cards.Card {
	t.Helper()
	c, diags := cards.ParseBytes("copyname_probe.txt", []byte(src))
	if c == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := c.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	return c
}

const cnMutavault = "Name:Mutavault\nManaCost:no cost\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ C | Amount$ 1\n"

// cnLower lowers a sorcery whose one spell ability is `line`, over a registry
// holding the named extra cards, and returns it with its dumped IR.
func cnLower(t *testing.T, line string, extra ...string) (*lCard, string) {
	t.Helper()
	reg := cards.NewRegistry()
	for _, src := range extra {
		reg.Add(cnCard(t, src))
	}
	card := cnCard(t, "Name:Copier\nManaCost:B\nTypes:Sorcery\nA:"+line+"\n")
	p := &pool{names: []string{"Copier"}, byName: map[string]*cards.Card{"Copier": card}, nDecks: map[string]int{}, reg: reg}
	c := lowerCard(p, "Copier", card, false)
	var sb strings.Builder
	dumpLIR(&sb, c)
	return c, sb.String()
}

// TestLowerCopypermDefinedname: CopyPermanent DefinedName$ lowers to the
// named-card copy (ExtCopyNamed) with the card riding the pool as a table
// token; a name gorge's universe lacks lowers to nothing (its Note path, no
// copy); every shape the op does not carry refuses by name.
func TestLowerCopypermDefinedname(t *testing.T) {
	t.Run("named card", func(t *testing.T) {
		c, ir := cnLower(t, "SP$ CopyPermanent | Cost$ B | DefinedName$ Mutavault | TokenTapped$ True", cnMutavault)
		if r := lowerRefusals(c); len(r) != 0 {
			t.Fatalf("refused: %v", r)
		}
		if !strings.Contains(ir, "CopyNamed") || !strings.Contains(ir, "Tapped|card=Mutavault") {
			t.Errorf("IR lacks the named copy with the tapped rider:\n%s", ir)
		}
		if len(c.Tokens) != 1 || c.Tokens[0] != copyCardKey+"Mutavault" {
			t.Errorf("tokens %v, want [%s]", c.Tokens, copyCardKey+"Mutavault")
		}
	})
	t.Run("name not in the universe", func(t *testing.T) {
		c, ir := cnLower(t, "SP$ CopyPermanent | Cost$ B | DefinedName$ No Such Card | TokenTapped$ True", cnMutavault)
		if r := lowerRefusals(c); len(r) != 0 {
			t.Fatalf("refused: %v", r)
		}
		if strings.Contains(ir, "CopyNamed") || len(c.Tokens) != 0 {
			t.Errorf("a name the universe lacks must lower to no copy (tokens %v):\n%s", c.Tokens, ir)
		}
	})
	forest := "Name:Forest\nManaCost:no cost\nTypes:Basic Land Forest\n"
	t.Run("named card is a basic land", func(t *testing.T) {
		c, ir := cnLower(t, "SP$ CopyPermanent | Cost$ B | DefinedName$ Forest", cnMutavault, forest)
		if r := lowerRefusals(c); len(r) != 0 {
			t.Fatalf("refused: %v", r)
		}
		if !strings.Contains(ir, "CopyNamed") {
			t.Errorf("IR lacks the named copy:\n%s", ir)
		}
		if len(c.Tokens) != 1 || c.Tokens[0] != copyCardKey+"Forest" {
			t.Errorf("tokens %v, want [%s]", c.Tokens, copyCardKey+"Forest")
		}
	})
	maker := "Name:Maker\nManaCost:no cost\nTypes:Land\nA:AB$ Token | Cost$ T | TokenScript$ no_such_token\n"
	broken := "Name:Broken\nManaCost:no cost\nTypes:Land\nA:AB$ NoSuchApi | Cost$ T\n"
	badger := "Name:Giant Badger\nManaCost:1 G G\nTypes:Creature Badger\nPT:2/2\n"
	for _, v := range []struct{ name, line, want string }{
		{"num copies", "SP$ CopyPermanent | Cost$ B | DefinedName$ Mutavault | NumCopies$ 2", `DefinedName$ "Mutavault" beside NumCopies$`},
		{"target", "SP$ CopyPermanent | Cost$ B | DefinedName$ Mutavault | ValidTgts$ Opponent | Controller$ TargetedPlayer", `DefinedName$ "Mutavault" beside ValidTgts$`},
		{"defined", "SP$ CopyPermanent | Cost$ B | DefinedName$ Mutavault | Defined$ Self", `DefinedName$ "Mutavault" beside Defined$`},
		{"choices", "SP$ CopyPermanent | Cost$ B | DefinedName$ Mutavault | Choices$ Creature", `DefinedName$ "Mutavault" beside Choices$`},
		{"counters", "SP$ CopyPermanent | Cost$ B | DefinedName$ Mutavault | WithCountersType$ P1P1", `DefinedName$ "Mutavault" beside WithCountersType$`},
		{"controller", "SP$ CopyPermanent | Cost$ B | DefinedName$ Mutavault | Controller$ Opponent", `CopyPermanent Controller "Opponent"`},
		{"named card does not lower", "SP$ CopyPermanent | Cost$ B | DefinedName$ Broken", `DefinedName$ "Broken": the named card does not lower`},
		{"named card makes tokens", "SP$ CopyPermanent | Cost$ B | DefinedName$ Maker", `DefinedName$ "Maker"`},
		{"named card is not a land", "SP$ CopyPermanent | Cost$ B | DefinedName$ Giant Badger", `DefinedName$ "Giant Badger": the named card is not a land`},
	} {
		t.Run(v.name, func(t *testing.T) {
			c, _ := cnLower(t, v.line, cnMutavault, maker, broken, badger, forest)
			var got []string
			for _, r := range lowerRefusals(c) {
				got = append(got, r.Reason)
			}
			kdefWant(t, got, v.want)
		})
	}
}

// TestLowerCopypermDefinednameMutableExplorer: Mutable Explorer lowers, and
// builds the named copy for its tapped Mutavault: the sub-op, the CopyFlags
// tapped bit, and F naming a table token that is Mutavault's face.
func TestLowerCopypermDefinednameMutableExplorer(t *testing.T) {
	all, bs := keclBuild(t, []string{"Mutable Explorer"})
	i := keclIndex(t, all, "Mutable Explorer")
	if r := lowerRefusals(all[i]); len(r) != 0 {
		t.Fatalf("Mutable Explorer refused: %v", r)
	}
	var found *kop.Instr
	for _, ab := range bs[i].def.Abilities {
		for n := range ab.Prog {
			if in := &ab.Prog[n]; in.Op == kop.OpExt && in.Ext() == kop.ExtCopyNamed {
				found = in
			}
		}
	}
	if found == nil {
		t.Fatal("no ExtCopyNamed instruction built")
	}
	if found.C&kop.CpTapped == 0 {
		t.Errorf("C %#x lacks CpTapped", found.C)
	}
	tok := int(found.B.N)
	if tok < len(all) && all[tok].Name != copyCardKey+"Mutavault" {
		t.Errorf("B.N %d names %q, want %s", tok, all[tok].Name, copyCardKey+"Mutavault")
	}
	if tok >= len(all) {
		t.Fatalf("B.N %d is outside the %d-entry pool", tok, len(all))
	}
	if found.F != 0 {
		t.Errorf("F = %d: instruction walkers read it as a filter", found.F)
	}
}
