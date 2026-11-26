package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"cost:BlightX", "CostBlight with Cost.N 0",
			"Blight<X> as a spell's own RaiseCost additional cost (CR 701.68, 601.2b; Soul Immolation's \"blight X. X can't be greater than the greatest toughness among creatures you control\"): the spell prints no {X}; the part announces it. The X ask offers 0..the greatest toughness among the creatures the payer controls (a negative toughness counts 0; gorge xAsk's blightX cap, the printed `XMax$ GrTo`), then the cost-pick stage asks the one creature (unasked when there is one candidate, even at X 0) and payBlightPicks puts X -1/-1 counters on it. Cost.N stays 0 (a literal Blight<N> has N >= 1); the part shares Cost.Filter with it. kgen lowers only the `Announce$ X | XMax$ GrTo` spelling whose SVar GrTo is Count$Valid Creature.YouCtrl$GreatestCardToughness."})
}

// usageBlightX records the announced-blight cost row of an ability's cost.
func usageBlightX(c kop.Cost, name string, add func(label, name string)) {
	if c.Flags&kop.CostBlight != 0 && c.N == 0 {
		add("cost:BlightX", name)
	}
}
