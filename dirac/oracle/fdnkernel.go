package oracle

import (
	"github.com/adams-shaun/gorge/dirac"
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
	pool := make([]int, len(tbl.Cards))
	var names []string
	for i := range tbl.Cards {
		pool[i] = -1
		if j, ok := set.Lookup(tbl.Cards[i].Name); ok {
			pool[i] = j
		} else {
			// a multi-face card's other face or a token is a pool entry
			// beside the deck cards: its abilities are that
			// face's.
			if names == nil {
				names = set.Names()
			}
			for j, n := range names {
				if n == tbl.Cards[i].Name {
					pool[i] = j
				}
			}
		}
	}
	rk := RealKernel(tbl, nil, cfg)
	return func(m *Matchup, seed uint64) (Side, error) {
		side, err := rk(m, seed)
		if err != nil {
			return nil, err
		}
		r := side.(*realSide)
		// a token's gorge name is its script's face name ("Goblin"), the
		// table's is the script stem ("r_1_1_goblin")
		for i := range tbl.Cards {
			if tbl.Cards[i].Types&kop.TToken != 0 && m.reg != nil {
				if tc, ok := m.reg.Token(tbl.Cards[i].Name); ok {
					r.gname[i] = cardName(tc)
				}
			}
		}
		// a token's other face shares its card's gorge name (the front
		// face's): gorge keeps one token card and flips FaceIdx, while the
		// kernel turns the object onto the face's CardID.
		for i := range tbl.Cards {
			if f := tbl.Cards[i].Faces; len(f) > 0 && int(f[0]) < len(r.gname) {
				r.gname[i] = r.gname[f[0]]
			}
		}
		// a decayed token's Attacks trigger is gorge's keyword trigger
		// (lower_decayed.go): a stack object with no face index
		decayed := make([]bool, len(tbl.Cards))
		for i := range tbl.Cards {
			if tbl.Cards[i].Types&kop.TToken == 0 || m.reg == nil {
				continue
			}
			if tc, ok := m.reg.Token(tbl.Cards[i].Name); ok && len(tc.Faces) > 0 && tc.Faces[0] != nil {
				for _, k := range tc.Faces[0].Keywords {
					decayed[i] = decayed[i] || k == "Decayed"
				}
			}
		}
		r.generated = true
		// a kernel-native ward trigger is gorge's Ward keyword trigger
		ward := make([]uint8, len(tbl.Cards))
		for i := range tbl.Cards {
			ward[i] = 255
			if m.reg == nil {
				continue
			}
			if gc, ok := m.reg.Lookup(tbl.Cards[i].Name); ok && len(gc.Faces) > 0 && gc.Faces[0] != nil {
				for j, tr := range gc.Faces[0].Triggers {
					if tr.Params["Keyword"] == "Ward" {
						ward[i] = 0x80 | uint8(j)
					}
				}
			}
		}
		r.origin = func(c dirac.CardID, ab uint8) (uint8, bool) {
			if ab == 253 { // kernel abWard
				return ward[c], ward[c] != 255
			}
			if ab >= 253 || int(c) >= len(pool) || pool[c] < 0 {
				return 0, false
			}
			if abs := tbl.Cards[c].Abilities; int(ab) < len(abs) && abs[ab].AFlags&(kop.AfLatent|kop.AfStation) != 0 {
				// a Station ability: gorge's minted body has no face index
				return 255, true
			}
			if decayed[c] && int(ab) < len(tbl.Cards[c].Abilities) && tbl.Cards[c].Abilities[ab].Kind == kop.AbTriggered {
				return 255, true
			}
			o := set.AbilityOrigin(pool[c], int(ab))
			return o, o != 0xFF
		}
		return r, nil
	}
}

// FDNSide is SetSide over the "fdn" set.
func FDNSide(tbl *kop.Table, cfg Config) SideFactory { return SetSide(fdnSet(), tbl, cfg) }
