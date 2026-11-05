package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Origin$ Battlefield,Graveyard lockstep: Ketramose, the New Dawn's
// ChangesZoneAll trigger ("one or more cards are put into exile from
// graveyards and/or the battlefield during your turn") and The Spot, Living
// Portal's enters trigger (a Defined$ Targeted ChangeZone over a nonland
// permanent and a graveyard card). Swords to Plowshares and Journey to
// Nowhere exile from the battlefield, Tormod's Crypt from a graveyard; Murder,
// Shock and the creatures fill the graveyards. Every decision, option list and
// state is compared in both engines.

var kOriginMultizoneDecks = [2][]cardset.Entry{
	{
		{Name: "Ketramose, the New Dawn", Count: 4}, {Name: "The Spot, Living Portal", Count: 4},
		{Name: "Swords to Plowshares", Count: 3}, {Name: "Journey to Nowhere", Count: 2},
		{Name: "Tormod's Crypt", Count: 3}, {Name: "Murder", Count: 3}, {Name: "Raise Dead", Count: 1},
		{Name: "Savannah Lions", Count: 3}, {Name: "Walking Corpse", Count: 3},
		{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 6},
	},
	{
		{Name: "Ketramose, the New Dawn", Count: 3}, {Name: "The Spot, Living Portal", Count: 3},
		{Name: "Swords to Plowshares", Count: 2}, {Name: "Journey to Nowhere", Count: 3},
		{Name: "Tormod's Crypt", Count: 2}, {Name: "Murder", Count: 2}, {Name: "Shock", Count: 3},
		{Name: "Savannah Lions", Count: 3}, {Name: "Walking Corpse", Count: 3},
		{Name: "Plains", Count: 7}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 3},
	},
}

// kOriginMultizoneTally counts Ketramose triggers by what the move window
// before the push held.
type kOriginMultizoneTally struct {
	fromBattlefield, fromGraveyard int // a push after a battlefield / graveyard exile
	both                           int // one push after exiles from both zones (one batch)
}

// kOriginMultizonePolicy casts and activates whenever it can (three games in
// four), else random, and scans the gorge log for Ketramose's pushes.
func kOriginMultizonePolicy(seed uint64, tl *kOriginMultizoneTally) Policy {
	rnd := RandomPolicy(seed)
	at := 0
	var win []events.Event // the zone moves since the resolution began; the push follows them
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			g := c.G.E.G
			evs := c.G.E.L.Events
			if len(evs) < at {
				at = 0
			}
			for j := at; j < len(evs); j++ {
				e := &evs[j]
				switch e.Kind {
				case events.MoveZone:
					win = append(win, *e)
				case events.StepChange, events.TurnChange, events.PutOnStack, events.Resolve:
					win = win[:0]
				case events.TriggerPush:
					o := g.Obj(e.Obj)
					if o == nil || o.Face() == nil || o.Face().Name != "Ketramose, the New Dawn" {
						continue
					}
					bf, gy := false, false
					for _, m := range win {
						if m.To == state.ZExile && m.Obj != e.Obj {
							bf = bf || m.From == state.ZBattlefield
							gy = gy || m.From == state.ZGraveyard
						}
					}
					switch {
					case bf && gy:
						tl.both++
					case bf:
						tl.fromBattlefield++
					case gy:
						tl.fromGraveyard++
					}
				}
			}
			at = len(evs)
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActActivate {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 || seed%4 == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

func TestOriginMultizoneLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("originmultizone")
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range kOriginMultizoneDecks {
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
			t.Fatalf("deck %d: %d cards", s, len(gd[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kOriginMultizoneDecks[0], kOriginMultizoneDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	var tl kOriginMultizoneTally
	divs, stalls := 0, 0
	const seeds = 256
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{"koriginmultizone-a", "koriginmultizone-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, kOriginMultizonePolicy(seed, &tl), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 20})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			divs++
			lo := max(d.At-12, 0)
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nlast %v\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail,
				actionStrings(res.Actions[lo:d.At]), d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d seeds, %d divergences, %d stalls; Ketramose pushes: %d after a battlefield exile, %d after a graveyard exile, %d after both",
		seeds, divs, stalls, tl.fromBattlefield, tl.fromGraveyard, tl.both)
	if tl.fromBattlefield < 10 || tl.fromGraveyard < 10 || tl.both < 3 {
		t.Fatalf("pool under-exercised: %+v", tl)
	}
}
