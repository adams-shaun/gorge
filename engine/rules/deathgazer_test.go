package rules

import "testing"

// Deathgazer (CR 603.7 delayed triggers, 506.4): "Whenever CARDNAME blocks or
// becomes blocked by a nonblack creature, destroy that creature at end of
// combat." Each half registers an end-of-combat delayed destroy of the
// pair's other member. The walls (0/8, 1/4) and the 3/3 Hill Giant survive
// Deathgazer's two points of damage, so only the delayed destroy can kill the
// nonblack one -- even after Deathgazer itself died in the combat (the delayed
// trigger remembers the creature, CR 603.7c); the black creatures are spared.

// deathgazerScenario: p1 attacks p0, p0 blocks, the block trigger resolves,
// combat settles into the end step (the delayed destroy resolves on the way).
func deathgazerScenario(name string, p0, p1, attackers []string, blocks string, expect string) string {
	q := func(xs []string) string {
		s := ""
		for i, x := range xs {
			if i > 0 {
				s += ","
			}
			s += `"` + x + `"`
		}
		return "[" + s + "]"
	}
	return `{"name":"` + name + `","cr":["603.7","506.4"],"why":"Deathgazer destroys at end of combat",` +
		`"setup":{"p0":{"battlefield":` + q(p0) + `},"p1":{"battlefield":` + q(p1) + `}},` +
		`"steps":[{"op":"attack","seat":1,"attackers":` + q(attackers) + `,"defender":"p0"},` +
		`{"op":"block","seat":0,"blocks":` + blocks + `},` +
		`{"op":"resolve","seat":0},{"op":"pass_to","seat":0,"step":"end","active":"p1"}],` +
		`"expect":` + expect + `}`
}

func TestDeathgazerDestroysTheNonblackCreatureItBlocks(t *testing.T) {
	// p1's Hill Giant attacks, p0's Deathgazer blocks it and dies to it
	playRuleScenario(t, deathgazerScenario("deathgazer-blocks-nonblack",
		[]string{"Deathgazer"}, []string{"Hill Giant"}, []string{"p1:Hill Giant"},
		`[["p0:Deathgazer","p1:Hill Giant"]]`,
		`[{"card":"p1:Hill Giant","zone":"graveyard"},{"card":"p0:Deathgazer","zone":"graveyard"}]`))
}

func TestDeathgazerDestroysTheNonblackCreatureThatBlocksIt(t *testing.T) {
	// p1's Deathgazer attacks, p0's Wall of Stone blocks it
	playRuleScenario(t, deathgazerScenario("deathgazer-blocked-by-nonblack",
		[]string{"Wall of Stone"}, []string{"Deathgazer"}, []string{"p1:Deathgazer"},
		`[["p0:Wall of Stone","p1:Deathgazer"]]`,
		`[{"card":"p0:Wall of Stone","zone":"graveyard"},{"card":"p1:Deathgazer","zone":"battlefield"}]`))
}

func TestDeathgazerSparesABlackCreature(t *testing.T) {
	playRuleScenario(t, deathgazerScenario("deathgazer-blocks-black",
		[]string{"Deathgazer"}, []string{"Gurmag Angler"}, []string{"p1:Gurmag Angler"},
		`[["p0:Deathgazer","p1:Gurmag Angler"]]`,
		`[{"card":"p1:Gurmag Angler","zone":"battlefield"}]`))
	playRuleScenario(t, deathgazerScenario("deathgazer-blocked-by-black",
		[]string{"Wall of Bone"}, []string{"Deathgazer"}, []string{"p1:Deathgazer"},
		`[["p0:Wall of Bone","p1:Deathgazer"]]`,
		`[{"card":"p0:Wall of Bone","zone":"battlefield"},{"card":"p1:Deathgazer","zone":"battlefield"}]`))
}
