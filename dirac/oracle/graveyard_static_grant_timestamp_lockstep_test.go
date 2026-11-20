package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/state"
)

// A graveyard-functioning static (Anger's haste, CR 113.6) takes the timestamp
// its card got entering the graveyard (CR 613.7d), so a loses-all-abilities
// Aura (Reprobation) attached before Anger died does not remove the grant
// (CR 613.1f: the later effect wins). The kernel gave the graveyard source
// timestamp 0 and dropped the haste (dr-mbc-aaaa pair 25, decision 480).

func TestGraveyardStaticGrantTimestampReprosLockstep(t *testing.T) {
	corpusDirOrSkip(t)
	stdReproNamed(t, "graveyard_static_grant_timestamp")
}

// gyGrantDeck holds the Anger / Reprobation interaction: combat puts Anger in the graveyard, Reprobation sits on a creature before or
// after that.
func gyGrantDeck() []string {
	return deckOf(4, "Anger", deckOf(4, "Reprobation",
		deckOf(4, "Midnight Guard", deckOf(4, "Whipcorder",
			deckOf(14, "Mountain", deckOf(10, "Plains")...)...)...)...)...)
}

// gyGrantObs counts the checks at which a Reprobation host that prints no
// haste has haste while an Anger sits in a graveyard: the grant outlived the
// older loss of abilities.
type gyGrantObs struct{ kept int }

func (o *gyGrantObs) check(g *GorgeSide) {
	gm := g.E.G
	anger := false
	for i := range gm.Objs {
		if ob := &gm.Objs[i]; ob.Zone == state.ZGraveyard && ob.Face() != nil && ob.Face().Name == "Anger" {
			anger = true
			break
		}
	}
	if !anger {
		return
	}
	for i := range gm.Objs {
		ob := &gm.Objs[i]
		if ob.Zone != state.ZBattlefield || ob.Face() == nil || ob.Face().Name != "Reprobation" || ob.AttachedTo == 0 {
			continue
		}
		host := gm.Obj(ob.AttachedTo)
		if host == nil || host.Zone != state.ZBattlefield || host.Face() == nil || host.Face().HasKeyword("Haste") {
			continue
		}
		if g.E.HasKeyword(host.ID, "Haste") {
			o.kept++
		}
	}
}

func TestGraveyardStaticGrantTimestampFourSeat(t *testing.T) {
	corpusDirOrSkip(t)
	obs := &gyGrantObs{}
	d := gyGrantDeck()
	play := fourSeatSetPlay(t, cardset.Raw(cardsettest.Synth(t, "dr-mbc-aaaa")), [4][]string{d, d, d, d}, nil,
		func(g *GorgeSide, _ Side) *Divergence { obs.check(g); return nil }, nil,
		func(seed uint64) Policy { return combatPolicy(seed, nil) })
	for seed := uint64(1); seed <= 20; seed++ {
		res := play(seed)
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if res.StallUnmodelled { // a capacity stall (four seats overflow a pool) is no divergence
			t.Fatalf("seed %d: kernel stopped unmodelled: %s", seed, res.Stall)
		}
	}
	t.Logf("%d checks saw a creature keep a graveyard-granted haste through an older loss of abilities", obs.kept)
	if obs.kept == 0 {
		t.Error("no creature kept a graveyard-granted keyword through an older loss of abilities: the mechanism was never reached")
	}
}

// TestGraveyardStaticGrantTimestampMutation restores the old timestamp 0: the
// recorded game then diverges.
func TestGraveyardStaticGrantTimestampMutation(t *testing.T) {
	corpusDirOrSkip(t)
	r, err := ReadRepro("testdata/repro/graveyard_static_grant_timestamp.repro.json")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugGraveyardGrantTsZero
	dirac.DebugGraveyardGrantTsZero = true
	defer func() { dirac.DebugGraveyardGrantTsZero = old }()
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, &ScriptPolicy{Actions: r.Actions})
	if res.Div == nil {
		t.Fatal("graveyard grant timestamp 0 replays the recorded game with no divergence")
	}
	t.Logf("mutation diverged at %d: %s %s", res.Div.At, res.Div.What, res.Div.Field)
}
