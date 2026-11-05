package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestDamagePreventionShapesLockstep: two DamageDone prevention replacements,
// gorge the answer at every decision.
//
//   - King T'Challa transforms into Black Panther, Hope Enduring, whose printed
//     "Prevent all damage that would be dealt to NICKNAME." lowers as
//     CardDef.PreventDmgTo. The opponent aims Lightning Strike and Truck Toss
//     (lethal for a 3/3) at it: it takes no damage and survives.
//   - The Mindskinner attacks unblocked (it can't be blocked): its 10 damage
//     to the opponent is prevented and the opponent mills 10 instead, losing
//     no life (ReplPreventMillOpps).
func TestDamagePreventionShapesLockstep(t *testing.T) {
	t.Run("BlackPanther", func(t *testing.T) {
		st := &dpsStats{}
		dpsRun(t, "std-msh", [2][]cardset.Entry{
			{{Name: "King T'Challa", Count: 12}, {Name: "Plains", Count: 14}, {Name: "Island", Count: 14}},
			{{Name: "Lightning Strike", Count: 12}, {Name: "Truck Toss", Count: 8}, {Name: "Mountain", Count: 20}},
		}, st, dpsPantherCheck(st))
		if st.games == 0 || st.prevented == 0 {
			t.Fatalf("no game prevented lethal damage to Black Panther (%d games)", st.games)
		}
		if st.died != 0 || st.dealt != 0 {
			t.Fatalf("Black Panther was dealt %d damage and died %d times", st.dealt, st.died)
		}
		t.Logf("%d games, %d lethal hits prevented on Black Panther", st.games, st.prevented)
	})
	t.Run("Mindskinner", func(t *testing.T) {
		st := &dpsStats{}
		dpsRun(t, "std-dsk", [2][]cardset.Entry{
			{{Name: "The Mindskinner", Count: 12}, {Name: "Island", Count: 28}},
			{{Name: "Swamp", Count: 40}},
		}, st, dpsMindskinnerCheck(st))
		if st.games == 0 || len(st.groups) == 0 || st.groups[0] != 10 {
			t.Fatalf("no game had The Mindskinner's damage replaced by a mill of 10 (%d games, mills %v)", st.games, st.groups)
		}
		for _, n := range st.groups {
			if n > 10 {
				t.Fatalf("one Mindskinner hit milled %d cards (mills %v)", n, st.groups)
			}
		}
		if st.dealt != 0 || st.lifeLost != 0 {
			t.Fatalf("the opponent was dealt damage %d times and lost life in %d games", st.dealt, st.lifeLost)
		}
		t.Logf("%d games, mills per hit %v", st.games, st.groups)
	})
}

type dpsStats struct {
	games, prevented, dealt, died int
	groups                        []int // cards milled after each Mindskinner attack
	lifeLost                      int   // games the Mindskinner's opponent ended below 20 life
	ref                           *GorgeSide
	evAt                          int
}

// scan walks the gorge events logged since the last call.
func (st *dpsStats) scan(ref *GorgeSide, f func(g *state.Game, e *events.Event)) {
	evs := ref.E.L.Events
	if st.ref != ref || len(evs) < st.evAt {
		st.ref, st.evAt = ref, 0
	}
	for j := st.evAt; j < len(evs); j++ {
		f(ref.E.G, &evs[j])
	}
	st.evAt = len(evs)
}

func dpsNamed(g *state.Game, id state.ObjID, name string) bool {
	o := g.Obj(id)
	return o != nil && o.Face() != nil && o.Face().Name == name
}

func dpsPantherCheck(st *dpsStats) func(*GorgeSide, Side) *Divergence {
	const bp = "Black Panther, Hope Enduring"
	return func(ref *GorgeSide, _ Side) *Divergence {
		st.scan(ref, func(g *state.Game, e *events.Event) {
			if !dpsNamed(g, e.Obj, bp) {
				return
			}
			switch {
			case e.Kind == events.Note && strings.Contains(e.Text, "prevented") && e.Amount >= 3:
				st.prevented++
			case e.Kind == events.Damage && e.Amount > 0:
				st.dealt++
			case e.Kind == events.MoveZone && e.From == state.ZBattlefield && e.To == state.ZGraveyard:
				st.died++
			}
		})
		return nil
	}
}

