package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestRageOfPurphorosLowers: the can't-regenerate follow-up that remembers
// its parent link's target folds into the damage instruction as the
// DmgNoRegenTarget flag; it emits nothing of its own and the Scry still lowers.
func TestRageOfPurphorosLowers(t *testing.T) {
	lc := cmd5Lower(t, "Rage of Purphoros")
	got := cmd5Ops(lc, "DealDamage")
	if len(got) != 1 || !strings.Contains(got[0].CS, "NoRegenTarget") {
		t.Fatalf("DealDamage = %+v, want one with NoRegenTarget", got)
	}
	_, all, _, err := buildPool(&lowered{cards: []*lCard{lc}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	found := false
	for _, ab := range all[0].def.Abilities {
		for _, in := range ab.Prog {
			if in.Op == kop.OpDealDamageEx && in.C&kop.DmgNoRegenTarget != 0 && in.C&kop.DmgNoRegen == 0 {
				found = true
			}
		}
	}
	if !found {
		t.Error("no OpDealDamageEx with DmgNoRegenTarget built")
	}
}

// TestNoRegenParentTargetRefusals: the ParentTarget follow-up folds only into
// a one-creature-target DealDamage that is its own parent link.
func TestNoRegenParentTargetRefusals(t *testing.T) {
	const eff = "SVar:DBNoRegen:DB$ Effect | RememberObjects$ ParentTarget | ForgetOnMoved$ Battlefield | StaticAbilities$ NoRegen\n" +
		"SVar:NoRegen:Mode$ CantRegenerate | ValidCard$ Card.IsRemembered\n"
	for _, c := range []struct{ name, head, want string }{
		{"any target", "A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 2 | SubAbility$ DBNoRegen", "Effect CantRegenerate ParentTarget beyond one creature target"},
		{"two targets", "A:SP$ DealDamage | ValidTgts$ Creature | TargetMin$ 0 | TargetMax$ 2 | NumDmg$ 2 | SubAbility$ DBNoRegen", "Effect CantRegenerate ParentTarget beyond one creature target"},
		{"not a damage parent", "A:SP$ Draw | NumCards$ 1 | SubAbility$ DBNoRegen", "Effect CantRegenerate ParentTarget without the previous DealDamage"},
	} {
		t.Run(c.name, func(t *testing.T) {
			src := "Name:Probe\nManaCost:R\nTypes:Instant\n" + c.head + " | SpellDescription$ Probe.\n" + eff
			card, diags := cards.ParseBytes("Probe.txt", []byte(src))
			if card == nil || len(diags) != 0 {
				t.Fatalf("parse: %v", diags)
			}
			if d := card.Link(); len(d) != 0 {
				t.Fatalf("link: %v", d)
			}
			p := &pool{names: []string{"Probe"}, byName: map[string]*cards.Card{"Probe": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
			var got []string
			for _, r := range lowerPool(p).cards[0].Refusals {
				got = append(got, r.Reason)
			}
			kdefWant(t, got, c.want)
		})
	}
}
