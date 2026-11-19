package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// A token copy minted from a face-down permanent copies its face-down
// copiable values (CR 707.2), including the ward {2} of a cloaked source and
// none of a manifested one (engine/state/copiable.go faceDownCopiableFace,
// CR 701.40a, 701.58a). These games make Cackling Counterpart copy a cloak
// from Cryptic Coat and a manifested Disguise card, so a provenance mark that
// is dropped at the mint (or projected as the token's own face-down status)
// shows up as a projection divergence.

var copyCloakedWardDeck = []cardset.Entry{
	{Name: "Cryptic Coat", Count: 4},
	{Name: "Cackling Counterpart", Count: 4},
	{Name: "Grizzly Bears", Count: 4},
	{Name: "Island", Count: 28},
}

var copyManifestDisguiseWardDeck = []cardset.Entry{
	{Name: "Soul Summons", Count: 4},
	{Name: "Cackling Counterpart", Count: 4},
	{Name: "Basilica Stalker", Count: 12},
	{Name: "Plains", Count: 10},
	{Name: "Island", Count: 10},
}

// copyWardDisguise are the printed Disguise cards in the manifested deck:
// a manifested Disguise has no ward (CR 701.40a), unlike a cast face-down
// Disguise.
var copyWardDisguise = map[string]bool{"Basilica Stalker": true}

// copyWardCounts counts the Cackling Counterpart targets this run aimed at a
// face-down permanent, by the provenance of the permanent copied.
type copyWardCounts struct {
	cloaked, manifestedDisguise int
}

// copyWardPolicy casts setup (Cryptic Coat or Soul Summons) first, holds
// Cackling Counterpart until a face-down permanent exists, and aims it at
// that permanent.
func copyWardPolicy(seed uint64, counts *copyWardCounts, setupCast string) Policy {
	rnd := RandomPolicy(seed)
	var buf, targets []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf, targets = buf[:0], targets[:0]
		faceDown := false
		if c.G != nil {
			g := c.G.E.G
			for p := range g.Players {
				for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
					if o := g.Obj(id); o != nil && o.FaceDown {
						faceDown = true
					}
				}
			}
		}
		var setup, copyCast *Canon
		for i := range c.D.Options {
			a := c.D.Options[i]
			switch a.Kind() {
			case ActCast, ActPlayLand:
				if c.G != nil {
					switch optionFaceName(c, a) {
					case setupCast:
						setup = &c.D.Options[i]
					case "Cackling Counterpart":
						copyCast = &c.D.Options[i]
					}
				}
				buf = append(buf, a)
			case ActTarget:
				if c.G != nil {
					if o, ok := c.G.GorgeOption(a); ok {
						if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.FaceDown {
							targets = append(targets, a)
						}
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
		case len(targets) > 0:
			if c.G != nil {
				if o, ok := c.G.GorgeOption(targets[0]); ok {
					if ob := c.G.E.G.Obj(o.Obj); ob != nil {
						switch {
						case ob.Cloaked:
							counts.cloaked++
						case ob.Manifested && copyWardDisguise[cardName(ob.Card)]:
							counts.manifestedDisguise++
						}
					}
				}
			}
			return targets[0]
		case setup != nil:
			return *setup
		case faceDown && copyCast != nil:
			return *copyCast
		case len(buf) > 0:
			return pick(buf)
		}
		return rnd.Pick(c)
	})
}

// runCopyWardLockstep plays deck against itself, asserting no divergence,
// and returns the provenance counts.
func runCopyWardLockstep(t *testing.T, deck []cardset.Entry, setupCast string, seeds uint64) copyWardCounts {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	var pool []*cards.Card
	for _, e := range deck {
		c, ok := reg.Lookup(e.Name)
		if !ok {
			t.Fatalf("%s not in the corpus", e.Name)
		}
		pool = append(pool, c)
	}
	pd, err := kgen.TableFromCards(reg, pool, "copyfacedownward-oracle")
	if err != nil {
		t.Fatalf("table: %v", err)
	}
	set := cardset.Raw(cardset.NewDataSet(pd))
	tbl, ids, err := set.MatchupN([][]cardset.Entry{deck, deck})
	if err != nil {
		t.Fatal(err)
	}
	var counts copyWardCounts
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := KernelMatchupN(reg, tbl, []string{"cfw-a", "cfw-b"}, ids, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		rc := RunConfig{MaxDecisions: 5000, MaxTurns: 60}
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, copyWardPolicy(seed, &counts, setupCast), rc)
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s", seed, d.At, d.What, d.Field, d.Diff, d.Detail)
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: stalled: %s", seed, res.Stall)
		}
	}
	return counts
}

// TestCopyCloakedWardLockstep: a token copy of a cloaked permanent keeps ward
// {2} as a base keyword (CR 701.58a).
func TestCopyCloakedWardLockstep(t *testing.T) {
	counts := runCopyWardLockstep(t, copyCloakedWardDeck, "Cryptic Coat", 16)
	t.Logf("Cackling Counterpart targeted a cloaked permanent %d times", counts.cloaked)
	if counts.cloaked == 0 {
		t.Fatal("no Cackling Counterpart copy targeted a cloaked permanent")
	}
}

// TestCopyManifestedDisguiseWardLockstep: a token copy of a manifested
// Disguise card has no ward, unlike one minted from a cloak (CR 701.40a).
func TestCopyManifestedDisguiseWardLockstep(t *testing.T) {
	counts := runCopyWardLockstep(t, copyManifestDisguiseWardDeck, "Soul Summons", 16)
	t.Logf("Cackling Counterpart targeted a manifested Disguise %d times", counts.manifestedDisguise)
	if counts.manifestedDisguise == 0 {
		t.Fatal("no Cackling Counterpart copy targeted a manifested Disguise card")
	}
}
