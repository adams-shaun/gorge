package kgen

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// SynthUnionTable lowers names, plus the basics, into one pool table in this
// process: the build a synthetic pool's table gets (buildOneTable), for a list that
// no pool on disk holds, a pool's own cards with the enablers other pools lend it
// (kfuzz -synth-with). Nothing is written. A list over a pool table's subtype
// limit (cardset.PoolMaxSubtypes) is an error; a game that needs more than one
// game table holds is refused later, by its matchup (cardset.SubtypeRefusal).
func SynthUnionTable(names []string, cardsDir, poolName string) (*cardset.PoolData, error) {
	p, err := loadCardNames(withBasics(names), cardsDir)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", poolName, err)
	}
	d, _, _, err := tableOf(p, poolName, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", poolName, err)
	}
	return d, nil
}
