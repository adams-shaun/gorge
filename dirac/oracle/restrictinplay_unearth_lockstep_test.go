package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// restrictInPlayDecks: a W/U deck whose black comes from Vedalken Engineer
// (and a few Swamps, so the unearth is sometimes honestly payable) beside
// Scrapwork Rager, against burn that sends the Ragers to the graveyard.
var restrictInPlayDecks = [2][]cardset.Entry{{
	{Name: "Vedalken Engineer", Count: 8}, {Name: "Scrapwork Rager", Count: 8},
	{Name: "Plains", Count: 10}, {Name: "Island", Count: 10}, {Name: "Swamp", Count: 4},
}, {
	{Name: "Grizzly Bears", Count: 8}, {Name: "Lightning Bolt", Count: 8}, {Name: "Shock", Count: 8},
	{Name: "Mountain", Count: 16},
}}

// restrictInPlayExposed: at c, a priority decision of the gorge side, the
// actor has a Scrapwork Rager in its graveyard and an untapped Vedalken
// Engineer whose two mana would complete {3}{B} that its other untapped
// lands cannot pay alone: the board where counting the Engineer's
// battlefield-only mana toward the unearth changes the offer.
func restrictInPlayExposed(c *PickCtx) bool {
	g := c.G.E.G
	p := state.PlayerID(c.D.Player)
	rager, eng, lands, swamp := false, false, 0, false
	for i := range g.Objs {
		ob := &g.Objs[i]
		if ob.Face() == nil {
			continue
		}
		switch {
		case ob.Zone == state.ZGraveyard && ob.Owner == p && ob.Face().Name == "Scrapwork Rager":
			rager = true
		case ob.Zone != state.ZBattlefield || ob.Controller != p || ob.Tapped:
		case ob.Face().Name == "Vedalken Engineer" && !ob.SummonSick:
			eng = true
		case ob.Face().IsLand():
			lands++
			swamp = swamp || ob.Face().Name == "Swamp"
		}
	}
	return rager && eng && lands >= 2 && (lands < 4 || !swamp)
}

// TestEngineerManaGraveUnearthLockstep: Vedalken Engineer's mana pays only
// abilities of artifacts on the battlefield (RestrictValid$
// Activated.Artifact+inZoneBattlefield, CR 106.6), so neither engine offers
// Scrapwork Rager's graveyard unearth on it; the games must reach that board.
func TestEngineerManaGraveUnearthLockstep(t *testing.T) {
	t.Parallel()
	reg, _ := kcastSetup(t)
	set, err := cardset.RawByName("restrictinplay")
	if err != nil {
		t.Fatal(err)
	}
	decks := kcastDeck(t, reg, restrictInPlayDecks)
	tbl, _, _, err := set.Matchup(restrictInPlayDecks[0], restrictInPlayDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	played := map[string]int{}
	exposed, divs := 0, 0
	for seed := uint64(1); seed <= 32; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"rip-a", "rip-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		inner := eagerPolicy(seed, played)
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil && c.D.Kind == DPriority && restrictInPlayExposed(c) {
				exposed++
			}
			return inner.Pick(c)
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		if res.Div != nil {
			divs++
			if divs <= 4 {
				t.Errorf("seed %d: diverged at %d: %s %s %v\n gorge %v\n kernel %v\n %s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail)
			}
		}
	}
	t.Logf("%d exposed priority decisions, %d divergent games, plays %v", exposed, divs, played)
	if exposed == 0 {
		t.Fatal("no game reached a Rager in the graveyard beside an untapped Engineer short of plain mana")
	}
	if played["Scrapwork Rager"] == 0 {
		t.Fatal("no Scrapwork Rager was cast or unearthed")
	}
}
