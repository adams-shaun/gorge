package canon

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// BindSet points r at tbl, a table built from set: each card's corpus name, its
// abilities' face indices through set.AbilityOrigin, and the ChGen* asks.
func (r *Translator) BindSet(set cardset.Set, tbl *kop.Table, reg *cards.Registry) {
	r.Tbl, r.Reg, r.Generated = tbl, reg, true
	r.GName = make([]string, len(tbl.Cards))
	pool := make([]int, len(tbl.Cards))
	var names []string
	for i := range tbl.Cards {
		cd := &tbl.Cards[i]
		r.GName[i] = cd.Name
		if len(cd.Faces) > 0 {
			r.GName[i] = tbl.Cards[cd.Faces[0]].Name // a face is its card: gorge names the front
		}
		pool[i] = -1
		if j, ok := set.Lookup(cd.Name); ok {
			pool[i] = j
			continue
		}
		// a multi-face card's other face or a token is a pool entry beside the
		// deck cards: its abilities are that face's
		if names == nil {
			names = set.Names()
		}
		for j, n := range names {
			if n == cd.Name {
				pool[i] = j
			}
		}
	}
	// a token's gorge name is its script's face name ("Goblin"), the table's is
	// the script stem ("r_1_1_goblin"); its other face shares the front's name,
	// since gorge keeps one token card and flips FaceIdx while the kernel turns
	// the object onto the face's CardID
	decayed := make([]bool, len(tbl.Cards))
	ward := make([]uint8, len(tbl.Cards))
	for i := range tbl.Cards {
		ward[i] = 255
		if n, ok := strings.CutPrefix(tbl.Cards[i].Name, kop.CopyCardKeyPrefix); ok {
			r.GName[i] = n // a named-card copy's entry (kop.ExtCopyNamed) is the corpus card of that name
		}
		if reg == nil {
			continue
		}
		if tbl.Cards[i].Types&kop.TToken != 0 {
			if tc, ok := reg.Token(tbl.Cards[i].Name); ok {
				r.GName[i] = frontName(tc)
				// a decayed token's Attacks trigger is gorge's keyword trigger: a
				// stack object with no face index
				if len(tc.Faces) > 0 && tc.Faces[0] != nil {
					for _, k := range tc.Faces[0].Keywords {
						decayed[i] = decayed[i] || k == "Decayed"
					}
				}
			}
		}
		// a kernel-native ward trigger is gorge's Ward keyword trigger
		if gc, ok := reg.Lookup(tbl.Cards[i].Name); ok && len(gc.Faces) > 0 && gc.Faces[0] != nil {
			for j, tr := range gc.Faces[0].Triggers {
				if tr.Params["Keyword"] == "Ward" {
					ward[i] = 0x80 | uint8(j)
				}
			}
		}
	}
	for i := range tbl.Cards {
		if f := tbl.Cards[i].Faces; len(f) > 0 && int(f[0]) < len(r.GName) {
			r.GName[i] = r.GName[f[0]]
		}
	}
	r.Origin = func(c dirac.CardID, ab uint8) (uint8, bool) {
		if ab == 253 { // the kernel's native ward trigger
			return ward[c], ward[c] != 255
		}
		if ab >= 253 || int(c) >= len(pool) || pool[c] < 0 {
			return 0, false
		}
		if abs := tbl.Cards[c].Abilities; int(ab) < len(abs) && abs[ab].AFlags&(kop.AfLatent|kop.AfStation) != 0 {
			return 255, true // a Station ability: gorge's minted body has no face index
		}
		if decayed[c] && int(ab) < len(tbl.Cards[c].Abilities) && tbl.Cards[c].Abilities[ab].Kind == kop.AbTriggered {
			return 255, true
		}
		o := set.AbilityOrigin(pool[c], int(ab))
		if abs := tbl.Cards[c].Abilities; o == 0xFF && int(ab) < len(abs) && abs[ab].Kind == kop.AbTriggered && abs[ab].Trig.Flags2&kop.Tf2ManaFromSelf != 0 {
			return 255, true // a TriggersWhenSpent$ rider (Pyromancer's Goggles): gorge queues it as a granted trigger, no face index
		}
		return o, o != 0xFF
	}
}

func frontName(c *cards.Card) string {
	if c == nil {
		return ""
	}
	if len(c.Faces) > 0 && c.Faces[0] != nil && c.Faces[0].Name != "" {
		return c.Faces[0].Name
	}
	return c.Path
}
