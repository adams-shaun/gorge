package kgen

import (
	"strings"
	"testing"
)

// TestKeywordCipher: a K:Cipher instant or sorcery lowers with OpExt
// ExtCipherEncode as its spell program's last instruction (gorge appends the
// encode to the resolution chain's tail); every form the kernel does not read
// refuses by name.
func TestKeywordCipher(t *testing.T) {
	for _, name := range []string{
		"Call of the Nightwing", "Hands of Binding", "Hidden Strings", "Last Thoughts", "Mental Vapors",
		"Midnight Recovery", "Paranoid Delusions", "Shadow Slice", "Stolen Identity", "Trait Doctoring",
		"Undercity Plague", "Voidwalk", "Whispering Madness", "Writ of Return",
	} {
		lc := lowerCorpusCard(t, name)
		for _, rf := range lc.Refusals {
			t.Errorf("%s: refused %s: %s", name, rf.Node, rf.Reason)
		}
		var spell *lAbility
		for _, a := range lc.Abilities {
			if a.Kind == akSpell && !a.Latent {
				spell = a
				break
			}
		}
		if spell == nil || len(spell.Prog) == 0 || spell.Prog[len(spell.Prog)-1].Op != "CipherEncode" {
			t.Errorf("%s: the spell program does not end with CipherEncode", name)
		}
		n := 0
		for _, in := range spell.Prog {
			if in.Op == "CipherEncode" {
				n++
			}
		}
		if n != 1 {
			t.Errorf("%s: %d CipherEncode instructions, want 1", name, n)
		}
	}

	// A body that casts a card: the kernel's free cast follows the program.
	if lc := lowerCorpusCard(t, "Arcane Heist"); !cipherRefused(lc, "casts a card") {
		t.Errorf("Arcane Heist: want the cast-a-card refusal, got %v", lc.Refusals)
	}

	const base = "Name:Cipher Probe\nManaCost:1 U\nTypes:Sorcery\nK:Cipher\nA:SP$ Draw | Defined$ You | NumCards$ 1 | SpellDescription$ Draw a card.\n"
	for _, c := range []struct{ name, src, want string }{
		{"plain sorcery", base, ""},
		{"a permanent", strings.Replace(base, "Types:Sorcery", "Types:Enchantment", 1), "permanent card"},
		{"a parameter", strings.Replace(base, "K:Cipher", "K:Cipher:2", 1), "Cipher with a parameter"},
		{"beside flashback", base + "K:Flashback:3 U\n", "moves the resolved spell"},
	} {
		got := strings.Join(lowerScript(t, "Cipher Probe", c.src), "; ")
		switch {
		case c.want == "" && strings.Contains(got, "Cipher"):
			t.Errorf("%s: refused %q", c.name, got)
		case c.want != "" && !strings.Contains(got, c.want):
			t.Errorf("%s: refusals %q, want %q", c.name, got, c.want)
		}
	}
}

func cipherRefused(lc *lCard, want string) bool {
	for _, rf := range lc.Refusals {
		if strings.Contains(rf.Reason, want) {
			return true
		}
	}
	return false
}
