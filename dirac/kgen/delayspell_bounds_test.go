package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// builtNamed lowers and builds names as one pool and returns each card's
// built definition by name.
func builtNamed(t *testing.T, names []string) (map[string]kop.CardDef, []kop.Filter) {
	t.Helper()
	lw := lowerNamedPool(t, names)
	all, bs, pt, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]kop.CardDef{}
	for i, c := range all {
		if len(c.Refusals) != 0 {
			var rs []string
			for _, r := range c.Refusals {
				rs = append(rs, r.Reason)
			}
			t.Fatalf("%s: refused: %s", c.Name, strings.Join(rs, "; "))
		}
		out[c.Name] = bs[i].def
	}
	return out, pt.filters
}

// TestDelaySpellSourceBound: Loki's delayed SpellCast filter lowers with the
// mana-value bound as the source's power (PredCMCLEV / VSelfPower), the one
// bound fireDelayedSpell's evalCtx carries.
func TestDelaySpellSourceBound(t *testing.T) {
	defs, filters := builtNamed(t, []string{"Loki Laufeyson"})
	def, ok := defs["Loki Laufeyson"]
	if !ok {
		t.Fatal("Loki Laufeyson was not built")
	}
	found := false
	for i := range def.Abilities {
		a := &def.Abilities[i]
		if a.Trig.Event != kop.TrigSpellCast {
			continue
		}
		f := filters[a.Trig.Filter]
		if f.Pred != kop.PredCMCLEV || f.PredV.K != kop.VSelfPower {
			t.Errorf("latent %d filter = pred %d bound %+v, want PredCMCLEV bound VSelfPower", i, f.Pred, f.PredV)
			continue
		}
		found = true
	}
	if !found {
		t.Fatal("no latent SpellCast ability carries the source-power bound")
	}
}
