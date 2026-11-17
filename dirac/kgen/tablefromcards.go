package kgen

import (
	"fmt"
	"sort"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TableFromCards lowers already-resolved cards into a pool table in this
// process: the same lowerPool/tableOf path kgen.Main's -card-list branch runs
// for real names, for cards no corpus list or synthetic pool holds (a test's
// inline cards.ParseBytes script beside corpus fillers). Each card's name is
// its front face's name, the name its gorge deck and cardset table share;
// duplicates collapse. reg is the registry the lowering reads tokens and named
// copies through (a corpus subset beside the fillers, or an empty Registry for
// a purely synthetic pool); the pool has no type-word vocabulary, so filters
// keep the pool's knownSubtype fallback. Nothing is written and no id index is
// kept: the caller turns the PoolData into a cardset.Set itself (cardset.Raw
// or AllowUnverified over cardset.NewDataSet).
func TableFromCards(reg *cards.Registry, cs []*cards.Card, poolName string) (*cardset.PoolData, error) {
	p := &pool{nDecks: map[string]int{}, byName: map[string]*cards.Card{}, reg: reg}
	seen := map[string]bool{}
	for _, c := range cs {
		if c == nil || len(c.Faces) == 0 || c.Faces[0] == nil {
			return nil, fmt.Errorf("TableFromCards: a card with no front face")
		}
		n := c.Faces[0].Name
		if seen[n] {
			continue
		}
		seen[n] = true
		p.names = append(p.names, n)
		p.nDecks[n] = 1
		p.byName[n] = c
	}
	if len(p.names) == 0 {
		return nil, fmt.Errorf("TableFromCards: no cards")
	}
	sort.Strings(p.names)
	d, _, _, err := tableOf(p, poolName, nil)
	if err != nil {
		return nil, err
	}
	return d, nil
}