func dpsMindskinnerCheck(st *dpsStats) func(*GorgeSide, Side) *Divergence {
	return func(ref *GorgeSide, _ Side) *Divergence {
		st.scan(ref, func(g *state.Game, e *events.Event) {
			switch {
			case e.Kind == events.Damage && e.Obj == 0 && e.Player == 1 && e.Amount > 0:
				st.dealt++
			case e.Kind == events.DeclareAttackers:
				for _, id := range e.IDs {
					if dpsNamed(g, id, "The Mindskinner") {
						st.groups = append(st.groups, 0) // one hit: it can't be blocked
					}
				}
			case e.Kind == events.MoveZone && e.From == state.ZLibrary && e.To == state.ZGraveyard && len(st.groups) > 0:
				st.groups[len(st.groups)-1]++
			}
		})
		return nil
	}
}

func dpsRun(t *testing.T, pool string, entries [2][]cardset.Entry, st *dpsStats, check func(*GorgeSide, Side) *Divergence) {
	t.Helper()
	reg := synthReg(t)
	set := cardsettest.Synth(t, pool)
	var decks [2][]*cards.Card
	for s := range entries {
		for _, e := range entries[s] {
			i, ok := set.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the %s pool", e.Name, pool)
			}
			if set.Status(i) == cardset.StatusNotLowered {
				t.Fatalf("%s is not lowered: %s", e.Name, set.Reason(i))
			}
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for j := 0; j < e.Count; j++ {
				decks[s] = append(decks[s], c)
			}
		}
	}
	tbl, _, _, err := set.Matchup(entries[0], entries[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= 6; seed++ {
		m, err := NewMatchup(reg, [2]string{"dps-a", "dps-b"}, decks, 0)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: true}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		finish := func(g *GorgeSide) {
			if g.E.G.Players[1].Life < 20 {
				st.lifeLost++
			}
		}
		res := Lockstep(m, seed, k, dpsPolicy{}, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30, Check: check, Finish: finish})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nrecent %v",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, res.RecentSources(m, 6))
		}
		st.games++
	}
}

// dpsPolicy: keep, play a land, seat 0 casts and transforms King T'Challa and
// attacks with everything; seat 1 burns Black Panther and never blocks.
type dpsPolicy struct{}

func (dpsPolicy) Pick(c *PickCtx) Canon {
	d := c.D
	g := c.G.E.G
	switch d.Kind {
	case DMulligan:
		return pickKind(c, ActKeep)
	case DAttackers:
		if d.Player == 0 {
			return pickKind(c, ActAttack)
		}
		return pickKind(c, ActDone)
	case DBlockers:
		return pickKind(c, ActDone)
	case DTarget:
		if bp := dpsFind(g, "Black Panther, Hope Enduring"); bp != nil {
			ref := c.G.InstRef(bp.ID)
			if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActTarget && a.Tgt() == ref }); ok {
				return a
			}
		}
		return d.Options[0]
	case DPriority:
		if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActPlayLand }); ok {
			return a
		}
		if d.Player == 1 {
			if dpsFind(g, "Black Panther, Hope Enduring") != nil {
				return pickKind(c, ActCast)
			}
			return pickKind(c, ActPass)
		}
		if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActActivate }); ok {
			return a
		}
		if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActCast }); ok && dpsFind(g, "King T'Challa") == nil &&
			dpsFind(g, "Black Panther, Hope Enduring") == nil {
			return a
		}
		return pickKind(c, ActPass)
	}
	return d.Options[0]
}

func dpsFind(g *state.Game, name string) *state.Object {
	for i := range g.Objs {
		o := &g.Objs[i]
		if o.Zone == state.ZBattlefield && o.Card != nil && o.Face() != nil && o.Face().Name == name {
			return o
		}
	}
	return nil
}
