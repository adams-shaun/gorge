package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// addStaticAbilityForms: AddStaticAbility$ on a Mode$ Continuous static
// grants a static to ANOTHER object with that object as its source. The
// SetMaxHandSize reader is wired on both sides (Nerd Rage, see
// grantedStaticKinds). Every other granted kind refuses by name until its own
// phase, and an inner mode that does not lower refuses with the inner reason.
var addStaticAbilityForms = []formCase{
	{card: "Nerd Rage", form: "granted SetMaxHandSize (gorge reads it)", lir: "granted-static NoMaxHandSize"},
	{card: "Callaphe, Beloved of the Sea", form: "granted RaiseCost per recipient (phase 2)"},
	{card: "Zurgo, Thunder's Decree", form: "granted CantSacrifice behind the outer Phases$ End of Turn gate", lir: "StepIn"},
	{card: "Tomik, Orzhov Lawmage", form: "granted AttackRestrict (inner mode does not lower)", refused: "AddStaticAbility PWTomik: static mode AttackRestrict"},
	{card: "Acolyte of Bahamut", form: "granted ReduceCost EffectZone$ Battlefield (inner does not lower)", refused: "AddStaticAbility DragonReduce"},
	{card: "The Masamune", form: "granted Panharmonicon to the graveyard (inner does not lower)", refused: "AddStaticAbility Masarmonicon"},
}

func TestAddStaticAbilityForms(t *testing.T) {
	cardsDir := ratchetCards(t)
	var names []string
	for _, f := range addStaticAbilityForms {
		names = append(names, f.card)
	}
	list := filepath.Join(t.TempDir(), "cards.txt")
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
	for _, f := range addStaticAbilityForms {
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
		if f.refused == "" && got != "" {
			t.Errorf("%s (%s): refused: %s", f.card, f.form, got)
		}
		if f.refused != "" && !strings.Contains(got, f.refused) {
			t.Errorf("%s (%s): want refusal %q, got %q", f.card, f.form, f.refused, got)
		}
		if f.lir == "" {
			continue
		}
		var sb strings.Builder
		dumpLIR(&sb, c)
		if !strings.Contains(sb.String(), f.lir) {
			t.Errorf("%s (%s): lowered IR lacks %q:\n%s", f.card, f.form, f.lir, sb.String())
		}
	}
}

// TestAddStaticAbilityBuildsGrantedStatic: Nerd Rage lowers the inner static
// beside its card's own and builds it into CardDef.Granted, never into
// Abilities, with the outer static naming it; its AddTrigger$ grant keeps
// working beside it.
func TestAddStaticAbilityBuildsGrantedStatic(t *testing.T) {
	if grantedStaticKinds["NoMaxHandSize"] != "" {
		t.Fatal("precondition: the NoMaxHandSize gate is not open")
	}
	lw := lowerNamedPool(t, []string{"Nerd Rage"})
	all, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range all {
		if c.Name != "Nerd Rage" {
			continue
		}
		if r := lowerRefusals(c); len(r) != 0 {
			t.Fatalf("refused with the gate open: %v", r)
		}
		d := bs[i].def
		if len(d.Granted) != 1 || d.Granted[0].Kind != kop.AbStatic || d.Granted[0].Static.Kind != kop.StatNoMaxHandSize {
			t.Fatalf("Granted = %+v, want one StatNoMaxHandSize static", d.Granted)
		}
		outer, trig := 0, 0
		for _, a := range d.Abilities {
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatNoMaxHandSize {
				t.Errorf("the granted static leaked into Abilities")
			}
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && a.Static.GrantSt == 1 {
				outer++
				if a.Static.Grant == 0 {
					t.Errorf("the outer static lost its AddTrigger$ grant")
				}
			}
			if a.Kind == kop.AbTriggered && a.AFlags&kop.AfLatent != 0 {
				trig++
			}
		}
		if outer != 1 || trig != 1 {
			t.Errorf("outer statics naming the grant %d, latent triggers %d; want 1 and 1", outer, trig)
		}
		return
	}
	t.Fatal("Nerd Rage not built")
}
