package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A comma-listed TokenScript$ (kop.TrList: Fae Offering's Clue, Food and
// Treasure) is one creation event in gorge: a ReplAddToken (Jolene, the
// Plunder Queen's "those tokens plus an additional Treasure token") matching
// any token of it appends its one extra token after the event's own tokens
// (rules/replacement_token.go applyTokenReplacementToPlan). The kernel mints
// the list op by op, so the extra token would follow the matching op, not the
// list, and an adder matching two listed tokens would add two. The runtime
// check stalls that creation (repl_tokorder.go replTokensOp); this guard
// refuses the matchup at its start instead, so a game never reaches the
// unmodelled stall.

// prepTokListAdd sets genPrep.tokListAdd: a TrList mint and a ReplAddToken
// ability both on the table.
func prepTokListAdd(t *kop.Table, g *genPrep) {
	var list, add bool
	for i := range t.Cards {
		list = list || tokListMinter(&t.Cards[i], i)
		add = add || tokAdder(&t.Cards[i], i)
	}
	g.tokListAdd = list && add
}

// tokListAdderPair is the pair text and halves of StallTokRewrite's matchup
// refusal (stallclass.go pairRefusalN).
func tokListAdderPair() (string, func(*kop.CardDef, int) bool, func(*kop.CardDef, int) bool) {
	return "a comma-listed TokenScript$ mint beside a token adder: gorge adds the extra token once per creation event, after all the listed tokens, and the kernel mints the list op by op",
		tokListMinter, tokAdder
}

// tokListMinter is true of a card with a CreateToken op of a listed script.
var tokListMinter = abilityOn(func(a *kop.Ability) bool {
	return progHas(a, func(in *kop.Instr) bool { return in.Op == kop.OpCreateToken && in.B.N&kop.TrList != 0 })
})

// tokAdder is true of a card with a ReplAddToken replacement.
var tokAdder = abilityOn(func(a *kop.Ability) bool {
	return a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplAddToken
})
