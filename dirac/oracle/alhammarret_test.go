package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Alhammarret, High Arbiter lockstep: "as it enters, each opponent reveals
// their hand; you choose the name of a nonland card revealed this way; your
// opponents can't cast spells with the chosen name" (ETB RevealHand
// Player.Opponent | RememberRevealed, NameCard ChooseFromDefinedCards$
// Remembered, and S:Mode$ CantBeCast | ValidCard$ Card.NamedCard; kernel
// ExtChooseName B = 3 and StatCantBeCast B&CantCastNamed). The name ask lists
// only the nonland cards in the opponent's hand, and none is posed when there
// are none. The policy names a card the opponent plays and casts before
// anything else, so the withheld cast offers show.

var alhammarretDecks = [2][]cardset.Entry{
	{
		{Name: "Alhammarret, High Arbiter", Count: 4}, {Name: "Mind Stone", Count: 4},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Prodigal Sorcerer", Count: 4}, {Name: "Island", Count: 20},
	},
	{
		{Name: "Alhammarret, High Arbiter", Count: 4}, {Name: "Mind Stone", Count: 4},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Prodigal Sorcerer", Count: 4}, {Name: "Swamp", Count: 20},
	},
}

// alhammarretRun counts what a run over the decks saw: games played, games in
// which an Alhammarret named a card, name asks posed and games in which one
// was on the battlefield.
type alhammarretRun struct{ games, named, asks, entered, withheld int }

func runAlhammarret(t *testing.T, dl [2][]cardset.Entry, seeds int) alhammarretRun {
	t.Helper()
	reg, set, decks, tbl := p3Setup(t, "alhammarret", dl)
	if len(tbl.NameCards) == 0 {
		t.Fatal("precondition: the matchup table lists no nameable cards")
	}
	var oppNames [2]map[uint16]bool
	for s := range oppNames {
		oppNames[s] = map[uint16]bool{}
	}
	for s := range decks {
		for _, c := range decks[s] {
			if !c.Faces[0].IsLand() {
				oppNames[1-s][canon.NameValue(cardName(c))] = true
			}
		}
	}
	oldC, oldH := dirac.DebugCantCastNamedCount, dirac.CantCastNamedHeld
	dirac.DebugCantCastNamedCount, dirac.CantCastNamedHeld = true, 0
	defer func() { dirac.DebugCantCastNamedCount, dirac.CantCastNamedHeld = oldC, oldH }()
	nameKind := ActChoose + CanonKind(mustChooseCode("name"))
	var run alhammarretRun
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"alhammarret-a", "alhammarret-b"}, decks, 1)
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
		inner := kkaiwinnPolicy(seed, oppNames)
		pol := PolicyFunc(func(c *PickCtx) Canon {
			for _, a := range c.D.Options {
				if a.Kind() == nameKind {
					run.asks++
					break
				}
			}
			return inner.Pick(c)
		})
		res := LockstepLed(m, seed, g, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
		CloseSide(k)
		run.games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: %s", seed, res.Stall)
		}
		for _, o := range g.E.G.Objs {
			if o.Zone == state.ZBattlefield && o.Face().Name == "Alhammarret, High Arbiter" {
				run.entered++
			}
			if o.ChosenName != "" {
				run.named++
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
	run.withheld = dirac.CantCastNamedHeld
	return run
}

func TestAlhammarretLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	run := runAlhammarret(t, alhammarretDecks, 40)
	if run.named == 0 || run.asks == 0 {
		t.Fatalf("precondition: %d games named a card, %d name asks posed over %d games", run.named, run.asks, run.games)
	}
	if run.withheld == 0 {
		t.Fatalf("precondition: no cast was withheld over %d games (%d named)", run.games, run.named)
	}
}

// TestAlhammarretEmptyRevealLockstep: the opponent holds nothing but lands, so
// the reveal shows no nonland card and gorge asks for no name; the kernel poses
// none either, and the Alhammarret stays unnamed.
func TestAlhammarretEmptyRevealLockstep(t *testing.T) {
	dl := alhammarretDecks
	dl[1] = []cardset.Entry{{Name: "Swamp", Count: 40}}
	run := runAlhammarret(t, dl, 20)
	if run.entered == 0 {
		t.Fatalf("precondition: no Alhammarret entered over %d games", run.games)
	}
	if run.named != 0 || run.asks != 0 {
		t.Fatalf("%d games named a card, %d name asks posed against an all-land hand", run.named, run.asks)
	}
}
