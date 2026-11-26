package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// planeBodyPin is one plane's refusal pin: want is a refusal substring that
// must still appear among the card's reasons, gone one that must not (the
// shape a ticket wave moved), clean that the card lowers with no refusal.
type planeBodyPin struct {
	card, want, gone, form string
	clean                  bool
}

// planechasePlaneBodies pins the three corpus planes' remaining kgen
// refusals. The planebodies ticket's ChaosEnsues-body shapes are lowered
// (zone_digplanar.go, api_planechase.go) and the tapsformana ticket admits
// the reflected command-zone TapsForMana trigger on a plane face, so what is
// left is Eloren Wilds' DB$ Effect body and Selesnya Loft Gardens'
// until-end-of-turn Effect trigger (effectTapExtraRefuse).
var planechasePlaneBodies = []planeBodyPin{
	// Eloren Wilds: the chaos body's DB$ Effect (a CantBeCast static over the
	// remembered player beside a PlaneswalkedTo exile trigger) is the
	// planebodies ticket's next shape; the reflected TapsForMana trigger no
	// longer refuses.
	{card: "Eloren Wilds", form: "reflected command-zone TapsForMana trigger admitted", gone: "Mode=TapsForMana"},
	{card: "Eloren Wilds", form: "ChaosEnsues body -> Effect static+trigger", want: "Effect StaticAbilities$ \"STCantCast\" beside Triggers$ (a Duration$)"},
	// The Fertile Lands of Saulvinia: the ChaosEnsues body lowers end to end
	// (the planar dig, the remembered-plane chaos and the Cleanup) and the
	// reflected TapsForMana trigger is admitted: no refusal is left.
	{card: "The Fertile Lands of Saulvinia", form: "lowers with no refusal", clean: true},
	{card: "The Fertile Lands of Saulvinia", form: "planar dig body moved", gone: "DB$ DigUntil"},
	// Selesnya Loft Gardens: its Effect TapsForMana trigger keeps the
	// tap-extra ticket's refusal; the two ActiveZones$ Command replacements
	// are the planebodies ticket's remaining shape.
	{card: "Selesnya Loft Gardens", form: "TapsForMana Effect trigger (tap-extra ticket)", want: "Effect TapsForMana trigger until end of turn"},
}

// TestPlanechasePlaneBodiesRefused pins the three corpus planes' remaining
// refusals: every shape the tapsformana ticket owns must still refuse by
// name, and every ChaosEnsues-body shape this ticket lowered must be gone.
func TestPlanechasePlaneBodiesRefused(t *testing.T) {
	cardsDir := ratchetCards(t)
	dir := t.TempDir()
	var names []string
	for _, f := range planechasePlaneBodies {
		names = append(names, f.card)
	}
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
	for _, f := range planechasePlaneBodies {
		c := byName[f.card]
		if c == nil {
			t.Errorf("%s: not lowered", f.card)
			continue
		}
		var reasons []string
		for _, r := range c.Refusals {
			reasons = append(reasons, r.Node+": "+r.Reason)
		}
		got := strings.Join(reasons, "; ")
		if f.clean && len(c.Refusals) != 0 {
			t.Errorf("%s (%s): want no refusal, got %q", f.card, f.form, got)
		}
		if f.want != "" && !strings.Contains(got, f.want) {
			t.Errorf("%s (%s): want refusal %q, got %q", f.card, f.form, f.want, got)
		}
		if f.gone != "" && strings.Contains(got, f.gone) {
			t.Errorf("%s (%s): refusal %q must be gone, got %q", f.card, f.form, f.gone, got)
		}
	}
}
