package kgen

// A token may make a token of its own (Diabolical Salvation's Devil makes a
// Treasure as it dies, lowerPool). A refused inner token refuses the token
// that makes it, to a fixpoint, so the card-level checks (lowerPool,
// buildPool), which read a card's own tokens, see the refusal.

// refuseByNestedToken carries a lowering refusal from a token to the
// tokens that make it.
func refuseByNestedToken(tokens []*lCard) {
	byName := map[string]*lCard{}
	for _, t := range tokens {
		byName[t.Name] = t
	}
	refuseNested(len(tokens), func(i int) *lCard { return tokens[i] }, func(i int) []*lCard {
		var made []*lCard
		for _, n := range tokens[i].Tokens {
			if u := byName[n]; u != nil {
				made = append(made, u)
			}
		}
		return made
	})
}

// refuseByNestedBuiltToken is refuseByNestedToken over buildPool's built
// tokens: all[nCards:] are the tokens, bs[i].tokens index all.
func refuseByNestedBuiltToken(all []*lCard, bs []built, nCards int) {
	refuseNested(len(all)-nCards, func(i int) *lCard { return all[nCards+i] }, func(i int) []*lCard {
		var made []*lCard
		for _, ti := range bs[nCards+i].tokens {
			made = append(made, all[ti])
		}
		return made
	})
}

func refuseNested(n int, tok func(int) *lCard, made func(int) []*lCard) {
	for changed := true; changed; {
		changed = false
		for i := 0; i < n; i++ {
			t := tok(i)
			if len(t.Refusals) > 0 {
				continue
			}
			for _, u := range made(i) {
				if len(u.Refusals) > 0 {
					t.Refusals = append(t.Refusals, refusal{Card: t.Name, Node: "token " + u.Name, Reason: "token refused"})
					changed = true
					break
				}
			}
		}
	}
}
