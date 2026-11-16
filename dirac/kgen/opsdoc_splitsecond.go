package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

func init() {
	opsCardFlagUsage = append(opsCardFlagUsage, struct {
		label string
		on    func(d *kop.CardDef) bool
	}{"card:SplitSecond", func(d *kop.CardDef) bool { return d.SplitSecond }})
	opsEntries = append(opsEntries, opsDoc{"card:SplitSecond", "CardDef.SplitSecond",
		"Printed K:Split second (CR 702.61a; no free KwMask bit, so its own field): while a spell with it, or a copy of it (CR 707.2), is on the stack the priority offer drops every cast and every non-mana ability, keeping land plays, the face-up special action and mana abilities (gorge rules/split_second.go filterSplitSecondActions; kernel kw_splitsecond.go splitSecondHolds, behind genPrep.anySplitSecond). A granted split second (stat:StackKw A=6) reads the same gate. Sudden Shock, Krosan Grip."})
}
