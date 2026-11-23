package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The granted-embalm lockstep (CR 702.128a): Cursecloth Wrappings' pump gives
// a creature card in its owner's graveyard "embalm until end of turn" with the
// embalm cost equal to that card's mana cost. The pool is
// dirac/cardset/pools/fixtures/expandedkw4granted: Wrappings beside cheap white
// creatures (printing no embalm) against burn that puts them in the graveyard.
// The lockstep compares the two sides' option lists at every decision, so the
// price gorge offers the granted ability at is the one the kernel charges; the
// count of copy tokens of a Wrappings deck creature proves the grant was
// activated and paid, so a green game is not one where it never fired.

const grantedEmbalmPool = "expandedkw4granted"

var grantedEmbalmDecks = [2][]cardset.Entry{
	{
		{Name: "Cursecloth Wrappings", Count: 8}, {Name: "Elite Vanguard", Count: 8}, {Name: "Savannah Lions", Count: 6},
		{Name: "Plains", Count: 9}, {Name: "Swamp", Count: 9},
	},
	{
		{Name: "Lightning Bolt", Count: 8}, {Name: "Hill Giant", Count: 8}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Mountain", Count: 12}, {Name: "Forest", Count: 8},
	},
}

// grantedEmbalmTokens counts the copy tokens of the Wrappings deck's creatures
// that entered gorge's battlefield: the deck has no other token maker.
func grantedEmbalmTokens(g *GorgeSide) (n int) {
	for _, e := range g.E.L.Events {
		if e.Kind != events.MoveZone || e.To != state.ZBattlefield {
			continue
		}
		o := g.E.G.Obj(e.Obj)
		if o != nil && o.IsToken && o.Face() != nil && (o.Face().Name == "Elite Vanguard" || o.Face().Name == "Savannah Lions") {
			n++
		}
	}
	return n
}

// TestExpandedKeywordsGrantedEmbalmLockstep: no divergence and no kernel stall
// over the seeds, and across them a granted embalm minted a token.
func TestExpandedKeywordsGrantedEmbalmLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	set, err := cardset.RawByName(grantedEmbalmPool)
	if err != nil {
		t.Fatal(err)
	}
	tbl, _, err := set.MatchupN([][]cardset.Entry{grantedEmbalmDecks[0], grantedEmbalmDecks[1]})
	if err != nil {
		t.Fatal(err)
	}
	p := kw4Pod{reg: reg, set: set, tbl: tbl}
	tokens := 0
	for _, seed := range []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12} {
		r := p.repro(grantedEmbalmDecks, seed)
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 8000, 40
		n := 0
		rc.Finish = func(g *GorgeSide) { n = grantedEmbalmTokens(g) }
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		lead, err := NewGorgeSide(m, seed, rc.Config, nil)
		if err != nil {
			CloseSide(k)
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, lead, k, castFirstPolicy(seed), rc)
		CloseSide(k)
		if res.Div != nil {
			kw4Fail(t, seed, res)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		tokens += n
		t.Logf("seed %d: %d decisions, over %v %s; %d token(s)", seed, res.Decisions, res.Over, res.Stall, n)
	}
	if tokens == 0 {
		t.Fatal("no granted embalm minted a token")
	}
}
