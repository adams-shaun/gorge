package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// kprepareLower lowers a synthetic Prepare card: front is the lines after
// PT:, face the inset spell's script body.
func kprepareLower(t *testing.T, front, face string) []refusal {
	t.Helper()
	src := "Name:Prep Probe\nManaCost:1 R\nTypes:Creature Human\nPT:1/1\n" + front + "\nAlternateMode:Prepare\n\nALTERNATE\n\nName:Prep Spell\nManaCost:R\n" + face + "\n"
	card, diags := cards.ParseBytes("prep_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{"Prep Probe"}, byName: map[string]*cards.Card{"Prep Probe": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lw := lowerPool(p)
	if _, _, _, err := buildPool(lw); err != nil {
		t.Fatal(err)
	}
	return lw.cards[0].Refusals
}

const (
	kprepSorcery = "Types:Sorcery\nA:SP$ GainLife | LifeAmount$ 1"
	kprepEnters  = "K:ETBReplacement:Other:DBReady\nSVar:DBReady:DB$ AlterAttribute | Attributes$ Prepared"
	kprepTrig    = "T:Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You | TriggerZones$ Battlefield | Execute$ TrigReady\nSVar:TrigReady:DB$ AlterAttribute | Attributes$ Prepared"
)

// TestPrepareRefusals: AlterAttribute Prepared lowers on a Prepare card's
// own permanent, or on a referent (a target, Defined$ Enchanted, Activate$
// False) in its gorge-read shape, and Count$ValidSelf only in
// its two IsPrepared spellings; every other shape is refused with a
// named reason.
func TestPrepareRefusals(t *testing.T) {
	for _, ok := range []string{
		kprepEnters,
		kprepTrig,
		strings.Replace(kprepTrig, "Prepared", "Prepared | Defined$ Self", 1),
		// a referent path (the kernel decides at resolution)
		strings.Replace(kprepTrig, "Prepared", "Prepared | Defined$ Enchanted", 1),
		strings.Replace(kprepTrig, "Prepared", "Prepared | ValidTgts$ Creature", 1),
		strings.Replace(kprepTrig, "Prepared", "Prepared | ValidTgts$ Creature | Activate$ False", 1),
		strings.Replace(kprepTrig, "Prepared", "Prepared | Activate$ False", 1),
		strings.Replace(kprepTrig, "Execute$ TrigReady", "Execute$ TrigReady | CheckSVar$ Count$ValidSelf Card.!IsPrepared", 1),
		strings.Replace(kprepTrig, "Execute$ TrigReady", "Execute$ TrigReady | CheckSVar$ Count$ValidSelf Card.IsPrepared", 1),
	} {
		if r := kprepareLower(t, ok, kprepSorcery); len(r) != 0 {
			t.Fatalf("an accepted shape is refused: %v\n%s", r, ok)
		}
	}
	cases := []struct{ name, front, face, want string }{
		{"face is a creature", kprepEnters, "Types:Creature Elf\nPT:1/1", "Prepare face 1 is not an instant or sorcery"},
		{"Defined Valid", strings.Replace(kprepTrig, "Prepared", "Prepared | Defined$ Valid Creature.YouCtrl", 1), kprepSorcery, `AlterAttribute Prepared Defined$ "Valid Creature.YouCtrl"`},
		{"Defined EffectSource", strings.Replace(kprepTrig, "Prepared", "Prepared | Defined$ EffectSource", 1), kprepSorcery, `AlterAttribute Prepared Defined$ "EffectSource"`},
		{"Activate True", strings.Replace(kprepTrig, "Prepared", "Prepared | ValidTgts$ Creature | Activate$ True", 1), kprepSorcery, `AlterAttribute Prepared Activate$ "True"`},
		{"Optional", strings.Replace(kprepTrig, "Prepared", "Prepared | ValidTgts$ Creature | Optional$ True", 1), kprepSorcery, "AlterAttribute Prepared Optional$"},
		{"ETB replacement target", strings.Replace(kprepEnters, "Prepared", "Prepared | ValidTgts$ Creature", 1), kprepSorcery, "in an ETB replacement"},
		{"ValidSelf other spec", strings.Replace(kprepTrig, "Execute$ TrigReady", "Execute$ TrigReady | CheckSVar$ Count$ValidSelf Card.Creature", 1), kprepSorcery, `Count$ValidSelf spec "Card.Creature"`},
		{"Card.prepared", strings.Replace(kprepTrig, "Execute$ TrigReady", "Execute$ TrigReady | IsPresent$ Card.Self+prepared", 1), kprepSorcery, "prepared"},
	}
	for _, c := range cases {
		r := kprepareLower(t, c.front, c.face)
		var got []string
		for _, x := range r {
			got = append(got, x.Reason)
		}
		if all := strings.Join(got, " | "); !strings.Contains(all, c.want) {
			t.Errorf("%s: refusals %q, want one containing %q", c.name, all, c.want)
		}
	}
}

// TestPrepareNonPreparation: AlterAttribute Prepared on a card that is not
// a preparation card refuses.
func TestPrepareNonPreparation(t *testing.T) {
	src := "Name:Plain Probe\nManaCost:1 R\nTypes:Creature Human\nPT:1/1\n" + kprepTrig + "\n"
	card, diags := cards.ParseBytes("plain_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	card.Link()
	p := &pool{names: []string{"Plain Probe"}, byName: map[string]*cards.Card{"Plain Probe": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lw := lowerPool(p)
	var got []string
	for _, x := range lw.cards[0].Refusals {
		got = append(got, x.Reason)
	}
	if all := strings.Join(got, " | "); !strings.Contains(all, "AlterAttribute Prepared on a non-preparation card") {
		t.Errorf("refusals %q", all)
	}
}
