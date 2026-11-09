package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// cmd5Lower lowers one corpus card and fails on any refusal.
func cmd5Lower(t *testing.T, name string) *lCard {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup(name)
	if !ok || card == nil {
		t.Fatalf("%s not in the corpus", name)
	}
	p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
	lc := lowerCard(p, card.Faces[0].Name, card, false)
	for _, rf := range lc.Refusals {
		t.Errorf("%s refused: %s", name, rf.Reason)
	}
	return lc
}

// cmd5Ops returns every lowered instruction of lc whose op is op.
func cmd5Ops(lc *lCard, op string) (out []lInstr) {
	for _, a := range lc.Abilities {
		for _, in := range a.Prog {
			if in.Op == op {
				out = append(out, in)
			}
		}
	}
	return out
}

// TestScourgeOfValkasLowers: the entering Dragon deals the damage to any
// target (a player or a permanent): the trigger-object DealDamageFrom in its
// any-target form.
func TestScourgeOfValkasLowers(t *testing.T) {
	lc := cmd5Lower(t, "Scourge of Valkas")
	got := cmd5Ops(lc, "DealDamageFrom")
	if len(got) != 1 || got[0].CS != "anytgt" || got[0].Obj.Kind != rTriggered {
		t.Fatalf("DealDamageFrom = %+v, want one anytgt from the trigger object", got)
	}
	_, all, _, err := buildPool(&lowered{cards: []*lCard{lc}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	found := false
	for _, ab := range all[0].def.Abilities {
		for _, in := range ab.Prog {
			if in.Op == kop.OpDealDamageFrom {
				found = true
				if in.B.N != 1 || kop.Ref(in.C) != kop.RefTarget0 || in.Obj != kop.RefTriggerObj {
					t.Errorf("kop DealDamageFrom B %v C %d Obj %d, want the player mode over target 0 from the trigger object", in.B, in.C, in.Obj)
				}
			}
		}
	}
	if !found {
		t.Error("no OpDealDamageFrom built")
	}
}

// TestThunderbreakRegentLowers: "that player" of an opponent-source
// BecomesTarget watcher is the one opponent.
func TestThunderbreakRegentLowers(t *testing.T) {
	lc := cmd5Lower(t, "Thunderbreak Regent")
	got := cmd5Ops(lc, "DealDamage")
	if len(got) != 1 || got[0].Obj.Kind != rEachOpp {
		t.Fatalf("DealDamage = %+v, want one to each opponent", got)
	}
}

// TestTriggeredSourceControllerRefusals: the source controller is the one
// opponent only where the trigger is an opponent-source BecomesTarget watcher.
func TestTriggeredSourceControllerRefusals(t *testing.T) {
	src := strings.Join([]string{
		"Name:Probe",
		"ManaCost:R",
		"Types:Creature Dragon",
		"PT:1/1",
		"T:Mode$ Attacks | ValidCard$ Card.Self | Execute$ TrigDmg | TriggerDescription$ Probe.",
		"SVar:TrigDmg:DB$ DealDamage | Defined$ TriggeredSourceController | NumDmg$ 1",
	}, "\n")
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
	kdefWant(t, got, `Defined$ "TriggeredSourceController" outside an opponent-source BecomesTarget watcher`)
}

// TestIncinerateLowers: the can't-regenerate follow-up folds into the damage
// instruction as the DmgNoRegen flag; it emits nothing of its own.
func TestIncinerateLowers(t *testing.T) {
	lc := cmd5Lower(t, "Incinerate")
	got := cmd5Ops(lc, "DealDamage")
	if len(got) != 1 || !strings.Contains(got[0].CS, "NoRegen") || !strings.Contains(got[0].CS, "Remember") {
		t.Fatalf("DealDamage = %+v, want one with Remember and NoRegen", got)
	}
	_, all, _, err := buildPool(&lowered{cards: []*lCard{lc}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	found := false
	for _, ab := range all[0].def.Abilities {
		for _, in := range ab.Prog {
			if in.Op == kop.OpDealDamageEx && in.C&kop.DmgNoRegen != 0 {
				found = true
			}
		}
	}
	if !found {
		t.Error("no OpDealDamageEx with DmgNoRegen built")
	}
}

// TestNoRegenEffectWithoutDamageRefused: the follow-up only folds into a
// RememberDamaged$ damage node just before it.
func TestNoRegenEffectWithoutDamageRefused(t *testing.T) {
	src := strings.Join([]string{
		"Name:Probe",
		"ManaCost:R",
		"Types:Instant",
		"A:SP$ GainLife | LifeAmount$ 1 | SubAbility$ DBNoRegen | SpellDescription$ Probe.",
		"SVar:DBNoRegen:DB$ Effect | RememberObjects$ Remembered.Creature | ForgetOnMoved$ Battlefield | StaticAbilities$ NoRegen",
		"SVar:NoRegen:Mode$ CantRegenerate | ValidCard$ Card.IsRemembered",
	}, "\n")
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
	kdefWant(t, got, "Effect CantRegenerate without the previous RememberDamaged$ damage")
}

// TestSkyclaveApparitionLowers: the leave trigger makes the exiled card's
// owner a token whose size is the card's mana value, in the remembered-persist
// shape.
func TestSkyclaveApparitionLowers(t *testing.T) {
	lc := cmd5Lower(t, "Skyclave Apparition")
	if !lc.RemPersist {
		t.Error("RemPersist not set")
	}
	got := cmd5Ops(lc, "CreateToken")
	if len(got) != 1 || got[0].Obj.Kind != rRememberedOwner {
		t.Fatalf("CreateToken = %+v, want one for the remembered card's owner", got)
	}
}
