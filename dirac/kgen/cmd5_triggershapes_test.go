package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// cmd5TriggerShapeForms: four cmd5 cards whose trigger shapes lower. Each
// card is read from the corpus, so a corpus-less run skips (ratchetCards).
var cmd5TriggerShapeForms = []formCase{
	{card: "Angelic Skirmisher", form: "mass Pump with KWChoice: one choice, one PumpAllEach branch per keyword", lir: "PumpAllEach A=0 B=0 CS=Lifelink"},
	{card: "Hunter's Insight", form: "delayed combat-damage trigger to a player or a planeswalker (Filter2 beside TfToPlayer)", lir: "F2={players=3 | types=0x10}"},
}

func TestCmd5TriggerShapesLower(t *testing.T) { checkForms(t, cmd5TriggerShapeForms) }

// TestPumpAllEachChoiceRefusals: a mass keyword choice lowers only beside no
// P/T change and no fixed keyword list.
func TestPumpAllEachChoiceRefusals(t *testing.T) {
	root := "SP$ Draw | Cost$ B"
	for _, c := range []struct{ name, sub, want string }{
		{"with a fixed keyword", "DB$ Pump | Defined$ Valid Creature.YouCtrl | KW$ Vigilance | KWChoice$ Flying,Haste", "Pump with KW$ and KWChoice$"},
		{"with a P/T change", "DB$ Pump | Defined$ Valid Creature.YouCtrl | NumAtt$ 1 | KWChoice$ Flying,Haste", "unread params [KWChoice]"},
		{"unsupported keyword", "DB$ Pump | Defined$ Valid Creature.YouCtrl | KWChoice$ Flying,Banding Of Doom", "keyword"},
	} {
		t.Run(c.name, func(t *testing.T) { kdefWant(t, kdefRefusals(t, root, c.sub), c.want) })
	}
	if got := kdefRefusals(t, root, "DB$ Pump | Defined$ Valid Creature.YouCtrl | KWChoice$ First Strike,Vigilance,Lifelink"); len(got) != 0 {
		t.Errorf("refused: %q", got)
	}
}

// TestDamageTargetPlayerOrPlaneswalkerBuilds: the damage trigger's "player
// or planeswalker" recipient builds TfToPlayer beside a planeswalker
// Filter2, and no other player-plus-type second filter builds.
func TestDamageTargetPlayerOrPlaneswalkerBuilds(t *testing.T) {
	cb := &cardBuilder{pt: &poolTables{filterKey: map[string]kop.FilterID{}, exprKey: map[string]int16{}}}
	// FilterID 0 is "no filter": intern one filter first so the next is not 0
	if _, err := cb.filter(&lFilter{Clauses: []lClause{{Types: tLand}}}, kop.ZoneBattlefield); err != nil {
		t.Fatal(err)
	}
	tr := func(f2 *lFilter) *lTrigger { return &lTrigger{Event: "DamageDone", F2: f2} }
	kt, err := cb.trigger(tr(&lFilter{Players: plBoth, Clauses: []lClause{{Types: tPlaneswalker}}}))
	if err != nil {
		t.Fatal(err)
	}
	if kt.Flags&kop.TfToPlayer == 0 || kt.Filter2 == 0 {
		t.Errorf("player-or-planeswalker: Flags %v Filter2 %d, want TfToPlayer and a Filter2", kt.Flags, kt.Filter2)
	}
	if _, err := cb.trigger(tr(&lFilter{Players: plBoth, Clauses: []lClause{{Types: tCreature}}})); err == nil {
		t.Error("player-or-creature built")
	}
}
