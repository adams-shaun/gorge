package oracle

import (
	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// fdnkernel.go: the adapter hooks for package kernel on a generated card
// set (dirac/cardset), with the FDN* functions wrapping the "fdn" set.

// fdnSet is the generated FDN set as reached through cardset.ByName. A missing
// table panics with ByName's error, which names the command that builds it.
func fdnSet() cardset.Set {
	s, err := cardset.ByName("fdn")
	if err != nil {
		panic(err)
	}
	return s
}

// SetDeckSupported reports whether every card of a gorge deck is in set and
// supported.
func SetDeckSupported(set cardset.Set, d []*cards.Card) bool {
	supported := set.Supported()
	for _, c := range d {
		i, ok := set.Lookup(cardName(c))
		if !ok || !supported[i] {
			return false
		}
	}
	return true
}

// FDNDeckSupported is SetDeckSupported over the "fdn" set.
func FDNDeckSupported(d []*cards.Card) bool { return SetDeckSupported(fdnSet(), d) }

// SetSide is RealKernel over a table built from set (SetMatchup's) with
// the generated table's adapter hooks: gorge ability indices through
// set.AbilityOrigin (a latent ability, granted / delayed / reflexive, has
// none), tokens named by their script's face name, and the ChGen* choice
// kinds canonicalised as gorge poses them.
func SetSide(set cardset.Set, tbl *kop.Table, cfg Config) SideFactory {
	rk := RealKernel(tbl, nil, cfg)
	return func(m *Matchup, seed uint64) (Side, error) {
		side, err := rk(m, seed)
		if err != nil {
			return nil, err
		}
		r := side.(*realSide)
		var t canon.Translator
		t.BindSet(set, tbl, m.reg)
		r.gname, r.origin, r.generated = t.GName, t.Origin, true
		return r, nil
	}
}

// FDNSide is SetSide over the "fdn" set.
func FDNSide(tbl *kop.Table, cfg Config) SideFactory { return SetSide(fdnSet(), tbl, cfg) }
