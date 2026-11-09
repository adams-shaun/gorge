package kgen

import "testing"

// cmd5TriggerShapeForms: four cmd5 cards whose trigger shapes lower. Each
// card is read from the corpus, so a corpus-less run skips (ratchetCards).
var cmd5TriggerShapeForms = []formCase{
	{card: "Angelic Skirmisher", form: "mass Pump with KWChoice: one choice, one PumpAllEach branch per keyword", lir: "PumpAllEach A=0 B=0 CS=Lifelink"},
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
