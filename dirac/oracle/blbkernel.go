package oracle

import (
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// blbkernel.go: the BLB lockstep pool. The adapter hooks are SetSide's; the
// pool is 17lands BLB PremierDraft decks built by
// dirac/oracle/pools/decks.py --set BLB with the FDN pool's recipe. No 17lands BLB
// deck is fully supported by the blb set, so each pool deck has had at
// most six kernel-refused maindeck copies replaced by basics (its notes name
// them); the script's --max-refused 0 is the strict pool. The pool registry
// (pools.go) says where its decks/ and pool.tsv live.

// blbSet is the generated BLB set as reached through cardset.ByName. A missing
// table panics with ByName's error, which names the command that builds it.
func blbSet() cardset.Set {
	s, err := cardset.ByName("blb")
	if err != nil {
		panic(err)
	}
	return s
}

// BLBMatchup is SetMatchup over the "blb" set.
func BLBMatchup(c *Corpus, a, b string, mulligans int) (*Matchup, *kop.Table, error) {
	return SetMatchup(blbSet(), c, a, b, mulligans)
}

// BLBSide is SetSide over the "blb" set.
func BLBSide(tbl *kop.Table, cfg Config) SideFactory { return SetSide(blbSet(), tbl, cfg) }
