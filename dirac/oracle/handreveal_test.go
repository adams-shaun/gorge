package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: Telepathy's view-only hand-reveal MayLookAt$ static
// (AffectedZone$ Hand, MayLookAt$ Player over the opponents' cards). gorge
// registers the look and no game-state reader consumes it; kgen lowers it to
// no static (dirac/kgen/lower_handreveal.go). Grizzly Bears is the vanilla
// body, the basics pay both.
var khandrevealDeck = []cardset.Entry{
	{Name: "Telepathy", Count: 6},
	{Name: "Grizzly Bears", Count: 10},
	{Name: "Island", Count: 12},
	{Name: "Forest", Count: 12},
}

// khandrevealCounts: casts of Telepathy chosen, decisions taken while a
// Telepathy is on gorge's battlefield and the other seat holds cards, and
// decisions where the library-top look read answered true.
type khandrevealCounts struct {
	casts, liveDecisions, libraryTopLooks int
}

// khandrevealLive reports whether gorge has a Telepathy on the battlefield
// whose controller's opponent holds at least one card.
func khandrevealLive(g *GorgeSide) bool {
	for p := state.PlayerID(0); p < 2; p++ {
		for _, id := range g.E.G.Zone(state.ZBattlefield, p) {
			o := g.E.G.Obj(id)
			if o == nil || o.Face() == nil || o.Face().Name != "Telepathy" {
				continue
			}
			if len(g.E.G.Zone(state.ZHand, 1-p)) > 0 {
				return true
			}
		}
	}
	return false
}

// khandrevealPolicy casts Telepathy first, then any cast/land/attack/block,
// falling back to the seeded random policy. At each decision it counts the
// live reveal and gorge's library-top look read, which the hand-reveal grant
// must not satisfy.
func khandrevealPolicy(seed uint64, c *khandrevealCounts) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(ctx *PickCtx) Canon {
		if ctx.G != nil && khandrevealLive(ctx.G) {
			c.liveDecisions++
			for p := state.PlayerID(0); p < 2; p++ {
				if ctx.G.E.MayLookAtLibraryTop(p) {
					c.libraryTopLooks++
				}
			}
		}
		buf = buf[:0]
		for _, a := range ctx.D.Options {
			switch a.Kind() {
			case ActCast:
				if kfacedownName(ctx, a) == "Telepathy" {
					c.casts++
					return a
				}
				buf = append(buf, a)
			case ActPlayLand, ActAttack, ActBlock:
				buf = append(buf, a)
			}
		}
		if len(buf) > 0 {
			d := *ctx.D
			d.Options = buf
			return rnd.Pick(&PickCtx{D: &d, G: ctx.G, Seq: ctx.Seq})
		}
		return rnd.Pick(ctx)
	})
}

func TestHandrevealLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("handreveal")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s := range decks {
		for _, e := range khandrevealDeck {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(khandrevealDeck, khandrevealDeck)
	if err != nil {
		t.Fatal(err)
	}

	var counts khandrevealCounts
	seeds := uint64(48)
	if testing.Short() {
		seeds = 8
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kh-a", "kh-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, khandrevealPolicy(seed, &counts), RunConfig{
			Config: cfg, MaxDecisions: 2000, MaxTurns: 20,
		})
		CloseSide(k)
		if d := res.Div; d != nil {
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Errorf("seed %d: kernel stalled: %s", seed, res.Stall)
		}
	}
	t.Logf("%d seeds: Telepathy casts %d, decisions with a live reveal %d, library-top looks %d",
		seeds, counts.casts, counts.liveDecisions, counts.libraryTopLooks)
	if uint64(counts.casts) < seeds {
		t.Errorf("Telepathy cast %d times over %d seeds, want at least one per seed", counts.casts, seeds)
	}
	if counts.liveDecisions < 100 {
		t.Errorf("%d decisions with a Telepathy live over a non-empty hand, want at least 100", counts.liveDecisions)
	}
	if counts.libraryTopLooks != 0 {
		t.Errorf("gorge's library-top look answered true %d times: the hand-reveal grant reached a library reader", counts.libraryTopLooks)
	}
}
