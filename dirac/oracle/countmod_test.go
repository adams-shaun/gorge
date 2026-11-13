package oracle

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Count$ heads lockstep: Count$LeftBattlefieldThisTurn
// (Kutzil's Flanker's counter mode), the /Mod.N value modifier (Sab-Sunen's
// even/odd counters: CantAttack/CantBlock CheckSVar$ and the Main1 draw),
// Count$ValidGraveyard $GreatestCardPower (Kraven's Last Hunt chapter I),
// Count$xPaid read by a Mode$ Cycled self trigger (Valor's Flagship, Webstrike
// Elite) and the Outlaw filter head (Hellspur Brute's Affinity:Outlaw).
// Gorge's planner prices no {X} activation, so an X cycle is offered only
// while the floating pool pays it, which no game here reaches (mana abilities
// are not priority options): the cycle trigger's X is pinned by the kernel's
// TestCycledCardReadsX, the two cards ride the pool as ordinary casts.
//
// The decks are one per colour pair so every card is castable: deck A is
// R/W/B (the Outlaw carriers, burn and removal feeding the leave-the-
// battlefield count, Valor's Flagship), deck B is G/U (Sab-Sunen, Kraven,
// Webstrike Elite and the creatures that fill the graveyards).
var kcnt9Decks = [2][]cardset.Entry{
	{
		{Name: "Kutzil's Flanker", Count: 4}, {Name: "Hellspur Brute", Count: 4}, {Name: "Hired Claw", Count: 4},
		{Name: "Outlaw Medic", Count: 2}, {Name: "Vadmir, New Blood", Count: 2}, {Name: "Shock", Count: 4}, {Name: "Murder", Count: 2},
		{Name: "Valor's Flagship", Count: 1}, {Name: "Gravedigger", Count: 1}, {Name: "Mind Stone", Count: 2},
		{Name: "Mountain", Count: 6}, {Name: "Plains", Count: 5}, {Name: "Swamp", Count: 3},
	},
	{
		{Name: "Sab-Sunen, Luxa Embodied", Count: 4}, {Name: "Kraven's Last Hunt", Count: 3}, {Name: "Webstrike Elite", Count: 3},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 4}, {Name: "Divination", Count: 2},
		{Name: "Mind Stone", Count: 2}, {Name: "Ornithopter", Count: 1}, {Name: "Forest", Count: 11}, {Name: "Island", Count: 6},
	},
}

// kcnt9Tally is the gorge side's own count of each shape, from its log and
// board, so a floor failing means the pool did not reach the shape.
type kcnt9Tally struct {
	game        int
	seen        map[[3]int32]bool
	kutzilPut   int // Kutzil's Flanker carrying a +1/+1 counter (PutCounter X >= 1)
	sabEven     int // Sab-Sunen attacked or blocked with an even counter count
	sabOdd      int // turns an odd-countered Sab-Sunen sat out the declare-attackers step (the CantAttack CheckSVar$ gate)
	sabDraw     int // Sab-Sunen's Main1 trigger drew two
	kravenDmg   int // Kraven chapter I damage >= 1
	bruteAff    int // Hellspur Brute cast with an Outlaw on its caster's battlefield
	kutzilModes int // Kutzil modes picked by the policy: the counter mode after a leaver
}

func (w *kcnt9Tally) once(turn, class int32, obj state.ObjID) bool {
	key := [3]int32{int32(w.game), turn*64 + class, int32(obj)}
	if w.seen[key] {
		return false
	}
	w.seen[key] = true
	return true
}

var kcnt9Outlaws = []string{"Assassin", "Mercenary", "Pirate", "Rogue", "Warlock"}

func (w *kcnt9Tally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	for p := range g.Players {
		for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
			o := g.Obj(id)
			if o == nil || o.Face() == nil {
				continue
			}
			switch o.Face().Name {
			case "Kutzil's Flanker":
				if o.Counter("P1P1") >= 1 && w.once(0, 1, id) {
					w.kutzilPut++
				}
			}
		}
	}
	sabCounters := func(id state.ObjID) int32 {
		if o := g.Obj(id); o != nil {
			return o.Counter("P1P1")
		}
		return 0
	}
	sabOf := func(ids []state.ObjID) (state.ObjID, bool) {
		for _, id := range ids {
			if chName(g, id) == "Sab-Sunen, Luxa Embodied" {
				return id, true
			}
		}
		return 0, false
	}
	if g.Step == state.StepDeclareAttackers {
		// the can't-attack check: an odd-countered Sab-Sunen of the active
		// player's, which gorge never lists as an attacker
		for _, id := range g.Zone(state.ZBattlefield, g.Active) {
			if chName(g, id) == "Sab-Sunen, Luxa Embodied" && sabCounters(id)%2 == 1 && w.once(g.Turn, 9, id) {
				w.sabOdd++
			}
		}
	}
	lastSabCtr, lastSab := -1, state.ObjID(0)
	for i := len(evs) - 1; i >= 0; i-- {
		ev := evs[i]
		if ev.Kind == events.TurnChange {
			break
		}
		switch ev.Kind {
		case events.DeclareAttackers:
			if id, ok := sabOf(ev.IDs); ok && w.once(g.Turn, 3, id) {
				if sabCounters(id)%2 == 0 {
					w.sabEven++
				} else {
					w.sabOdd++
				}
			}
		case events.DeclareBlockers:
			var ids []state.ObjID
			for _, pr := range ev.Pairs {
				ids = append(ids, pr[0])
			}
			ids = append(ids, ev.IDs...)
			if id, ok := sabOf(ids); ok && w.once(g.Turn, 4, id) {
				if sabCounters(id)%2 == 0 {
					w.sabEven++
				} else {
					w.sabOdd++
				}
			}
		case events.CounterChange:
			if lastSabCtr < 0 && chName(g, ev.Obj) == "Sab-Sunen, Luxa Embodied" && ev.Amount > 0 {
				lastSabCtr, lastSab = i, ev.Obj
			}
		case events.DamageProvenance:
			if chName(g, ev.Obj) == "Kraven's Last Hunt" && ev.Amount >= 1 && w.once(g.Turn, 5, ev.Obj) {
				w.kravenDmg++
			}
		case events.PutOnStack:
			if chName(g, ev.Obj) == "Hellspur Brute" && w.once(g.Turn, 6, ev.Obj) {
				n := 0
				for _, id := range g.Zone(state.ZBattlefield, ev.Player) {
					if o := g.Obj(id); o != nil && o.Face() != nil && chName(g, id) != "Hellspur Brute" {
						for _, ty := range o.Face().Types {
							if slices.Contains(kcnt9Outlaws, ty) {
								n++
								break
							}
						}
					}
				}
				if n > 0 {
					w.bruteAff++
				}
			}
		}
	}
	if lastSabCtr >= 0 {
		draws := 0
		for _, ev := range evs[lastSabCtr:] {
			if ev.Kind == events.Draw {
				draws++
			}
		}
		if draws >= 2 && sabCounters(lastSab)%2 == 1 && w.once(g.Turn, 7, 0) {
			w.sabDraw++
		}
	}
	return nil
}

