// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

func kwMentor(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// CR 702.134: "Whenever this creature attacks, put a +1/+1 counter on
	// target attacking creature with lesser power." Mentor is an ordinary
	// Attacks self-trigger whose body is a TARGETED PutCounter -- unlike
	// Training (cards/kw_training.go), which counters the source, Mentor
	// counters another attacking creature chosen on resolution, so it cannot
	// use the Defined$ Self shape.
	//
	// The strict lesser-power restriction rides the body's Mentor$ marker,
	// read by rules' mentorAdmits at both the target offer and the CR 608.2b
	// recheck. It is NOT the corpus's powerLTX spelling: a target spec's
	// numeric RHS resolves against the announced {X} (targetSpecContext's
	// Resolve), not the resolving source's power, so `powerLTX` would offer
	// an empty list for every Mentor source (measured -- see the ticket
	// report; Unliving Psychopath's own Creature.powerLTX target offer is
	// empty for the same reason). The marker keeps the source-relative
	// comparison in ONE rules helper shared by offer and recheck.
	f.addKeywordTrigger(head, k,
		"Mode$ Attacks | ValidCard$ Card.Self | TriggerDescription$ Mentor",
		"DB$ PutCounter | ValidTgts$ Creature.attacking | TgtPrompt$ Select target attacking creature with lesser power | Mentor$ True | CounterType$ P1P1 | CounterNum$ 1", has)
}

func init() { registerKeyword(kwMentor, "Mentor") }
