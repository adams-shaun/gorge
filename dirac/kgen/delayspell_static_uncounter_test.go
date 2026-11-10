package kgen

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestDelaySpellStaticUncounter: a Static$ True delayed SpellCast whose body
// is exactly the "that spell can't be countered" Effect lowers to the
// DelaySpellUncounter registration (no body: the kernel's record is the
// Effect); a Static$ body of any other shape keeps its refusal by name.
func TestDelaySpellStaticUncounter(t *testing.T) {
	checkCarrier(t, []string{"Mistrise Village", "Theorist's Proxy"}, nil, []string{
		"DelayedTrigger C=1 CS=NextSpellCastUncounter",
		"Latent DelayedSpellUncounter",
	})

	cardsDir := ratchetCards(t)
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte("Mistrise Village\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	// each mutation of the Effect or its replacement leaves a shape the
	// kernel's record does not carry
	for _, mu := range []struct {
		name, svar, from, to string
	}{
		{"activated abilities too", "AntiMagic", "ValidSA$ Spell", "ValidSA$ Activated"},
		{"a replacement with a body", "AntiMagic", "Layer$ CantHappen", "Layer$ Other"},
		{"no forget on leaving the stack", "DBEffect", "ForgetOnMoved$ Stack", "ForgetOnMoved$ Battlefield"},
		{"another replacement event", "AntiMagic", "Event$ Counter", "Event$ DamageDone"},
		{"an Effect with a duration", "DBEffect", "ForgetOnMoved$ Stack", "ForgetOnMoved$ Stack | Duration$ Permanent"},
	} {
		t.Run(mu.name, func(t *testing.T) {
			c := *p.byName["Mistrise Village"]
			f := *c.Faces[0]
			f.SVars = maps.Clone(f.SVars)
			if !strings.Contains(f.SVars[mu.svar], mu.from) {
				t.Fatalf("SVar %s lacks %q", mu.svar, mu.from)
			}
			f.SVars[mu.svar] = strings.Replace(f.SVars[mu.svar], mu.from, mu.to, 1)
			// the Execute$ body resolves off the face's SVars at lowering
			f.Abilities = append([]*cards.SA(nil), f.Abilities...)
			c.Faces = append(c.Faces[:0:0], &f)
			got := lowerCard(p, "Mistrise Village", &c, false)
			var rs []string
			for _, r := range got.Refusals {
				rs = append(rs, r.Reason)
			}
			if all := strings.Join(rs, " | "); !strings.Contains(all, "delayed SpellCast Static$") {
				t.Fatalf("refusals %q, want the Static$ refusal", all)
			}
		})
	}
}
