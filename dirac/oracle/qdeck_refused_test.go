package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestQRefusedCopyCastTaps pins the paid copy cast Q's exile grant cannot pay:
// gorge begins the cast, its mana window taps the offered source, the pool
// stays short and the cast is reversed with the tap standing (CR 733.1). The
// lockstep reaches such activations on the autopay surface (even seeds); with
// the kernel latching StallCopyCast before the tap, every seed that activates
// one stalls or diverges on the tapped land's projection.
func TestQRefusedCopyCastTaps(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, acts := runQDeck(t, 32, false)
	if acts[qdeckUnpayable] == 0 {
		t.Fatalf("no unpayable granted activation in %d games: %v", games, acts)
	}
	if n := acts["Hill Giant"] + acts["Shock"]; n == 0 {
		t.Fatalf("no granted exile activation in %d games", games)
	}
	t.Logf("%d games, %d unpayable activations, %d divergences, %d stalls", games, acts[qdeckUnpayable], divs, stalls)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
}

// qdeckTargetedPolicy activates Q's granted exile ability whenever the stack
// is empty, affordable or not (one copy at a time), counting each unaffordable
// activation under qdeckUnpayable+" "+<card name>. A targeted card (Shock)
// whose copy cannot be paid aborts at gorge's target ask before the mana
// window, tapping nothing; the kernel must mint the copy, let it cease and
// ask no target.
func qdeckTargetedPolicy(seed uint64, acts map[string]int) Policy {
	inner := poisonhighPolicy(seed, map[string]int{}, acts)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return inner.Pick(c)
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActActivate {
				if o, ok := c.G.GorgeOption(a); ok && o.Kind == "ability" && o.SVar != "" {
					g := c.G.E.G
					if ob := g.Obj(o.Obj); ob != nil && ob.Zone == state.ZExile && ob.Face() != nil {
						if len(g.Stack) != 0 {
							continue
						}
						mana := int(g.Players[o.Player].Pool.Total())
						for _, id := range g.Zone(state.ZBattlefield, o.Player) {
							if l := g.Obj(id); l != nil && l.Face() != nil && l.Face().IsLand() && !l.Tapped {
								mana++
							}
						}
						if mana < int(ob.Face().ManaValue()) {
							acts[qdeckUnpayable+" "+ob.Face().Name]++
						}
					}
				}
			}
			buf = append(buf, a)
		}
		d := *c.D
		d.Options = buf
		return inner.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// TestQRefusedTargetedCopyCastLockstep pins an unaffordable targeted
// exile-granted copy cast (Shock) beside an untargeted one (Hill Giant) on the
// autopay surface for every seed 1..128: the kernel's copy cast of an
// unpayable spell with a required target ends before the target ask.
func TestQRefusedTargetedCopyCastLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("qdeck")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kqdeckDecks {
		for _, e := range d {
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
	tbl, _, _, err := set.Matchup(kqdeckDecks[0], kqdeckDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	acts := map[string]int{}
	cfg := Config{GorgeAutopaySurface: true}
	bad := 0
	for seed := uint64(1); seed <= 128; seed++ {
		m, err := NewMatchup(reg, [2]string{"kqdeck-a", "kqdeck-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, qdeckTargetedPolicy(seed, acts), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 20})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			bad++
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			bad++
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s", seed, d.At, d.What, d.Field, d.Diff, d.Detail)
		}
	}
	shock, giant := acts[qdeckUnpayable+" Shock"], acts[qdeckUnpayable+" Hill Giant"]
	t.Logf("unaffordable activations: Shock %d, Hill Giant %d, %d bad games", shock, giant, bad)
	if shock == 0 {
		t.Fatalf("no unaffordable targeted (Shock) activation in 128 games: %v", acts)
	}
	if giant == 0 {
		t.Fatalf("no unaffordable untargeted (Hill Giant) activation in 128 games: %v", acts)
	}
}
