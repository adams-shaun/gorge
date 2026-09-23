package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

func TestCantAttackTargetPlaneswalkerController(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	vow, ok := reg.Lookup("Vow of Lightning")
	if !ok {
		t.Fatal("corpus missing Vow of Lightning")
	}
	var target, walkerTarget string
	for _, line := range vow.Faces[0].Statics {
		if line.Mode != "CantAttack" {
			continue
		}
		target = line.Params["Target"]
		for _, part := range strings.Split(target, ",") {
			if strings.HasPrefix(strings.TrimSpace(part), "Planeswalker.") {
				walkerTarget = strings.TrimSpace(part)
				break
			}
		}
		break
	}
	if walkerTarget != "Planeswalker.YouCtrl" {
		t.Fatalf("precondition: Vow of Lightning Target$ = %q, want its corpus Planeswalker.YouCtrl clause", target)
	}
	walker := card(t, "Name:Target Walker\nTypes:Planeswalker\nPT:0\nOracle:x\n")
	bear := card(t, staticBearFixture)
	e, _ := restrictionGame(t, 6130,
		[][]*cards.Card{{vow}, nil},
		[][]*cards.Card{{walker}, {bear}})
	walkerID := bearOnBoard(t, e, 0, walker)
	walkerObj := e.G.Obj(walkerID)
	if walkerObj == nil || walkerObj.Zone != state.ZBattlefield || !faceHasType(walkerObj, "Planeswalker") {
		t.Fatalf("precondition: Target Walker is not a battlefield planeswalker: %+v", walkerObj)
	}
	bearID := bearOnBoard(t, e, 1, bear)
	if e.G.Obj(bearID).Controller == walkerObj.Controller {
		t.Fatal("precondition: attacker and planeswalker controller must differ")
	}
	if !restrictionPlayerTargetMatches(e.G, walkerTarget, walkerObj.Controller, 0, nil) {
		t.Fatal("Planeswalker.YouCtrl did not match the controller's battlefield planeswalker")
	}
	if restrictionPlayerTargetMatches(e.G, walkerTarget, 1, 0, nil) {
		t.Fatal("Planeswalker.YouCtrl matched a defender who does not control the walker's target")
	}
}
