package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The MBC keywords-2 family (fixture pool mbckw2): granted intimidate
// (Bellowing Tanglewurm's and Vow of Malice's statics, Hideous Visage's
// PumpAll), granted infect (Phyresis's static, Tainted Strike's and Vector
// Asp's Pump) and printed bloodthirst. The black-green deck holds every
// grant and no printed infect or intimidate, so a poison counter or an
// intimidate attacker can only come from a grant; the red deck shares no
// colour with it (intimidate restricts its blockers but Ornithopter, an
// artifact creature) and its burn and hasty attackers turn bloodthirst on.
var mbcKw2GrantDeck = []cardset.Entry{
	{Name: "Tainted Strike", Count: 4}, {Name: "Vector Asp", Count: 4},
	{Name: "Phyresis", Count: 3}, {Name: "Vow of Malice", Count: 3},
	{Name: "Bellowing Tanglewurm", Count: 3}, {Name: "Hideous Visage", Count: 3},
	{Name: "Grizzly Bears", Count: 4},
	{Name: "Swamp", Count: 8}, {Name: "Forest", Count: 8},
}

var mbcKw2RedDeck = []cardset.Entry{
	{Name: "Raging Goblin", Count: 4}, {Name: "Goblin Piker", Count: 4},
	{Name: "Shock", Count: 4}, {Name: "Ornithopter", Count: 4},
	{Name: "Raging Goblin", Count: 4}, {Name: "Goblin Piker", Count: 4},
	{Name: "Mountain", Count: 16},
}

// mbcKw2Cards are the family's carriers the games must cast.
var mbcKw2Cards = map[string]bool{
	"Tainted Strike": true, "Vector Asp": true, "Phyresis": true, "Vow of Malice": true,
	"Bellowing Tanglewurm": true, "Hideous Visage": true,
}

// TestMBCKeywords2Lowers plays the family's fixture pool in lockstep (both
// seatings, both option surfaces) and asserts each spelling was reached on
// the gorge side, so its decisions were compared: infect (a player got a
// poison counter: only a granted infect gives one here) and intimidate (a
// declare-blockers decision with an attacker gorge derives intimidate for,
// the defender holding a creature it restricts).
func TestMBCKeywords2Lowers(t *testing.T) {
	t.Parallel()
	matchups := []fixtureDecks{{mbcKw2GrantDeck, mbcKw2RedDeck}, {mbcKw2RedDeck, mbcKw2GrantDeck}}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mbckw2")
	if err != nil {
		t.Fatal(err)
	}
	poisoned, intimidating := 0, 0
	casts := map[string]int{}
	const seeds = 60
	for seed := uint64(1); seed <= seeds; seed++ {
		md := matchups[seed/2%2]
		var gd [2][]*cards.Card
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					gd[s] = append(gd[s], c)
				}
			}
			if len(gd[s]) != 40 {
				t.Fatalf("deck %d: %d cards, want 40", s, len(gd[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"mbckw2-a", "mbckw2-b"}, gd, 1)
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
		gamePoison, gameIntim := false, false
		res := LockstepLed(m, seed, g, k, fixtureWatchPolicy(seed, mbcKw2Cards, casts), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30,
			Check: func(ref *GorgeSide, _ Side) *Divergence {
				if ref.E == nil {
					return nil
				}
				gamePoison = gamePoison || mbcKw2Poisoned(ref.E.G)
				gameIntim = gameIntim || mbcKw2IntimidateBlock(ref)
				return nil
			}})
		CloseSide(k)
		if gamePoison {
			poisoned++
		}
		if gameIntim {
			intimidating++
		}
		if err := unmodelledStallError(t.Name(), &res); err != nil {
			t.Errorf("seed %d: %v", seed, err)
			continue
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Errorf("seed %d: kernel capacity stall %q", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	if poisoned == 0 {
		t.Error("precondition: infect: no player got a poison counter in any game, so no granted infect dealt damage")
	}
	if intimidating == 0 {
		t.Error("precondition: intimidate: no blocker declaration faced an attacker with a granted intimidate")
	}
	for n := range mbcKw2Cards {
		if casts[n] == 0 {
			t.Errorf("%s: never cast in %d seeds", n, seeds)
		}
	}
	t.Logf("mbckw2: %d games with poison, %d with an intimidate attacker facing blockers; casts %v", poisoned, intimidating, casts)
}

// mbcKw2Poisoned: some player has a poison counter.
func mbcKw2Poisoned(g *state.Game) bool {
	for i := range g.Players {
		for _, c := range g.Players[i].Counters {
			if strings.EqualFold(c.Kind, "poison") && c.N > 0 {
				return true
			}
		}
	}
	return false
}

// mbcKw2IntimidateBlock: blockers are being declared (an attacker is in
// combat, gorge's pending decision is the defender's), an attacker has
// intimidate, and its defender controls an untapped creature that is no
// artifact and shares no colour with it -- one intimidate forbids.
func mbcKw2IntimidateBlock(ref *GorgeSide) bool {
	e, g := ref.E, ref.E.G
	if d := ref.Pending(); d == nil || d.Kind != decision.KBlockers {
		return false
	}
	for _, id := range mbcKw2Battlefield(g) {
		a := g.Obj(id)
		if a == nil || !a.IsAttacking || !e.HasKeyword(id, "Intimidate") {
			continue
		}
		for _, bid := range g.Zone(state.ZBattlefield, a.Attacking) {
			b := g.Obj(bid)
			if b == nil || b.Tapped || !e.IsCreature(bid) || b.Face() != nil && b.Face().IsArtifact() {
				continue
			}
			if !strings.ContainsAny(e.Colors(bid), e.Colors(id)) {
				return true
			}
		}
	}
	return false
}

// mbcKw2Battlefield is every player's battlefield, in seat order.
func mbcKw2Battlefield(g *state.Game) []state.ObjID {
	var out []state.ObjID
	for i := range g.Players {
		out = append(out, g.Zone(state.ZBattlefield, state.PlayerID(i))...)
	}
	return out
}
