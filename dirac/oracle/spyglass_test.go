package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Sorcerous Spyglass lockstep: "as Spyglass enters, look at an opponent's
// hand, then choose any card name; activated abilities of sources with the
// chosen name can't be activated unless they're mana abilities" (ETB
// ChoosePlayer / RevealHand Look$ / NameCard with no ValidCards$, and S:Mode$
// CantBeActivated | ValidCard$ Card.NamedCard; kernel ExtChooseName B = 2 and
// StatCantActivate A&CantActNamed). Each seat names the other's Mind Stone (a
// mana ability and a {1},{T}, sacrifice draw): the opponent keeps the mana
// ability and loses the draw.

var spyglassDecks = [2][]cardset.Entry{
	{
		{Name: "Sorcerous Spyglass", Count: 4}, {Name: "Mind Stone", Count: 4},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Prodigal Sorcerer", Count: 4}, {Name: "Island", Count: 20},
	},
	{
		{Name: "Sorcerous Spyglass", Count: 4}, {Name: "Mind Stone", Count: 4},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Prodigal Sorcerer", Count: 4}, {Name: "Swamp", Count: 20},
	},
}

func TestSorcerousSpyglassLockstep(t *testing.T) {
	reg, set, decks, tbl := p3Setup(t, "spyglass", spyglassDecks)
	if len(tbl.NameCardsAny) <= len(tbl.NameCards) {
		t.Fatalf("precondition: the any-card name list (%d) must hold the lands beside the nonland list (%d)", len(tbl.NameCardsAny), len(tbl.NameCards))
	}
	var oppNames [2]map[uint16]bool
	for s := range oppNames {
		oppNames[s] = map[uint16]bool{canon.NameValue("Mind Stone"): true}
	}
	oldC, oldH, oldM := dirac.DebugCantActCount, dirac.CantActHeld, dirac.CantActManaHeld
	dirac.DebugCantActCount, dirac.CantActHeld, dirac.CantActManaHeld = true, 0, 0
	defer func() { dirac.DebugCantActCount, dirac.CantActHeld, dirac.CantActManaHeld = oldC, oldH, oldM }()
	games, named := 0, 0
	for seed := uint64(1); seed <= 30; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"spyglass-a", "spyglass-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, kkaiwinnPolicy(seed, oppNames), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 25})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: %s", seed, res.Stall)
		}
		for _, o := range g.E.G.Objs {
			if o.ChosenName == "Mind Stone" {
				named++
				break
			}
		}
		if d := res.Div; d != nil {
			tail := res.Actions
			if len(tail) > 6 {
				tail = tail[len(tail)-6:]
			}
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nlast %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, tail)
		}
	}
	if named == 0 {
		t.Fatal("precondition: no game named Mind Stone")
	}
	if dirac.CantActHeld == 0 {
		t.Fatalf("precondition: no activation was forbidden over %d games (%d named)", games, named)
	}
	if dirac.CantActManaHeld != 0 {
		t.Fatalf("a mana ability was forbidden %d times", dirac.CantActManaHeld)
	}
}
