package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestOffZoneCyclingForms lowers the AffectedZone$ Hand Cycling and
// TypeCycling grants the hand offer models (lower_offzone.go offZoneCycling,
// dirac kw_cyclinggrant.go): each granting card keeps a StatCyclingGrant
// static in the hand zone naming its latent cycling body (the printed
// expansion, cards.GrantedKeywordAbility: a hand activated ability whose cost
// discards the card, AfCycling), so the kernel's hand offer runs the body with
// the affected card as its source.
func TestOffZoneCyclingForms(t *testing.T) {
	cases := []struct {
		name string
		head string
		gen  uint8
		pips [6]uint8 // WUBRG order, then colourless
	}{
		{"Rhet-Tomb Mystic", "Cycling", 1, [6]uint8{0, 1, 0, 0, 0, 0}},
		{"Jo Grant", "Cycling", 2, [6]uint8{1, 0, 0, 0, 0, 0}},
		{"Tectonic Reformation", "Cycling", 0, [6]uint8{0, 0, 0, 1, 0, 0}},
		{"Homing Sliver", "TypeCycling", 3, [6]uint8{}},
	}
	reg := testutil.CorpusRegistry(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			card, ok := reg.Lookup(tc.name)
			if !ok || card == nil {
				t.Fatalf("%s not in the corpus", tc.name)
			}
			// Precondition: the printed static really is a hand grant of this
			// keyword, so a pass cannot come from a card whose text moved.
			grant := false
			for _, st := range card.Faces[0].Statics {
				if st.Params["AffectedZone"] == "Hand" && strings.HasPrefix(st.Params["AddKeyword"], tc.head) {
					grant = true
				}
			}
			if !grant {
				t.Fatalf("%s has no AffectedZone$ Hand AddKeyword$ %s static", tc.name, tc.head)
			}
			p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
			lc := lowerCard(p, card.Faces[0].Name, card, false)
			if len(lc.Refusals) != 0 {
				t.Fatalf("%s refusals = %v, want none: the cycling grant must lower", tc.name, lc.Refusals)
			}
			_, built, _, err := buildPool(&lowered{cards: []*lCard{lc}})
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			def := &built[0].def
			statics := 0
			for _, ab := range def.Abilities {
				if ab.Kind != kop.AbStatic || ab.Static.Kind != kop.StatCyclingGrant {
					continue
				}
				statics++
				if ab.Static.GrantBody == 0 {
					t.Fatal("the StatCyclingGrant static names no latent body")
				}
				if ab.Static.Zone != kop.ZoneHand {
					t.Fatalf("static zone %d, want the hand", ab.Static.Zone)
				}
				body := &def.Abilities[ab.Static.GrantBody-1]
				if body.Kind != kop.AbActivated || body.AFlags&kop.AfLatent == 0 || body.AFlags&kop.AfCycling == 0 || body.ActZone != kop.ZoneHand {
					t.Fatalf("granted body kind %v flags %v zone %v: want a latent hand cycling ability", body.Kind, body.AFlags, body.ActZone)
				}
				if body.Cost.Flags&kop.CostDiscardSelf == 0 {
					t.Fatal("the body's cost does not discard the card itself")
				}
				if body.Cost.Mana.Generic != tc.gen || body.Cost.Mana.Pips != tc.pips {
					t.Fatalf("body cost %+v, want generic %d pips %v", body.Cost.Mana, tc.gen, tc.pips)
				}
			}
			if statics != 1 {
				t.Fatalf("%d StatCyclingGrant statics, want one", statics)
			}
		})
	}
}

// TestOffZoneCyclingRefusals keeps the shapes the hand offer does not model
// on the generic AffectedZone refusal: Bicycle Rack's static is read from the
// command zone (EffectZone$ Command), which the battlefield static walk does
// not see, and Satoru Umezawa's ninjutsu grant has no hand offer yet.
func TestOffZoneCyclingRefusals(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{"Bicycle Rack", "Satoru Umezawa"} {
		t.Run(name, func(t *testing.T) {
			card, ok := reg.Lookup(name)
			if !ok || card == nil {
				t.Fatalf("%s not in the corpus", name)
			}
			p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
			lc := lowerCard(p, card.Faces[0].Name, card, false)
			named := false
			for _, r := range lc.Refusals {
				if strings.Contains(r.Reason, `AffectedZone "Hand" without MayPlay`) {
					named = true
				}
			}
			if !named {
				t.Fatalf("%s refusals = %v, want the AffectedZone Hand reason", name, lc.Refusals)
			}
		})
	}
}