// kcnt9Policy casts and activates (cycles) whenever offered, attacks with
// everything, picks Kutzil's counter mode when a creature left under its
// controller this turn, answers a cycling X ask with X >= 1, else random.
func kcnt9Policy(seed uint64, w *kcnt9Tally) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		a := rnd.Pick(c)
		if c.G == nil {
			return a
		}
		g := c.G.E.G
		switch c.D.Kind {
		case DPriority:
			buf = buf[:0]
			for _, o := range c.D.Options {
				if k := o.Kind(); k == ActCast || k == ActActivate {
					buf = append(buf, o)
				}
			}
			if len(buf) == 0 {
				return a
			}
			d := *c.D
			d.Options = buf
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		case DAttackers:
			for _, o := range c.D.Options {
				if o.Kind() == ActAttack {
					return o
				}
			}
		case DModes:
			pd := c.G.Pending()
			if pd == nil || chName(g, pd.Source) != "Kutzil's Flanker" {
				return a
			}
			for _, en := range g.Entered {
				if en.From == state.ZBattlefield {
					if o := g.Obj(en.Obj); o != nil && o.Owner == pd.Player {
						for _, o := range c.D.Options {
							if o.Kind() == ActMode && o.Mode() == 0 {
								w.kutzilModes++
								return o
							}
						}
					}
				}
			}
		}
		return a
	})
}

func runCountmod(t *testing.T, seeds int, mut bool) (w *kcnt9Tally, games, divs, stalls int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("countmod")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kcnt9Decks {
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
	tbl, _, _, err := set.Matchup(kcnt9Decks[0], kcnt9Decks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mut {
		dirac.DebugModOff = true
		defer func() { dirac.DebugModOff = false }()
	}
	w = &kcnt9Tally{seen: map[[3]int32]bool{}}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kcnt9-a", "kcnt9-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := kcnt9Policy(seed, w)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		w.game = games
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2500, MaxTurns: 25, Check: w.check})
		CloseSide(k)
		games++
		if d := res.Div; d != nil {
			divs++
			if !mut {
				lo := max(d.At-12, 0)
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Logf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Logf("seed %d: kernel stall %q", seed, res.Stall)
		}
	}
	return w, games, divs, stalls
}

func TestCountHeadsLeftAndModLockstep(t *testing.T) {
	t.Parallel()
	w, games, divs, stalls := runCountmod(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls; Kutzil counters %d (modes picked %d), Sab-Sunen combat even %d odd %d, draw-two %d, Kraven damage %d, Brute with an Outlaw %d",
		games, divs, stalls, w.kutzilPut, w.kutzilModes, w.sabEven, w.sabOdd, w.sabDraw, w.kravenDmg, w.bruteAff)
	if divs != 0 {
		t.Errorf("%d divergences", divs)
	}
	if stalls != 0 {
		t.Errorf("%d kernel stalls", stalls)
	}
	for _, f := range []struct {
		name string
		n    int
	}{{"Kutzil counters", w.kutzilPut}, {"Sab-Sunen even combat", w.sabEven}, {"Sab-Sunen odd combat", w.sabOdd}, {"Sab-Sunen draw two", w.sabDraw},
		{"Kraven damage", w.kravenDmg}, {"Brute with an Outlaw", w.bruteAff}} {
		if f.n < 3 {
			t.Errorf("%s: %d, want >= 3", f.name, f.n)
		}
	}
}

// TestCountHeadsLeftAndModMutation turns /Mod.N off in the kernel (ExMod returns its
// operand): Sab-Sunen's parity reads then differ from gorge's.
func TestCountHeadsLeftAndModMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	_, games, divs, _ := runCountmod(t, 64, true)
	if divs == 0 {
		t.Fatalf("DebugModOff: no divergence in %d games", games)
	}
	t.Logf("DebugModOff diverged in %d of %d games", divs, games)
}
