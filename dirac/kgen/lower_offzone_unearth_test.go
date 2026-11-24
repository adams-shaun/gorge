package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestOffZoneOfferForms lowers the off-zone Unearth grants the offer walk
// models (dirac/kgen/lower_offzone.go offZoneKeyword's Unearth arm,
// kw_unearthgrant.go): each granting card keeps a StatUnearthGrant static
// naming its latent unearth body (the printed kwUnearth expander's
// AB$ ChangeZone, GrantedKeywordAbility), so the kernel's graveyard offer
// runs the body with the affected card as its source. The priced shapes are
// the explicit mana costs; a CardManaCost grant charges the affected card's
// own printed mana cost, which the body bakes as zero. Every other granted
// keyword keeps its named refusal (Varolz's Scavenge, Wire Surgeons' Encore,
// Naktamun's Embalm), and a grant whose static the kernel cannot read live
// (Grixis's EffectZone$ Command source) keeps the named refusal too.
func TestOffZoneOfferForms(t *testing.T) {
	cases := []struct {
		name string
		head string
		// the grant's cost: generic pips and coloured pips (WUBRG order)
		gen     uint8
		pips    [6]uint8
		cardGen bool // CardManaCost: the affected card's own mana cost
	}{
		{"Sedris, the Traitor King", "Unearth", 2, [6]uint8{0, 0, 1, 0, 0, 0}, false},
		{"Dregscape Sliver", "Unearth", 2, [6]uint8{}, false},
		{"Solemn Doomguide", "Unearth", 1, [6]uint8{0, 0, 1, 0, 0, 0}, false},
	}
	reg := testutil.CorpusRegistry(t)
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			card, ok := reg.Lookup(tc.name)
			if !ok || card == nil {
				t.Fatalf("%s not in the corpus", tc.name)
			}
			// Precondition: the printed static really is a graveyard grant of
			// this keyword, so a pass cannot come from a card whose text moved.
			grant := false
			for _, st := range card.Faces[0].Statics {
				if st.Params["AffectedZone"] == "Graveyard" && strings.HasPrefix(st.Params["AddKeyword"], tc.head) {
					grant = true
				}
			}
			if !grant {
				t.Fatalf("%s has no AffectedZone$ Graveyard AddKeyword$ %s static", tc.name, tc.head)
			}
			p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
			lc := lowerCard(p, card.Faces[0].Name, card, false)
			if len(lc.Refusals) != 0 {
				t.Fatalf("%s refusals = %v, want none: the unearth grant must lower", tc.name, lc.Refusals)
			}
			all, built, _, err := buildPool(&lowered{cards: []*lCard{lc}})
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			_ = all
			def := &built[0].def
			statics := 0
			for _, ab := range def.Abilities {
				if ab.Kind != kop.AbStatic || ab.Static.Kind != kop.StatUnearthGrant {
					continue
				}
				statics++
				if ab.Static.GrantBody == 0 {
					t.Fatal("the StatUnearthGrant static names no latent body")
				}
				if ab.Static.Zone != kop.ZoneGraveyard {
					t.Fatalf("static zone %d, want the graveyard", ab.Static.Zone)
				}
				body := &def.Abilities[ab.Static.GrantBody-1]
				if body.Kind != kop.AbActivated || body.AFlags&kop.AfLatent == 0 {
					t.Fatalf("granted body %+v: want a latent activated ability", body.Kind)
				}
				if tc.cardGen {
					if body.Cost.Mana.Generic != 0 || body.Cost.Mana.Pips != [6]uint8{} {
						t.Fatalf("CardManaCost body cost %+v: want the placeholder zero", body.Cost.Mana)
					}
				} else {
					if body.Cost.Mana.Generic != tc.gen || body.Cost.Mana.Pips != tc.pips {
						t.Fatalf("body cost %+v, want generic %d pips %v", body.Cost.Mana, tc.gen, tc.pips)
					}
				}
				bodyOK := false
				for _, in := range body.Prog {
					if in.Op == kop.OpUnearth {
						bodyOK = true
					}
				}
				if !bodyOK {
					t.Fatal("the body lowers no OpUnearth: the haste and exile riders would not apply")
				}
			}
			if statics != 1 {
				t.Fatalf("%d StatUnearthGrant statics, want one", statics)
			}
		})
	}
}

// TestOffZoneOfferUnearthBesideRefusal: Mishra, Tamer of Mak Fawa's grant
// lowers, but the card stays refused for a printed shape beside it (its
// "Ward:Sac<1/Permanent>" keyword, unlowered): the pool keeps the card out,
// and the refusal must not name the grant that lowered.
func TestOffZoneOfferUnearthBesideRefusal(t *testing.T) {
	const name = "Mishra, Tamer of Mak Fawa"
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup(name)
	if !ok || card == nil {
		t.Fatalf("%s not in the corpus", name)
	}
	grant := false
	for _, st := range card.Faces[0].Statics {
		if st.Params["AffectedZone"] == "Graveyard" && strings.HasPrefix(st.Params["AddKeyword"], "Unearth") {
			grant = true
		}
	}
	if !grant {
		t.Fatalf("%s has no AffectedZone$ Graveyard AddKeyword$ Unearth static", name)
	}
	p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
	lc := lowerCard(p, card.Faces[0].Name, card, false)
	if len(lc.Refusals) == 0 {
		t.Fatal("refusals = none, want the printed Ward keyword's")
	}
	for _, r := range lc.Refusals {
		if strings.Contains(r.Reason, "Unearth") {
			t.Fatalf("refusal %q names the unearth grant that lowered", r.Reason)
		}
	}
}

// TestOffZoneOfferRefusals keeps the shapes the offer walk does not model on
// the named refusal the census pins: Grixis's static is read from the command
// zone (EffectZone$ Command), which eachStaticC's battlefield walk does not
// see, and the other graveyard keyword grants (Scavenge, Encore, Embalm) have
// no offer reader yet.
func TestOffZoneOfferRefusals(t *testing.T) {
	cases := []struct {
		name string
		head string
	}{
		{"Grixis", "Unearth"},
		{"Varolz, the Scar-Striped", "Scavenge"},
		{"Wire Surgeons", "Encore"},
		{"Naktamun", "Embalm"},
		{"Graywater's Fixer", "Encore"},
	}
	reg := testutil.CorpusRegistry(t)
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			card, ok := reg.Lookup(tc.name)
			if !ok || card == nil {
				t.Fatalf("%s not in the corpus", tc.name)
			}
			p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
			lc := lowerCard(p, card.Faces[0].Name, card, false)
			want := "(" + tc.head + " grant not modelled)"
			shaped := 0
			for _, r := range lc.Refusals {
				if strings.Contains(r.Reason, want) {
					shaped++
				}
			}
			if shaped != 1 {
				t.Fatalf("%s refusals = %v, want exactly one containing %q", tc.name, lc.Refusals, want)
			}
		})
	}
}
