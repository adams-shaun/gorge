package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The X-total divided-damage lockstep: Rolling Thunder, Fire Covenant, Hail of
// Arrows, Rock Slide and Shatterskull Smashing deal a paid X (Shatterskull: X,
// or twice it from 6) divided as you choose among targets, with TargetMax$ not
// the total (an all-candidates count, or two). gorge clamps the announced
// target count to the total (each target takes at least 1, CR 601.2d) and asks
// the division from two targets up; a zero total announces no target at all.
// The lockstep is decision by decision, both sides playing the same mirror
// deck: before the lowering none of the five is in the pool.

var kdxtDecks = [2][]cardset.Entry{{
	{Name: "Rolling Thunder", Count: 3}, {Name: "Fire Covenant", Count: 3}, {Name: "Hail of Arrows", Count: 3},
	{Name: "Rock Slide", Count: 3}, {Name: "Shatterskull Smashing", Count: 3},
	{Name: "Savannah Lions", Count: 3}, {Name: "Mons's Goblin Raiders", Count: 3}, {Name: "Hill Giant", Count: 1}, {Name: "Suntail Hawk", Count: 1},
	{Name: "Mountain", Count: 8}, {Name: "Plains", Count: 5}, {Name: "Swamp", Count: 4},
}, {
	{Name: "Rolling Thunder", Count: 3}, {Name: "Fire Covenant", Count: 3}, {Name: "Hail of Arrows", Count: 3},
	{Name: "Rock Slide", Count: 3}, {Name: "Shatterskull Smashing", Count: 3},
	{Name: "Savannah Lions", Count: 3}, {Name: "Mons's Goblin Raiders", Count: 3}, {Name: "Hill Giant", Count: 1}, {Name: "Suntail Hawk", Count: 1},
	{Name: "Mountain", Count: 8}, {Name: "Plains", Count: 5}, {Name: "Swamp", Count: 4},
}}

var kdxtCards = []string{"Rolling Thunder", "Fire Covenant", "Hail of Arrows", "Rock Slide", "Shatterskull Smashing"}

// kdxtShape is a divided spell on the stack as last seen: its targets are
// chosen after it first appears, its X when it is announced.
type kdxtShape struct {
	name string
	n    int
	x    int32
}

type kdxtCount struct {
	casts   int
	zero    int // no target chosen
	one     int // one target
	divided int // two or more targets over a total above their count: a division is asked
	players int // a player among the targets
}

type kdxtStats struct {
	games, divs int
	by          map[string]*kdxtCount
	bigSmash    int // Shatterskull Smashing casts with X >= 6 over two targets (twice X)
}

// kdxtTotal is the total the cast divides.
func kdxtTotal(name string, x int32) int {
	if name == "Shatterskull Smashing" && x >= 6 {
		return int(2 * x)
	}
	return int(x)
}

func kdxtCheck(seen map[state.ObjID]*kdxtShape, players map[state.ObjID]bool) func(*GorgeSide, Side) *Divergence {
	return func(ref *GorgeSide, _ Side) *Divergence {
		g := ref.E.G
		for _, id := range g.Stack {
			o := g.Obj(id)
			if o == nil || o.Ability != nil || o.Face() == nil {
				continue
			}
			name := o.Face().Name
			if !strings.HasPrefix(name, "Shatterskull") && name != "Rolling Thunder" && name != "Fire Covenant" && name != "Hail of Arrows" && name != "Rock Slide" {
				continue
			}
			sh := seen[id]
			if sh == nil {
				sh = &kdxtShape{name: name}
				seen[id] = sh
			}
			sh.n, sh.x = len(o.Targets), o.X
			for _, t := range o.Targets {
				if t.IsPlayer {
					players[id] = true
				}
			}
		}
		return nil
	}
}

func (st *kdxtStats) tally(seen map[state.ObjID]*kdxtShape, players map[state.ObjID]bool) {
	for id, sh := range seen {
		c := st.by[sh.name]
		if c == nil {
			c = &kdxtCount{}
			st.by[sh.name] = c
		}
		c.casts++
		switch {
		case sh.n == 0:
			c.zero++
		case sh.n == 1:
			c.one++
		case kdxtTotal(sh.name, sh.x) > sh.n:
			c.divided++
		}
		if players[id] {
			c.players++
		}
		if sh.name == "Shatterskull Smashing" && sh.x >= 6 && sh.n == 2 {
			st.bigSmash++
		}
	}
}

