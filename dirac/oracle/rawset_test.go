package oracle

import (
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// rawSet is pool name with every lowered card playable: a fixture exercises a card
// whatever the support lists say about it. A missing table panics with the error,
// which names the command that builds it.
func rawSet(name string) cardset.Set {
	s, err := cardset.RawByName(name)
	if err != nil {
		panic(err)
	}
	return s
}
