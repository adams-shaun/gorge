package lab

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac"
)

// Describe is a readable line for action a at the pending decision: its kind and
// the cards, players and values it names. An action it has no wording for prints
// its kind and raw fields.
func (g *Game) Describe(a dirac.Action) string {
	src, tgt := g.code(a.Source()), g.code(a.Target())
	switch a.Kind() {
	case dirac.AkPass:
		return "pass"
	case dirac.AkPlayLand:
		return "play land " + src
	case dirac.AkCast:
		return "cast " + src + onTarget(tgt)
	case dirac.AkActivate:
		return fmt.Sprintf("activate %s ability %d%s", src, a.Mode(), onTarget(tgt))
	case dirac.AkAttack:
		return "attack with " + src
	case dirac.AkBlock:
		return "block " + tgt + " with " + src
	case dirac.AkDone:
		return "done"
	case dirac.AkTarget:
		return "target " + tgt
	case dirac.AkMode:
		return fmt.Sprintf("mode %d", a.Mode())
	case dirac.AkChoose:
		if a.Source() < dirac.MaxObj {
			return "choose " + src
		}
		return fmt.Sprintf("choose %d", a.Mode())
	case dirac.AkKeep:
		return "keep hand"
	case dirac.AkMulligan:
		return "mulligan"
	}
	return fmt.Sprintf("kind %d src %s tgt %s mode %d", a.Kind(), src, tgt, a.Mode())
}

func onTarget(t string) string {
	if t == "-" {
		return ""
	}
	return " targeting " + t
}

// code names a source or target byte: a card, a stack ability, a seat or "-".
func (g *Game) code(c dirac.TCode) string {
	switch {
	case dirac.IsObjTarget(c):
		if int(c) >= g.s.NumObjs() {
			return fmt.Sprintf("obj%d", c)
		}
		return fmt.Sprintf("%s#%d", g.su.tbl.Cards[g.s.ObjCard(dirac.Obj(c))].Name, c)
	case c >= dirac.PlayerTarget0 && int(c-dirac.PlayerTarget0) < dirac.MaxSeats:
		return fmt.Sprintf("seat %d", c-dirac.PlayerTarget0)
	case c >= dirac.StackTarget0 && c < dirac.PlayerTarget0:
		return fmt.Sprintf("stack[%d]", c-dirac.StackTarget0)
	}
	return "-"
}
