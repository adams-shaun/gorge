package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

func TestTriggerObjectsCardsCountProperties(t *testing.T) {
	h := newHost(t, 2)
	creature := mkCard(t, "Name:Creature\nManaCost:3\nTypes:Creature Beast\nPT:4/4\nOracle:x\n")
	artifact := mkCard(t, "Name:Artifact\nManaCost:5\nTypes:Artifact\nOracle:x\n")
	creatureObj := h.g.AddObject(creature, 0)
	artifactObj := h.g.AddObject(artifact, 0)
	c := &Ctx{Controller: 0, Captured: []state.Target{{Obj: creatureObj.ID}, {Obj: artifactObj.ID}}}
	if len(c.Captured) != 2 || h.g.Obj(creatureObj.ID).Face().Cmc() == h.g.Obj(artifactObj.ID).Face().Cmc() {
		t.Fatal("precondition: batch needs two differently-costed objects")
	}
	for _, tc := range []struct {
		expr string
		want int32
	}{
		{"TriggerObjectsCards$CardTypes", 3},
		{"TriggerObjectsCards$GreatestCardManaCost", 5},
		{"TriggerObjectsCards$CardPower", 4},
	} {
		if got, ok := EvalCountOK(h, c, tc.expr); !ok || got != tc.want {
			t.Errorf("EvalCountOK(%s) = (%d, %v), want (%d, true)", tc.expr, got, ok, tc.want)
		}
	}
}
