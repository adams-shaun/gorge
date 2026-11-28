package oracle

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// A creature sacrificed by a player who does not own it, read back through
// RememberSacrificed$: the reader answers with the controller it last had on
// the battlefield (CR 608.2h), not the owner its graveyard card has.

// remSacStolen is one stolen-sacrifice game: seat 0 casts steal on the victim
// seat 1 owns, then sweep, which sacrifices it and reads its controller.
type remSacStolen struct {
	name         string
	steal, sweep string
	victim       string
	decks        [2][]cardset.Entry
	start        rules.StartSpec
	picks        []string // object names the policy takes when offered, after the casts
	from, to     state.Zone
	moved        []string // seat 0's cards, one of whose from->to move proves gorge's read named seat 0
	mutation     *bool    // the kernel switch that reads the owner instead
}

// remSacStolenPolicy passes priority except to cast steal while the victim is
// not seat 0's and sweep once it is, each on an empty stack; every other ask
// takes the first offered object named in picks, else answers at random.
func remSacStolenPolicy(seed uint64, c remSacStolen) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(pc *PickCtx) Canon {
		if pc.G == nil {
			return rnd.Pick(pc)
		}
		g := pc.G.E.G
		if pc.D.Kind == DPriority {
			want := c.steal
			for i := range g.Objs {
				o := &g.Objs[i]
				if o.Zone == state.ZBattlefield && o.Controller == 0 && o.Face() != nil && o.Face().Name == c.victim {
					want = c.sweep
				}
			}
			if len(g.Stack) == 0 {
				for _, a := range pc.D.Options {
					if a.Kind() == ActCast && refCardName(pc.G, a.Src()) == want {
						return a
					}
				}
			}
			for _, a := range pc.D.Options {
				if a.Kind() == ActPass {
					return a
				}
			}
		}
		for _, a := range pc.D.Options {
			if oa, ok := pc.G.GorgeOption(a); ok && oa.Kind == "yes" {
				return a // the Optional$ search
			}
		}
		for _, name := range c.picks {
			for _, a := range pc.D.Options {
				oa, ok := pc.G.GorgeOption(a)
				if !ok || oa.Obj == 0 {
					continue
				}
				if o := g.Obj(oa.Obj); o != nil && o.Face() != nil && o.Face().Name == name {
					return a
				}
			}
		}
		return rnd.Pick(pc)
	})
}

// remSacStolenReached reads gorge's log: the victim was sacrificed while seat
// 0 controlled it, and afterwards seat 0 made the move that only follows when
// the remembered card's controller reads as seat 0.
func remSacStolenReached(g *GorgeSide, c remSacStolen) (stolenSac, moved bool) {
	ctrl := map[state.ObjID]state.PlayerID{}
	for j := range g.E.L.Events {
		e := &g.E.L.Events[j]
		o := g.E.G.Obj(e.Obj)
		if o == nil || o.Face() == nil {
			continue
		}
		name := o.Face().Name
		if e.Kind == events.ControlChange {
			ctrl[e.Obj] = e.Player
		}
		if e.Kind != events.MoveZone {
			continue
		}
		if name == c.victim && e.From == state.ZBattlefield && e.To == state.ZGraveyard {
			if p, ok := ctrl[e.Obj]; ok && p == 0 && o.Owner == 1 {
				stolenSac = true
			}
		}
		if stolenSac && slices.Contains(c.moved, name) && o.Owner == 0 && e.From == c.from && e.To == c.to {
			moved = true
		}
	}
	return stolenSac, moved
}

