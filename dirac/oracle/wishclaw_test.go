package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Wishclaw Talisman lockstep: it enters with three wish counters
// (K:etbCounter:WISH:3, kernel kop.CtrWish, a named counter with no rules
// meaning; CR 122.1); "{1}, {T}, remove a wish counter: search your library
// for a card, put it into your hand, then shuffle; an opponent gains control
// of Wishclaw Talisman; activate only during your turn". The policy prefers
// casting and activating, so the Talisman changes hands and its new
// controller activates it on their own turn. Every decision and projection
// must match gorge.

var wishclawDeck = []cardset.Entry{
	{Name: "Wishclaw Talisman", Count: 6}, {Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 6},
	{Name: "Swamp", Count: 10}, {Name: "Forest", Count: 5}, {Name: "Mountain", Count: 5},
}

// wishclawTally counts the Talisman's activations in gorge's view.
type wishclawTally struct {
	byOwner, byTaker int // activations by its owner, and by a player who gained control of it
}

func wishclawPolicy(seed uint64, t *wishclawTally) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCast, ActActivate, ActPlayLand:
				buf = append(buf, a)
			}
		}
		var a Canon
		if len(buf) > 0 {
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		} else {
			a = rnd.Pick(c)
		}
		if c.G != nil && a.Kind() == ActActivate {
			if opt, ok := c.G.GorgeOption(a); ok {
				if o := c.G.E.G.Obj(opt.Obj); o != nil && o.Face() != nil && o.Face().Name == "Wishclaw Talisman" {
					if o.Controller == o.Owner {
						t.byOwner++
					} else {
						t.byTaker++
					}
				}
			}
		}
		return a
	})
}

func TestWishclawTalismanLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, set, decks, tbl := p3Setup(t, "wishclaw", [2][]cardset.Entry{wishclawDeck, wishclawDeck})
	var tally wishclawTally
	wished := 0 // games ending with a Talisman holding fewer than three wish counters
	for seed := uint64(1); seed <= 30; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"wishclaw-a", "wishclaw-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, wishclawPolicy(seed, &tally), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 25})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: %s", seed, res.Stall)
		}
		for _, o := range g.E.G.Objs {
			if o.Zone == state.ZBattlefield && o.Face() != nil && o.Face().Name == "Wishclaw Talisman" && o.Counter("WISH") < 3 {
				wished++
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
	if tally.byOwner == 0 || tally.byTaker == 0 || wished == 0 {
		t.Fatalf("precondition: owner activations %d, activations after the control change %d, games with a spent wish counter %d (each must be > 0)", tally.byOwner, tally.byTaker, wished)
	}
	t.Logf("activations: %d by the owner, %d by the new controller; %d games spent a wish counter", tally.byOwner, tally.byTaker, wished)
}