// kdxtPolicy plays lands, then casts whenever it can; every other pick (X,
// targets, how many, the division, attackers and blockers) is random. X is
// announced up to 7 so a pick never outgrows the kernel's division arrays.
func kdxtPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		attacking := false
		if c.G != nil {
			g := c.G.E.G
			for _, id := range g.Zone(state.ZBattlefield, 0) {
				attacking = attacking || g.Obj(id).IsAttacking
			}
			for _, id := range g.Zone(state.ZBattlefield, 1) {
				attacking = attacking || g.Obj(id).IsAttacking
			}
		}
		name := func(a Canon) string {
			if c.G == nil {
				return ""
			}
			if o, ok := c.G.GorgeOption(a); ok && o.Obj != 0 {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
					return ob.Face().Name
				}
			}
			return ""
		}
		// the modal card is played as its land only when nothing else is
		var smash Canon
		haveSmash := false
		for _, a := range c.D.Options {
			if a.Kind() == ActPlayLand {
				if name(a) == "Shatterskull Smashing" {
					smash, haveSmash = a, true
					continue
				}
				return a
			}
		}
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActCastFace || k == ActActivate {
				// the combat-only spells are cast with attackers to target
				if n := name(a); !attacking && (n == "Hail of Arrows" || n == "Rock Slide") {
					continue
				}
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			if haveSmash {
				return smash
			}
			for _, a := range c.D.Options {
				if k := a.Kind(); !attacking && (k == ActCast || k == ActCastFace) {
					continue // a combat-only spell held back above
				}
				if xc, _ := chooseCode("x"); a.Kind() != ActChoose+CanonKind(xc) || a.Mode() <= 7 {
					buf = append(buf, a)
				}
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

func runDividedXTotal(t *testing.T, seeds int) kdxtStats {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("divxtotal")
	if err != nil {
		t.Skip(err) // no tables yet: make dirac-tables
	}
	var decks [2][]*cards.Card
	for s, d := range kdxtDecks {
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
	tbl, _, _, err := set.Matchup(kdxtDecks[0], kdxtDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	st := kdxtStats{by: map[string]*kdxtCount{}}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kdxt-a", "kdxt-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		seen := map[state.ObjID]*kdxtShape{}
		players := map[state.ObjID]bool{}
		res := Lockstep(m, seed, k, kdxtPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 25, Check: kdxtCheck(seen, players)})
		CloseSide(k)
		st.tally(seen, players)
		st.games++
		if d := res.Div; d != nil {
			st.divs++
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
	}
	return st
}

func TestDividedXTotalLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runDividedXTotal(t, 600)
	for _, n := range kdxtCards {
		c := st.by[n]
		if c == nil {
			c = &kdxtCount{}
		}
		t.Logf("%s: %d casts; %d with no target, %d with one, %d dividing over two or more, %d naming a player", n, c.casts, c.zero, c.one, c.divided, c.players)
	}
	t.Logf("%d games, %d divergences; Shatterskull Smashing at X >= 6 over two targets: %d", st.games, st.divs, st.bigSmash)
	if st.divs != 0 {
		t.Fatalf("%d divergences", st.divs)
	}
	// every card must be cast with the targets that exercise the clamp and the
	// division: Rolling Thunder and Shatterskull Smashing reach players and
	// divide among two or more, Fire Covenant, Hail of Arrows and Rock Slide
	// divide among creatures, and each is cast with a target and with none
	for _, n := range kdxtCards {
		c := st.by[n]
		if c == nil || c.one < 2 || c.zero < 2 || c.divided < 2 {
			t.Errorf("%s: casts %+v, want at least 2 each with no target, one target and a division", n, c)
		}
	}
	if c := st.by["Rolling Thunder"]; c != nil && c.players < 2 {
		t.Errorf("Rolling Thunder named a player in only %d casts", c.players)
	}
	if st.bigSmash < 1 {
		t.Errorf("no Shatterskull Smashing cast at X >= 6 over two targets")
	}
}
