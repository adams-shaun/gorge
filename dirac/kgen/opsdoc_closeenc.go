package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"cost:CloseEnc", "Cost.CloseEnc (N 0)",
			"Close Encounter's own additional cost, RaiseCost Cost$ ChooseCard<1/Creature.YouCtrl+inZoneBattlefield;Creature.YouOwn+inZoneExile+warped> — \"as an additional cost to cast this spell, choose a creature you control or a warped creature card you own in exile\" (CR 118.3, 601.2b). The sole ChooseCard cost gorge's parser admits (costvocab.CloseEncounterChooseSpec, cost/token_match.go); the choice moves and marks nothing. gorge offers the cast only with a candidate (pay.NonManaCastable's RevealOrChoose gate over pay.RevealOrChooseCandidates: the creatures the payer controls, then the payer's warped creature cards in exile — warpCostExiled over the event log), takes a lone one unasked and asks a KChoose (option kind \"choosecost\") over several; a candidate that vanished aborts the cast (revealCostOrChooseAsk's abortCast, CR 601.2h, 733.1). The elected object rides the cast's paid \"Revealed\" list (Ctx.Paid.Revealed; EmitChoiceCosts announces a CHOICE, never a reveal), which the spell body's RevealedCards$ value reads — the chosen object's power where it now is, live on the battlefield, the printed face in exile (gorge count_ref_property.go's CardPower). A table byte, not a CostFlags bit (the words are full). A spell's own additional cost."},
		opsDoc{"ref:ChosenEnc", "RefChosenEnc",
			"The object the resolving spell's own CloseEncounter ChooseCard additional cost chose (kop.Cost.CloseEnc; gorge's Ctx.Paid.Revealed, the cast-cost PAID list its RevealedCards$ value reads). It rides the spell entry's trigObj, as RefDiscarded does: a spell's trigObj is otherwise unused but by a sneak cast's attack, which never pays a ChooseCard cost, and a copy keeps it (CR 707.10, gorge inheritCopyRecord). The body's X is the chosen object's power where it now is: live on the battlefield, the printed face in exile (gorge count_ref_property.go's CardPower). 0 when the cast chose nothing. Close Encounter's DealDamage."})
}

// usageCloseEnc records the choose-cost row of a card's own cost.
func usageCloseEnc(c kop.Cost, name string, add func(label, name string)) {
	if c.CloseEnc {
		add("cost:CloseEnc", name)
	}
}
