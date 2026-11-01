package adapter

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac"
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

// faceSA is the spell ability a face index names: Face.Abilities, Face.Triggers'
// effect (0x80|i), or the card's spell ability (255).
func (g *Game) faceSA(c dirac.CardID, gi uint8) *cards.SA {
	f := g.face(c)
	if f == nil {
		return nil
	}
	switch {
	case gi == 255:
		return f.SpellAbility()
	case gi&0x80 != 0:
		if i := int(gi &^ 0x80); i < len(f.Triggers) {
			return f.Triggers[i].Effect
		}
	case int(gi) < len(f.Abilities):
		return f.Abilities[gi]
	}
	return nil
}

// cardName is a kernel object's display name.
func (g *Game) cardName(o uint8) string {
	if int(o) >= int(g.kp.NObj) {
		return "object"
	}
	c := g.kp.Objs[o].Card
	if int(c) >= len(g.tr.GName) {
		return "object"
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
