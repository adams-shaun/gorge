package cardset

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// InteractionRefusal is a matchup the kernel cannot play because of what its
// cards do together, read from the built table: the game would stall at its
// start. It is not a by-name refusal; either card plays without the other.
type InteractionRefusal struct {
	Reason string   // the stall the game would have started with (dirac.StallName)
	Why    string   // the rule the kernel does not model
	Cards  []string // the cards carrying the two halves
}

func (e *InteractionRefusal) Error() string {
	return fmt.Sprintf("interaction %s: %s: %s", e.Reason, strings.Join(e.Cards, ", "), e.Why)
}

// interactionGuard refuses t, built from the sorted pool indexes pool, when
// kop.PairGuard does. A token carrier is named by the deck card that makes it,
// since nothing can be done about a token but through its maker.
func (s dataSet) interactionGuard(t *kop.Table, pool []int) error {
	if kop.PairGuard == nil {
		return nil
	}
	r := kop.PairGuard(t)
	if r == nil {
		return nil
	}
	d := s.d
	var names []string
	seen := map[string]bool{}
	for _, id := range r.Cards {
		n := t.Cards[id].Name
		if i := pool[id]; i >= d.NumCards {
			n = s.tokenMaker(i, pool, n)
		}
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	return &InteractionRefusal{Reason: r.Reason, Why: r.Why, Cards: names}
}

// tokenMaker names the first deck card in pool that makes token i, directly or
// through another token; name when none does.
func (s dataSet) tokenMaker(i int, pool []int, name string) string {
	var makes func(c, depth int) bool
	makes = func(c, depth int) bool {
		if depth == 0 {
			return false
		}
		for _, t := range s.d.CardTokens[c] {
			if int(t) == i || makes(int(t), depth-1) {
				return true
			}
		}
		return false
	}
	for _, c := range pool {
		if c < s.d.NumCards && makes(c, 4) {
			return s.d.Names[c]
		}
	}
	return name
}
