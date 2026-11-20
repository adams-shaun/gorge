package oracle

// The Count$ heads kgen lowers whose amount needs no stack entry: a count of
// the objects a filter matches on the board or in a zone, and the life totals.
func init() {
	for _, h := range []string{
		"Count$Valid Creature.YouCtrl", "Count$Valid Creature.OppCtrl", "Count$Valid Creature",
		"Count$Valid Land.YouCtrl", "Count$Valid Land.OppCtrl", "Count$Valid Permanent.YouCtrl",
		"Count$Valid Card.tapped+YouCtrl", "Count$Valid Creature.attacking",
		"Count$Valid Creature.token+YouCtrl", "Count$Valid Creature.powerGE3+YouCtrl",
		"Count$ValidHand Card.YouOwn", "Count$ValidHand Card.OppOwn",
		"Count$ValidGraveyard Card.YouOwn", "Count$ValidGraveyard Creature.YouOwn", "Count$ValidGraveyard Card.OppOwn",
		"Count$ValidExile Card.YouOwn",
		"Count$YourLifeTotal",
	} {
		RegisterCountAtom(CountAtom{Name: "count:" + h, GorgeHead: h})
	}
}
