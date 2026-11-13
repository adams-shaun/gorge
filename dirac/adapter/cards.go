package adapter

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// card is table card c's compiled card, nil when the corpus has none.
func (g *Game) card(c dirac.CardID) *cards.Card {
	if int(c) >= len(g.tbl.Cards) || g.eng.reg == nil {
		return nil
	}
	if gc, ok := g.eng.reg.Lookup(g.tr.GName[c]); ok {
		return gc
	}
	if tc, ok := g.eng.reg.Token(g.tbl.Cards[c].Name); ok {
		return tc
	}
	return nil
}

// face is the face table card c plays as.
func (g *Game) face(c dirac.CardID) *cards.Face {
	gc := g.card(c)
	if gc == nil || len(gc.Faces) == 0 {
		return nil
	}
	if i := int(g.tbl.Cards[c].Face); i < len(gc.Faces) {
		return gc.Faces[i]
	}
	return gc.Faces[0]
}

// faceSA is the spell ability a face index names on table card c.
func (g *Game) faceSA(c dirac.CardID, gi uint8) *cards.SA {
	f := g.face(c)
	if f == nil {
		return nil
	}
	return canon.FaceSA(f, gi)
}

// grantedSA is activated ability ab of card c, which c's statics grant to another
// permanent, and the SVar that names it: the kernel lists them on the card in the
// order the statics do.
func (g *Game) grantedSA(c dirac.CardID, ab uint8) (string, *cards.SA) {
	f := g.face(c)
	if f == nil || int(ab) >= len(g.tbl.Cards[c].Abilities) {
		return "", nil
	}
	n := 0
	for _, a := range g.tbl.Cards[c].Abilities[:ab] {
		if a.Kind == kop.AbActivated && a.AFlags&kop.AfLatent != 0 {
			n++
		}
	}
	for i := range f.Statics {
		for _, name := range strings.Split(f.Statics[i].ParamStr(cards.PKAddAbility), " & ") {
			sa := cards.ResolveSVar(f.SVars, name)
			if sa == nil || sa.Kind != "AB" || cards.IsManaAbilitySA(sa) {
				continue
			}
			if n == 0 {
				return name, sa
			}
			n--
		}
	}
	return "", nil
}

// cardName is a kernel object's display name.
func (g *Game) cardName(o canon.Ref) string {
	if int(o) >= int(g.kp.NObj) {
		return "object"
	}
	c := g.kp.Objs[o].Card
	if int(c) >= len(g.tr.GName) || g.tr.GName[c] == "" {
		return g.tbl.Cards[c].Name
	}
	return strings.TrimPrefix(g.tr.GName[c], "name:")
}

// stackName names the stack entry at position i.
func (g *Game) stackName(i int) string {
	if i >= int(g.kp.NStack) {
		return "ability"
	}
	if c := g.kp.Stack[i].Card; int(c) < len(g.tr.GName) {
		return g.tr.GName[c]
	}
	return "ability"
}
