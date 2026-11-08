package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

func TestZZScratch(t *testing.T) {
	e, _ := raiseEngine(t, []string{"Champions of the Perfect", "Selfless Safewright", "Safewright Cavalry", "Springleaf Drum", "Forest", "Grizzly Bears"}, nil)
	champ := blightMove(t, e, 0, "Champions of the Perfect", state.ZHand)
	blightMove(t, e, 0, "Selfless Safewright", state.ZHand)
	blightMove(t, e, 0, "Safewright Cavalry", state.ZBattlefield)
	blightMove(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	blightMove(t, e, 0, "Springleaf Drum", state.ZBattlefield)
	blightMove(t, e, 0, "Forest", state.ZBattlefield)
	blightMove(t, e, 0, "Forest", state.ZBattlefield)
	blightMove(t, e, 0, "Forest", state.ZBattlefield)
	t.Log("3 forests+drum:", raiseCastOffered(e, champ))
	addMana(t, e, 0, "GGGG")
	t.Log("floating GGGG:", raiseCastOffered(e, champ))
}
