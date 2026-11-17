package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// The plain RepeatEach-over-cards loop's lockstep (dirac/kgen
// lower_repeatcardsloop.go; kernel act_repeatcards.go, C kop.RepeatPlain):
// Balan, Wandering Knight's "attach all Equipment you control" is a
// RepeatEach over Equipment.YouCtrl without ChangeZoneTable$, whose body
// attaches the repeated card (Remembered) to Balan, one per pass. Both
// engines must agree decision by decision through games where it resolves
// with Equipment on the battlefield.
func krepeatLoopDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Balan, Wandering Knight", Count: 6},
		{Name: "Short Sword", Count: 4},
		{Name: "Bonesplitter", Count: 4},
		{Name: "Leonin Scimitar", Count: 4},
		{Name: "Vulshok Morningstar", Count: 4},
		{Name: "Plains", Count: 18},
	}
	return [2][]cardset.Entry{append([]cardset.Entry(nil), d...), append([]cardset.Entry(nil), d...)}
}

// krepeatLoopPolicy activates Balan's attach-all whenever gorge offers it
// while its controller has an Equipment on the battlefield, else casts,
// activates and plays lands whenever it can, else random; it counts those
// activations.
func krepeatLoopPolicy(acts *int) func(uint64) Policy {
	return func(seed uint64) Policy {
		rnd := RandomPolicy(seed)
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil && c.D.Kind == DPriority {
				for _, a := range c.D.Options {
					if a.Kind() != ActActivate {
						continue
					}
					o, ok := c.G.GorgeOption(a)
					if !ok || o.Kind != "ability" {
						continue
					}
					ob := c.G.E.G.Obj(o.Obj)
					if ob == nil || ob.Face() == nil || ob.Zone != state.ZBattlefield || ob.Face().Name != "Balan, Wandering Knight" {
						continue
					}
					if !krepeatHasEquipment(c.G.E.G, ob.Controller) {
						continue
					}
					*acts++
					return a
				}
			}
			buf := make([]Canon, 0, 8)
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActCast, ActCastFace, ActActivate, ActPlayLand:
					buf = append(buf, a)
				}
			}
			if len(buf) == 0 || len(buf) == len(c.D.Options) {
				return rnd.Pick(c)
			}
			d := *c.D
			d.Options = buf
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		})
	}
}

// krepeatHasEquipment reports whether p controls an Equipment on the
// battlefield (the loop then has a subject to attach).
func krepeatHasEquipment(g *state.Game, p state.PlayerID) bool {
	for seat := state.PlayerID(0); seat < 2; seat++ {
		for _, id := range g.Zone(state.ZBattlefield, seat) {
			if ob := g.Obj(id); ob != nil && ob.Controller == p && ob.Face() != nil && slices.Contains(ob.Face().Types, "Equipment") {
				return true
			}
		}
	}
	return false
}

func TestRepeatEachCardsLoopLockstep(t *testing.T) {
	t.Parallel()
	acts := new(int)
	if div := lockstepRunPolicy(t, "repeateachcards", krepeatLoopDecks(), 40, true, nil, nil, krepeatLoopPolicy(acts)); div != 0 {
		t.Fatalf("%d seeds diverged", div)
	}
	if *acts == 0 {
		t.Fatal("Balan's attach-all was never activated with an Equipment out; the lockstep proves nothing")
	}
	t.Logf("Balan attach-all activations with an Equipment out: %d", *acts)
}
