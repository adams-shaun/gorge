package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The Extort class (CR 702.101): the printed K:Extort expansion is a SpellCast
// trigger whose body lowers to the ExtExtort op, and Pontiff of Blight's
// layer-6 AddKeyword$ Extort grant becomes a latent trigger on the grantor's
// card. A refusal here would keep kgen from lowering every card in the class.
func TestExtortKeywordLowers(t *testing.T) {
	names := []string{"Blind Obedience", "Crypt Ghast", "Life Insurance", "Pontiff of Blight", "Tithe Drinker", "Treasury Thrull"}
	cardsDir := ratchetCards(t)
	dir := t.TempDir()
	list := filepath.Join(dir, "cards.txt")
	if err := os.WriteFile(list, []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	byName := map[string]*lCard{}
	for _, c := range lw.cards {
		byName[c.Name] = c
	}
	for _, name := range names {
		c := byName[name]
		if c == nil {
			t.Errorf("%s: not lowered", name)
			continue
		}
		if len(c.Refusals) != 0 {
			var why []string
			for _, r := range c.Refusals {
				why = append(why, r.Node+": "+r.Reason)
			}
			t.Errorf("%s: refused: %s", name, strings.Join(why, "; "))
			continue
		}
		if !cardHasExtort(c) {
			t.Errorf("%s: no SpellCast trigger with an Extort body", name)
		}
	}
	// Pontiff's static must carry the latent granted trigger, not silently
	// drop its AddKeyword$ Extort: the kernel lists it once per granting
	// static for every affected creature (CR 702.101b).
	pontiff := byName["Pontiff of Blight"]
	if pontiff == nil {
		return
	}
	granted := false
	for _, st := range pontiff.Statics {
		for _, gi := range st.Grant {
			if gi < 0 || gi >= len(pontiff.Abilities) {
				t.Fatalf("Pontiff of Blight: grant index %d out of range", gi)
			}
			a := pontiff.Abilities[gi]
			if !a.Latent || a.Name != "GrantedExtort" {
				continue
			}
			for _, in := range a.Prog {
				if in.Op == "Extort" {
					granted = true
				}
			}
		}
	}
	if !granted {
		t.Fatal("Pontiff of Blight: no latent GrantedExtort trigger with an Extort body")
	}
}

// cardHasExtort reports whether c has a battlefield SpellCast trigger for the
// controller whose body is the Extort op.
func cardHasExtort(c *lCard) bool {
	for _, a := range c.Abilities {
		if a.Kind != akTriggered || a.Trigger == nil || a.Trigger.Event != "SpellCast" ||
			a.Trigger.Zone != zBattlefield || a.Trigger.Whose != whoYou {
			continue
		}
		for _, in := range a.Prog {
			if in.Op == "Extort" {
				return true
			}
		}
	}
	return false
}
