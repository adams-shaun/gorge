package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// maxEmbCold is how many emblems past the first maxEmb each player keeps in
// the cold buffer, for a table whose emblems can be made again and again
// (genPrep.anyEmbCold): a planeswalker's loyalty ability makes one a turn.
const maxEmbCold = 4

// coldEmb is every player's records past maxEmb, for a table that needs them.
var coldEmb = newCold[embRec]("emb", func(g *genPrep) int {
	if g.anyEmbCold {
		return MaxSeats * maxEmbCold
	}
	return 0
})

// maxEmbTotal is the most emblems one player can hold.
const maxEmbTotal = maxEmb + maxEmbCold

// embRec is one emblem past maxEmb, the same fields genState keeps for the
// first maxEmb: the Emblems index+1, the creating card and its object, the
// frozen chosen number, the layer timestamp and the remembered slots 1..3
// (object + 1; static_emblemrem.go).
type embRec struct {
	idx uint8
	obj Obj
	num objWord
	rem [embRemSlots - 1]objWord
	c   CardID
	t   uint16
}

// tableEmblemRepeats reports whether some activated or triggered ability of t
// creates an emblem, so one game can make more than maxEmb of them.
func tableEmblemRepeats(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Kind == kop.AbSpell || a.Kind == kop.AbStatic {
				continue
			}
			for _, in := range a.Prog {
				if in.Op == kop.OpCreateEmblem {
					return true
				}
			}
		}
	}
	return false
}

// embCap is how many emblems one player can hold in s.
func (s *State) embCap() int {
	return maxEmb + coldEmb.count(s.cold.layout)/MaxSeats
}

// embCold is the cold records: player p's are [p*maxEmbCold:], in creation
// order after the first maxEmb.
func (s *State) embCold() []embRec { return coldEmb.of(s) }

// embRecAt is player p's emblem i past maxEmb.
func (s *State) embRecAt(p uint8, i int) *embRec {
	return &s.embCold()[int(p)*maxEmbCold+i-maxEmb]
}

func (s *State) embIdx(p uint8, i int) uint8 {
	if i < maxEmb {
		return s.g.emb[p][i]
	}
	return s.embRecAt(p, i).idx
}

func (s *State) embCard(p uint8, i int) CardID {
	if i < maxEmb {
		return s.g.embC[p][i]
	}
	return s.embRecAt(p, i).c
}

func (s *State) embObj(p uint8, i int) Obj {
	if i < maxEmb {
		return s.g.embO[p][i]
	}
	return s.embRecAt(p, i).obj
}

func (s *State) embNum(p uint8, i int) objWord {
	if i < maxEmb {
		return s.g.embN[p][i]
	}
	return s.embRecAt(p, i).num
}

func (s *State) embStamp(p uint8, i int) uint16 {
	if i < maxEmb {
		return s.g.embT[p][i]
	}
	return s.embRecAt(p, i).t
}

// embGet is player p's emblem i as a record.
func (s *State) embGet(p uint8, i int) embRec {
	r := embRec{idx: s.embIdx(p, i), obj: s.embObj(p, i), num: s.embNum(p, i), c: s.embCard(p, i), t: s.embStamp(p, i)}
	for j := range r.rem {
		r.rem[j] = s.embRemGet(p, i, j+1)
	}
	return r
}

// embPut sets player p's emblem i.
func (s *State) embPut(p uint8, i int, r embRec) {
	if i < maxEmb {
		s.g.emb[p][i], s.g.embC[p][i], s.g.embO[p][i], s.g.embN[p][i], s.g.embT[p][i] = r.idx, r.c, r.obj, r.num, r.t
		for j, v := range r.rem {
			s.embRemSet(p, i, j+1, v)
		}
		return
	}
	*s.embRecAt(p, i) = r
}

// embSetNum sets the frozen number of player p's emblem i.
func (s *State) embSetNum(p uint8, i int, n objWord) {
	if i < maxEmb {
		s.g.embN[p][i] = n
		return
	}
	s.embRecAt(p, i).num = n
}

// embColdLive is player p's records past maxEmb that are in use.
func (s *State) embColdLive(p uint8) []embRec {
	n := int(s.g.nemb[p]) - maxEmb
	if n <= 0 {
		return nil
	}
	return s.embCold()[int(p)*maxEmbCold:][:n]
}
