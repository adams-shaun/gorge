// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

// kwProvoke expands Provoke (CR 702.39a): "Whenever this creature attacks,
// you may have target creature defending player controls untap and block
// this creature this combat if able." The optional Attacks self trigger
// untaps its target, then the sub-ability binds that same creature to block
// the source (MustBlock's default attacker) until end of combat.
func kwProvoke(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	if has("T", k) {
		return
	}
	block := "__kwProvokeBlock"
	f.setSVar(block, "DB$ MustBlock | Defined$ ParentTarget | Duration$ UntilEndOfCombat")
	f.addKeywordTrigger(head, k,
		"Mode$ Attacks | ValidCard$ Card.Self | OptionalDecider$ You | TriggerDescription$ Provoke",
		"DB$ Untap | ValidTgts$ Creature.DefenderCtrl | TgtPrompt$ Select target creature defending player controls | SubAbility$ "+block, has)
}

func init() { registerKeyword(kwProvoke, "Provoke") }
