package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// A token copy of a blank permanent copy: Cackling Counterpart copies a
// Malleable Impostor that entered as a copy of a face-down card. Gorge mints
// the token over the source's copy face (copyTokenBasis, src.CopiesFaceDown
// through the copy face being the face-down characteristic set), so the
// token is a nameless 2/2 colourless Creature; the kernel's
// copyTokenEnterFrom must not give the token the blank copy's provenance,
// which would stop projFaceDown from blanking its name (CR 707.2, 707.3).

var cloneFaceDownTokCopyDeck = []cardset.Entry{
	{Name: "Malleable Impostor", Count: 6}, {Name: "Soul Summons", Count: 6},
	{Name: "Cackling Counterpart", Count: 6},
	{Name: "Island", Count: 12}, {Name: "Plains", Count: 10},
}

// cloneFaceDownTokCopyPolicy casts, plays lands and copies whenever it can,
// aiming every copy target at a blank Impostor. It counts the decisions that
// offered one, and the tokens gorge minted over a blank Impostor copy (a
// token whose basis card is the Impostor's and whose copy face is the blank
// face-down characteristic set).
func cloneFaceDownTokCopyPolicy(seed uint64, st *tokCopyCounts) Policy {
	rnd := RandomPolicy(seed)
	var buf, cc, tgt []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			g := c.G.E.G
			for p := range g.Players {
				for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
					o := g.Obj(id)
					if o != nil && o.IsToken && o.CopyFace != nil && o.CopyFace.Name == "" &&
						cardName(o.Card) == "Malleable Impostor" && !st.tok[id] {
						st.tok[id] = true
						st.tokens++
					}
				}
			}
		}
		buf, cc, tgt = buf[:0], cc[:0], tgt[:0]
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCast, ActPlayLand:
				buf = append(buf, a)
				if optionFaceName(c, a) == "Cackling Counterpart" {
					cc = append(cc, a)
				}
			case ActTarget:
				if o, ok := c.G.GorgeOption(a); ok {
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && !ob.FaceDown &&
						cardName(ob.Card) == "Malleable Impostor" && ob.CopyFace != nil && ob.CopyFace.Name == "" {
						tgt = append(tgt, a)
						st.offers++
					}
				}
				buf = append(buf, a)
			}
		}
		pick := func(opts []Canon) Canon {
			d := *c.D
			d.Options = opts
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		switch {
		case len(tgt) > 0:
			return pick(tgt)
		case len(cc) > 0:
			return pick(cc)
		case len(buf) > 0:
			return pick(buf)
		}
		return rnd.Pick(c)
	})
}

// tokCopyCounts is one game's counts.
type tokCopyCounts struct {
	offers int // decisions that offered a blank Impostor as a copy target
	tokens int // tokens gorge minted over a blank Impostor copy
	tok    map[state.ObjID]bool
}

// TestCloneFaceDownTokCopyLockstep: two-seat games of the deck play in
// lockstep with Cackling Counterpart tokens copying blank Impostors.
func TestCloneFaceDownTokCopyLockstep(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	var pool []*cards.Card
	for _, e := range cloneFaceDownTokCopyDeck {
		c, ok := reg.Lookup(e.Name)
		if !ok {
			t.Fatalf("%s not in the corpus", e.Name)
		}
		pool = append(pool, c)
	}
	pd, err := kgen.TableFromCards(reg, pool, "clonefacedowntokcopy-oracle")
	if err != nil {
		t.Fatalf("table: %v", err)
	}
	set := cardset.Raw(cardset.NewDataSet(pd))
	deck := cloneFaceDownTokCopyDeck
	tbl, ids, err := set.MatchupN([][]cardset.Entry{deck, deck})
	if err != nil {
		t.Fatal(err)
	}
	const seeds = 12
	totalOffers, totalTokens := 0, 0
	for seed := uint64(1); seed <= seeds; seed++ {
		st := tokCopyCounts{tok: map[state.ObjID]bool{}}
		m, err := KernelMatchupN(reg, tbl, []string{"tokcopy-a", "tokcopy-b"}, ids, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		rc := RunConfig{MaxDecisions: 3000, MaxTurns: 30}
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, cloneFaceDownTokCopyPolicy(seed, &st), rc)
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Errorf("seed %d: diverged at decision %d (%d blank-target offers, %d blank tokens): %s %s %v %s", seed, d.At, st.offers, st.tokens, d.What, d.Field, d.Diff, d.Detail)
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		totalOffers += st.offers
		totalTokens += st.tokens
	}
	t.Logf("%d games, %d blank-target offers, %d tokens over blank Impostor copies", seeds, totalOffers, totalTokens)
	if totalTokens < 1 {
		t.Errorf("no game minted a token copy over a blank Impostor copy")
	}
}