// TestStolenSacrificeRememberedControllerLockstep: Deadly Brew's
// ConditionDefined$ Remembered | ConditionPresent$ Card.Permanent+YouCtrl gate
// holds for the thief who sacrificed the stolen creature (the Ornithopter
// comes back), and Wave of Vitriol's per-card loop offers the thief, the
// ImprintedController of the stolen Dryad Arbor, the basic land search.
func TestStolenSacrificeRememberedControllerLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg := synthReg(t)
	set, err := cardset.RawByName("repeatplayerssel")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []remSacStolen{
		{
			name: "condition gate", steal: "Control Magic", sweep: "Deadly Brew", victim: "Grizzly Bears",
			decks: [2][]cardset.Entry{
				{{Name: "Deadly Brew", Count: 1}, {Name: "Control Magic", Count: 1}, {Name: "Ornithopter", Count: 1}, {Name: "Island", Count: 17}, {Name: "Swamp", Count: 10}, {Name: "Forest", Count: 10}},
				{{Name: "Grizzly Bears", Count: 1}, {Name: "Forest", Count: 39}},
			},
			start: rules.StartSpec{Seats: []rules.StartSeat{
				{Battlefield: []int{3, 4, 5, 6, 20, 21, 30, 31}, Hand: []int{0, 1}, Graveyard: []int{2}},
				{Battlefield: []int{0, 1, 2}, Hand: []int{3}},
			}},
			picks: []string{"Grizzly Bears", "Ornithopter"},
			from:  state.ZGraveyard, to: state.ZHand, moved: []string{"Ornithopter"},
			mutation: &dirac.DebugRemGateOwner,
		},
		{
			name: "imprinted controller", steal: "Act of Treason", sweep: "Wave of Vitriol", victim: "Dryad Arbor",
			decks: [2][]cardset.Entry{
				{{Name: "Act of Treason", Count: 1}, {Name: "Wave of Vitriol", Count: 1}, {Name: "Forest", Count: 28}, {Name: "Mountain", Count: 10}},
				{{Name: "Dryad Arbor", Count: 1}, {Name: "Forest", Count: 39}},
			},
			start: rules.StartSpec{Seats: []rules.StartSeat{
				{Battlefield: []int{2, 3, 4, 5, 6, 7, 8, 30, 31, 32}, Hand: []int{0, 1}},
				{Battlefield: []int{0, 1, 2}, Hand: []int{3}},
			}},
			picks: []string{"Dryad Arbor", "Forest", "Mountain"},
			from:  state.ZLibrary, to: state.ZBattlefield, moved: []string{"Forest", "Mountain"},
			mutation: &dirac.DebugRemCtlOff,
		},
	} {
		t.Run(strings.ReplaceAll(c.name, " ", ""), func(t *testing.T) {
			var decks [2][]*cards.Card
			for s, d := range c.decks {
				for _, e := range d {
					card, ok := reg.Lookup(e.Name)
					if !ok {
						t.Fatalf("%s not in the corpus", e.Name)
					}
					for range e.Count {
						decks[s] = append(decks[s], card)
					}
				}
			}
			tbl, _, _, err := set.Matchup(c.decks[0], c.decks[1])
			if err != nil {
				t.Fatal(err)
			}
			play := func(seed uint64) (Result, *GorgeSide) {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"remsac-a", "remsac-b"}, decks, 0)
				if err != nil {
					t.Fatal(err)
				}
				m.Start = &c.start
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				defer CloseSide(k)
				g, err := NewGorgeSide(m, seed, cfg, nil)
				if err != nil {
					t.Fatal(err)
				}
				return LockstepLed(m, seed, g, k, remSacStolenPolicy(seed, c), RunConfig{Config: cfg, MaxDecisions: 600, MaxTurns: 4}), g
			}
			reached := 0
			for seed := uint64(1); seed <= 4; seed++ {
				res, g := play(seed)
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v %s\nref %v\nker %v", seed,
						res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail, res.Div.RefOptions, res.Div.KerOptions)
				}
				if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
					t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
				}
				sac, moved := remSacStolenReached(g, c)
				if sac && !moved {
					t.Errorf("seed %d: the thief sacrificed %s but moved none of %v from %v to %v: gorge read the owner", seed, c.victim, c.moved, c.from, c.to)
				}
				if sac {
					reached++
				}
			}
			if reached == 0 {
				t.Fatalf("no game sacrificed a %s its controller did not own", c.victim)
			}
			*c.mutation = true
			defer func() { *c.mutation = false }()
			for seed := uint64(1); seed <= 4; seed++ {
				if res, _ := play(seed); res.Div != nil {
					return
				}
			}
			t.Fatal("the kernel reading the owner changed no game: the captured controller is not read")
		})
	}
}
