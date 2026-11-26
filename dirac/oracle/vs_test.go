package oracle

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The vs lockstep: the four Verges whose gated second
// mana ability is the LOWER colour in WUBRG order (Bleachbone, Riverpyre,
// Wastewood, Hushwood). gorge's payment window and a failed ward payment
// activate a two-ability source through "Choose a mana ability of <name>";
// the oracle answers the first option, ability 0, which for these four is
// the higher colour (dirac/mana_gate.go). Three matchups: the gate on
// and off at the planner and the window (0), a ward payment that floats
// (1), and a Spree spell's coloured window (2). Each matchup is its own test.

type kvsVerge struct {
	a0, a1 int       // ability 0 and the gated ability 1, a bit of WUBRG
	gate   [2]string // the basic land names the gate counts
}

var kvsVerges = map[string]kvsVerge{
	"Bleachbone Verge": {a0: 2, a1: 0, gate: [2]string{"Plains", "Swamp"}},
	"Riverpyre Verge":  {a0: 3, a1: 1, gate: [2]string{"Island", "Mountain"}},
	"Wastewood Verge":  {a0: 4, a1: 2, gate: [2]string{"Swamp", "Forest"}},
	"Hushwood Verge":   {a0: 4, a1: 0, gate: [2]string{"Forest", "Plains"}},
}

const kvsSyms = "WUBRGC"

var kvsDecks = [][2][]cardset.Entry{{
	// matchup 0: the gate on and off at the planner and the window
	{
		{Name: "Bleachbone Verge", Count: 2}, {Name: "Riverpyre Verge", Count: 2}, {Name: "Wastewood Verge", Count: 2}, {Name: "Hushwood Verge", Count: 2},
		{Name: "Island", Count: 4}, {Name: "Mountain", Count: 4}, {Name: "Forest", Count: 3}, {Name: "Plains", Count: 1},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Walking Corpse", Count: 4}, {Name: "Wind Drake", Count: 4}, {Name: "Savannah Lions", Count: 4},
	},
	{
		{Name: "Bleachbone Verge", Count: 2}, {Name: "Riverpyre Verge", Count: 2}, {Name: "Wastewood Verge", Count: 2}, {Name: "Hushwood Verge", Count: 2},
		{Name: "Swamp", Count: 4}, {Name: "Plains", Count: 4}, {Name: "Forest", Count: 2},
		{Name: "Grizzly Bears", Count: 5}, {Name: "Hill Giant", Count: 4}, {Name: "Walking Corpse", Count: 5}, {Name: "Wind Drake", Count: 4}, {Name: "Savannah Lions", Count: 4},
	},
}, {
	// matchup 1: a failed ward payment floats, a paid one windows
	{
		{Name: "Bleachbone Verge", Count: 2}, {Name: "Riverpyre Verge", Count: 2}, {Name: "Wastewood Verge", Count: 2}, {Name: "Hushwood Verge", Count: 2},
		{Name: "Swamp", Count: 2}, {Name: "Mountain", Count: 2}, {Name: "Forest", Count: 1}, {Name: "Plains", Count: 2},
		{Name: "Tomakul Honor Guard", Count: 3}, {Name: "Bristlebane Battler", Count: 3}, {Name: "Shock", Count: 5}, {Name: "Swords to Plowshares", Count: 3},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Hill Giant", Count: 2}, {Name: "Wind Drake", Count: 3}, {Name: "Walking Corpse", Count: 3},
	},
	{
		{Name: "Bleachbone Verge", Count: 2}, {Name: "Riverpyre Verge", Count: 2}, {Name: "Wastewood Verge", Count: 2}, {Name: "Hushwood Verge", Count: 2},
		{Name: "Swamp", Count: 2}, {Name: "Mountain", Count: 2}, {Name: "Forest", Count: 1}, {Name: "Plains", Count: 2},
		{Name: "Tomakul Honor Guard", Count: 3}, {Name: "Bristlebane Battler", Count: 3}, {Name: "Shock", Count: 5}, {Name: "Swords to Plowshares", Count: 3},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Hill Giant", Count: 2}, {Name: "Wind Drake", Count: 3}, {Name: "Walking Corpse", Count: 3},
	},
}, {
	// matchup 2: a coloured window, the Spree faces gorge's planner declines
	{
		{Name: "Gamma Grotesque", Count: 2}, {Name: "She-Hulk, Jade Defender", Count: 2}, {Name: "Trash the Town", Count: 4},
		{Name: "Wastewood Verge", Count: 4}, {Name: "Hushwood Verge", Count: 4},
		{Name: "Swamp", Count: 10}, {Name: "Forest", Count: 14},
	},
	{
		{Name: "Unliving Legionnaire", Count: 4}, {Name: "Final Showdown", Count: 2}, {Name: "Insatiable Avarice", Count: 2},
		{Name: "Bleachbone Verge", Count: 4}, {Name: "Wastewood Verge", Count: 4},
		{Name: "Swamp", Count: 12}, {Name: "Plains", Count: 8}, {Name: "Forest", Count: 4},
	},
}, {
	// matchup 3: Stature, Size Shifter's {X} power-up; Mind Spring floats the mana
	{
		{Name: "Stature, Size Shifter", Count: 8}, {Name: "Mind Spring", Count: 6}, {Name: "Island", Count: 26},
	},
	{
		{Name: "Stature, Size Shifter", Count: 8}, {Name: "Mind Spring", Count: 6}, {Name: "Island", Count: 26},
	},
}}

// kvsGateOn reads a Verge's gate on gorge's battlefield for its controller.
func kvsGateOn(g *state.Game, ob *state.Object, name string) bool {
	v := kvsVerges[name]
	for _, o := range g.Objs {
		if o.Face() != nil && o.Zone == state.ZBattlefield && o.Controller == ob.Controller &&
			(o.Face().Name == v.gate[0] || o.Face().Name == v.gate[1]) {
			return true
		}
	}
	return false
}

// kvsRun is one matchup's games. mk builds the policy, which sees the counts.
type kvsRun struct {
	mi        int
	seeds     int
	maxTurns  int32
	policy    func(seed uint64, k *kvsObs) Policy
	mutateTbl func(*kop.Table)
	autopay   func(seed uint64) bool // GorgeAutopaySurface; nil: seed%2 == 0
}

// kvsObs is a game's gorge-side observations, written into the run's counts.
type kvsObs struct {
	counts   map[string]int
	seed     uint64
	g        *GorgeSide
	prio     bool // the last pick activated a Verge at priority (not a payment)
	wardPay  bool // the last decision paid a ward cost: the next asks are the float
	wardOpen bool // a ward payment's asks were made; the next pick judges it
	wardPl   int
	wardAsks map[string]int
	evAt     int
	pendTap  state.ObjID
	surf     string
	castCost string // the face mana cost of the spell being cast (the last cast pick)
	// Stature's power-up tally (powerupTally's recipe)
	turn   int
	entry  map[state.ObjID]int
	used   map[state.ObjID]bool
	held   map[[2]int]bool
	lastX  int // the X answered at Stature's ask
	statXd bool
}

func (o *kvsObs) hook(d *decision.Decision, idx int) {
	if !strings.HasPrefix(d.Prompt, "Choose a mana ability of ") {
		return
	}
	name := strings.TrimPrefix(d.Prompt, "Choose a mana ability of ")
	v, ok := kvsVerges[name]
	if !ok {
		return
	}
	ob := o.g.E.G.Obj(d.Source)
	if ob == nil || !kvsGateOn(o.g.E.G, ob, name) {
		o.counts["MUST0/ask while the gate fails"]++
		return
	}
	if len(d.Options) < 2 || d.Options[0].Index != idx {
		o.counts["MUST0/ask not answered with ability 0"]++
	}
	_ = v
	o.counts["ask0/"+name]++
	if o.prio {
		o.counts["prio ask"]++
	} else if o.wardPay {
		o.counts["ward asks/"+name]++
		o.counts["ward asks"]++
		if o.wardAsks == nil {
			o.wardAsks = map[string]int{}
		}
		o.wardAsks[name]++
	} else {
		o.counts["window0/"+name]++
		o.counts["window0 coloured"]++
	}
}

// scan reads gorge's new events: a tap of an out-of-order Verge and the
// mana it adds. gate on + ability 1's colour is the gated colour produced;
// gate off + that colour is the must-be-0.
func (o *kvsObs) scan(ref *GorgeSide) {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < o.evAt {
		o.evAt = 0
	}
	for j := o.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.TurnChange:
			o.turn++
		case events.MoveZone:
			if e.To == state.ZBattlefield {
				o.entry[e.Obj] = o.turn
			}
		case events.AbilityPush:
			if ob := g.Obj(e.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Stature, Size Shifter" {
				o.used[e.Obj] = true
				k := "full"
				if o.entry[e.Obj] == o.turn {
					k = "reduced"
				}
				if o.lastX == 0 {
					k += " X=0"
				} else {
					k += " X>=1"
				}
				o.counts["stature/"+k]++
			}
		case events.Tap:
			o.pendTap = 0
			if ob := g.Obj(e.Obj); ob != nil && ob.Face() != nil {
				if _, ok := kvsVerges[ob.Face().Name]; ok {
					o.pendTap = e.Obj
				}
			}
		case events.ManaAdd:
			if o.pendTap == 0 || e.Amount <= 0 {
				continue
			}
			ob := g.Obj(o.pendTap)
			o.pendTap = 0
			if ob == nil {
				continue
			}
			name := ob.Face().Name
			v := kvsVerges[name]
			col := strings.Index(kvsSyms, e.Counter)
			on := kvsGateOn(g, ob, name)
			if os.Getenv("KVS_PROBE") != "" {
				fmt.Printf("probe %s on=%v counter=%q text=%q\n", name, on, e.Counter, e.Text)
			}
			switch {
			case col == v.a1 && !on:
				o.counts["MUST0/gated colour with the gate off"]++
			case col == v.a1 && on:
				o.counts["gated/"+o.surf]++
				o.counts["gated/"+name]++
			case col == v.a0 && !on:
				o.counts["off a0/"+o.surf]++
			case col == v.a0 && on:
				o.counts["on a0/"+o.surf]++
			}
		}
	}
	o.evAt = len(evs)
}

func kvsEnv(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("vs")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

func kvsDecksFor(t *testing.T, reg *cards.Registry, md [2][]cardset.Entry) (decks [2][]*cards.Card) {
	t.Helper()
	for s, d := range md {
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
	return decks
}

// kvsGames runs the matchup's seeds and returns the counts; a divergence is
// reported through t.Errorf (when t is non-nil) and returned as the first.
func kvsGames(t *testing.T, reg *cards.Registry, set cardset.Set, r kvsRun, counts map[string]int, stopAtDiv bool) (diverged bool, games int) {
	md := kvsDecks[r.mi]
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	if r.mutateTbl != nil {
		t2 := *tbl
		t2.Cards = slices.Clone(tbl.Cards)
		r.mutateTbl(&t2)
		tbl = &t2
	}
	decks := kvsDecksFor(t, reg, md)
	for seed := uint64(1); seed <= uint64(r.seeds); seed++ {
		m, err := NewMatchup(reg, [2]string{"kvs-a", "kvs-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		if r.autopay != nil {
			cfg.GorgeAutopaySurface = r.autopay(seed)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		o := &kvsObs{counts: counts, seed: seed, surf: "window", turn: 1, entry: map[state.ObjID]int{}, used: map[state.ObjID]bool{}, held: map[[2]int]bool{}}
		if seed%2 == 0 {
			o.surf = "planner"
		}
		pol := r.policy(seed, o)
		hooked := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil && c.G.OnInternal == nil {
				o.g = c.G
				c.G.OnInternal = o.hook
			}
			a := pol.Pick(c)
			// a Verge activated at priority is a mana ask outside any payment
			o.prio = false
			if c.G != nil && a.Kind() == ActActivate && int(a.Src()) < len(c.G.id.obj) {
				if ob := c.G.E.G.Obj(c.G.id.obj[a.Src()]); ob != nil && ob.Face() != nil {
					_, o.prio = kvsVerges[ob.Face().Name]
				}
			}
			return a
		})
		var lastRef *GorgeSide
		check := func(ref *GorgeSide, _ Side) *Divergence {
			lastRef = ref
			o.scan(ref)
			return nil
		}
		res := Lockstep(m, seed, k, hooked, RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: r.maxTurns, Check: check})
		CloseSide(k)
		if res.Div != nil && os.Getenv("KVS_DUMP") != "" && lastRef != nil {
			evs := lastRef.E.L.Events
			for j := max(0, len(evs)-60); j < len(evs); j++ {
				e := evs[j]
				name := ""
				if ob := lastRef.E.G.Obj(e.Obj); ob != nil && ob.Face() != nil {
					name = ob.Face().Name
				}
				fmt.Printf("ev %d kind=%d p=%d obj=%d %s ctr=%q amt=%d text=%q\n", j, e.Kind, e.Player, e.Obj, name, e.Counter, e.Amount, e.Text)
			}
		}
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			if t != nil {
				t.Errorf("matchup %d seed %d: kernel capacity stall %q", r.mi, seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			diverged = true
			if t != nil && !stopAtDiv {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", r.mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", r.mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			if stopAtDiv {
				t.Logf("matchup %d seed %d diverged at decision %d: %s %s", r.mi, seed, d.At, d.What, d.Field)
				return
			}
		}
	}
	return
}

// kvsPlayPolicy plays lands, then casts or activates; at a ward payment it
// pays whenever offered (so the kernel's window or, with too little mana,
// the failed-payment float runs). It orders options, never removes one.
func kvsPlayPolicy(seed uint64, o *kvsObs) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	var lands [2]int
	return PolicyFunc(func(c *PickCtx) Canon {
		o.wardPay = false
		if o.wardOpen && c.G != nil {
			// the payment is over: mana still floating in gorge's pool means
			// it failed (a paid ward spends the pool), and every Verge asked
			// during it was tapped for ability 0's colour and left floating
			o.wardOpen = false
			if c.G.E.G.Players[o.wardPl].Pool.Total() > 0 {
				for n, k := range o.wardAsks {
					o.counts["float/"+n] += k
					o.counts["float"] += k
				}
			}
			clear(o.wardAsks)
		}
		if c.G != nil {
			if d := c.G.Pending(); d != nil && d.ResumeKind == "unless_pay" && strings.HasSuffix(d.Prompt, " for ward?") {
				for _, a := range c.D.Options {
					if a.Kind() == ActMode && a.Mode() == 0 {
						o.wardPay = true
						if os.Getenv("KVS_PROBE") != "" {
							var sb strings.Builder
							for _, ob := range c.G.E.G.Objs {
								if ob.Face() != nil && ob.Zone == state.ZBattlefield && int(ob.Controller) == int(d.Player) {
									fmt.Fprintf(&sb, "%s(t=%v) ", ob.Face().Name, ob.Tapped)
								}
							}
							fmt.Printf("wardpay seed %d p%d pool=%d: %s\n", seed, d.Player, c.G.E.G.Players[d.Player].Pool.Total(), sb.String())
						}
						o.wardOpen = true
						o.wardPl = int(d.Player)
						o.counts["ward/paid"]++
						return a
					}
				}
			}
		}
		buf = buf[:0]
		// tight seeds ORDER the options so the Shock caster is short of
		// mana at the ward ask: after two land drops lands are last, and
		// the Shock goes before the other casts
		tight := seed%4 < 3
		pl := int(c.D.Player & 1)
		for _, a := range c.D.Options {
			if a.Kind() == ActPlayLand && (!tight || lands[pl] < 2) {
				buf = append(buf, a)
			}
		}
		if len(buf) > 0 {
			lands[pl]++
		}
		if len(buf) == 0 {
			for _, a := range c.D.Options {
				if k := a.Kind(); (k == ActCast || k == ActActivate) && (!tight || kvsNamed(c, a, "Shock") || kvsNamed(c, a, "Swords to Plowshares")) {
					buf = append(buf, a)
				}
			}
		}
		if len(buf) == 0 {
			for _, a := range c.D.Options {
				if k := a.Kind(); k == ActCast || k == ActActivate {
					buf = append(buf, a)
				}
			}
		}
		if len(buf) == 0 && tight {
			for _, a := range c.D.Options {
				if a.Kind() != ActPlayLand {
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

// kvsNamed: the option's source card is called name.
func kvsNamed(c *PickCtx, a Canon, name string) bool {
	if c.G == nil || int(a.Src()) >= len(c.G.id.obj) {
		return false
	}
	ob := c.G.E.G.Obj(c.G.id.obj[a.Src()])
	return ob != nil && ob.Face() != nil && ob.Face().Name == name
}

func kvsReport(t *testing.T, label string, games int, counts map[string]int) {
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&sb, " %s=%d", k, counts[k])
	}
	t.Logf("%s: %d games;%s", label, games, sb.String())
}

func kvsMustBeZero(t *testing.T, counts map[string]int) {
	for k, n := range counts {
		if strings.HasPrefix(k, "MUST0/") && n != 0 {
			t.Errorf("%s: %d, want 0", k, n)
		}
	}
}

func kvsFloor(t *testing.T, counts map[string]int, keys ...string) {
	for _, k := range keys {
		if counts[k] < 10 {
			t.Errorf("%s: %d, want at least 10", k, counts[k])
		}
	}
}

func TestLowerColourVergeLockstepM0(t *testing.T) {
	t.Parallel()

	reg, set := kvsEnv(t)
	counts := map[string]int{}
	div, games := kvsGames(t, reg, set, kvsRun{mi: 0, seeds: 256, maxTurns: 16, policy: kvsPlayPolicy}, counts, false)
	_ = div
	kvsReport(t, "M0", games, counts)
	kvsMustBeZero(t, counts)
	kvsFloor(t, counts, "gated/planner", "gated/window", "off a0/planner", "off a0/window", "on a0/planner", "on a0/window")
	for n := range kvsVerges {
		kvsFloor(t, counts, "gated/"+n)
	}
}

func TestLowerColourVergeLockstepM1(t *testing.T) {
	t.Parallel()

	reg, set := kvsEnv(t)
	counts := map[string]int{}
	_, games := kvsGames(t, reg, set, kvsRun{mi: 1, seeds: 1024, maxTurns: 16, policy: kvsPlayPolicy}, counts, false)
	kvsReport(t, "M1", games, counts)
	kvsMustBeZero(t, counts)
	kvsFloor(t, counts, "float")
	for n := range kvsVerges {
		if n == "Wastewood Verge" {
			continue // its ability 0 shows in M2's coloured window (window0/)
		}
		if counts["float/"+n] < 1 {
			t.Errorf("float/%s: %d, want at least 1", n, counts["float/"+n])
		}
	}
}

func TestLowerColourVergeLockstepM2(t *testing.T) {
	t.Parallel()

	reg, set := kvsEnv(t)
	counts := map[string]int{}
	pol := func(seed uint64, o *kvsObs) Policy { return powerupPolicy(seed, 4) }
	_, games := kvsGames(t, reg, set, kvsRun{mi: 2, seeds: 128, maxTurns: 34, policy: pol}, counts, false)
	kvsReport(t, "M2", games, counts)
	kvsMustBeZero(t, counts)
	kvsFloor(t, counts, "window0 coloured")
	for _, n := range []string{"Bleachbone Verge", "Wastewood Verge", "Hushwood Verge"} {
		kvsFloor(t, counts, "window0/"+n)
	}
}

// kvsEditVerges applies f to the mana abilities of each out-of-order Verge
// in a copy of the table (the CardDef's Abilities slice is cloned first).
func kvsEditVerges(f func(abs []kop.Ability)) func(*kop.Table) {
	return func(tbl *kop.Table) {
		for i := range tbl.Cards {
			if _, ok := kvsVerges[tbl.Cards[i].Name]; ok {
				tbl.Cards[i].Abilities = slices.Clone(tbl.Cards[i].Abilities)
				f(tbl.Cards[i].Abilities)
			}
		}
	}
}

func kvsManaAbs(abs []kop.Ability) (idx []int) {
	for j := range abs {
		if abs[j].Kind == kop.AbMana && abs[j].AFlags&kop.AfLatent == 0 {
			idx = append(idx, j)
		}
	}
	return
}

// TestLowerColourVergeMutation: each edit of the kernel's table must diverge from gorge
// on some seed (the lockstep sees the property the edit breaks).
func TestLowerColourVergeMutation(t *testing.T) {
	t.Parallel()

	reg, set := kvsEnv(t)
	muts := []struct {
		name string
		mi   int
		edit func(*kop.Table)
	}{
		// (a) read as in-order: the colours swapped, the gate kept on
		// ability 1 (the earlier "lowest colour first" assumption)
		{"read as in-order", 2, kvsEditVerges(func(abs []kop.Ability) {
			m := kvsManaAbs(abs)
			abs[m[0]].ManaColors, abs[m[1]].ManaColors = abs[m[1]].ManaColors, abs[m[0]].ManaColors
		})},
		// (b) gate dropped
		{"gate dropped", 0, kvsEditVerges(func(abs []kop.Ability) {
			for _, j := range kvsManaAbs(abs) {
				abs[j].Trig.Cond = 0
			}
		})},
		// (c) reduction dropped for Stature (CostOncePerGame kept)
		{"reduction dropped for Stature", 3, func(tbl *kop.Table) {
			for i := range tbl.Cards {
				if tbl.Cards[i].Name == "Stature, Size Shifter" {
					tbl.Cards[i].Abilities = slices.Clone(tbl.Cards[i].Abilities)
					for j := range tbl.Cards[i].Abilities {
						tbl.Cards[i].Abilities[j].Cost.Flags &^= kop.CostPowerUp
					}
				}
			}
		}},
	}
	for _, mu := range muts {
		pol := func(seed uint64, o *kvsObs) Policy { return kvsPlayPolicy(seed, o) }
		var autopay func(uint64) bool
		switch mu.mi {
		case 2:
			pol = func(seed uint64, o *kvsObs) Policy { return powerupPolicy(seed, 12) }
		case 3:
			pol = kvsStaturePolicy
			autopay = func(seed uint64) bool { return seed%4 == 0 }
		}
		r := kvsRun{mi: mu.mi, seeds: 64, maxTurns: 34, policy: pol, mutateTbl: mu.edit, autopay: autopay}
		div, _ := kvsGames(t, reg, set, r, map[string]int{}, true)
		if !div {
			t.Errorf("mutation %q: no divergence in %d seeds", mu.name, r.seeds)
		}
	}
}

func (o *kvsObs) statObj(c *PickCtx, a Canon) *state.Object {
	if c.G == nil || int(a.Src()) >= len(c.G.id.obj) {
		return nil
	}
	ob := c.G.E.G.Obj(c.G.id.obj[a.Src()])
	if ob == nil || ob.Face() == nil {
		return nil
	}
	return ob
}

// kvsStaturePolicy scripts the Stature, Size Shifter / Mind Spring recipe on
// the lowering seeds (an {X} spell's witness taps for the largest X, so a
// smaller X leaves mana floating): cast Stature, cast Mind Spring with an X
// that floats exactly {U} or more, optionally play a land after the float,
// then power up with the smallest or the largest X; one seed in four plays
// RandomPolicy. It orders options, never removes one.
func kvsStaturePolicy(seed uint64, o *kvsObs) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	pick := func(c *PickCtx) Canon {
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		if seed%4 == 0 || c.G == nil {
			return rnd.Pick(c)
		}
		g := c.G.E.G
		pend := c.G.Pending()
		// an X ask: whose?
		var xs []Canon
		for _, a := range c.D.Options {
			if a.Kind() == ActChoose {
				xs = append(xs, a)
			}
		}
		if pend != nil && len(xs) > 1 && len(xs) == len(c.D.Options) {
			src := g.Obj(pend.Source)
			name := ""
			if src != nil && src.Face() != nil {
				name = src.Face().Name
			}
			slices.SortFunc(xs, func(a, b Canon) int { return int(a.Mode()) - int(b.Mode()) })
			switch name {
			case "Stature, Size Shifter":
				for _, ob := range g.Objs {
					if ob.Face() != nil && ob.Zone == state.ZBattlefield && ob.Controller == pend.Player && !ob.Tapped && slices.Contains(ob.Face().Types, "Land") {
						o.counts["stature/X ask with an untapped source"]++
						break
					}
				}
				a := xs[len(xs)-1]
				if seed/4%2 == 0 {
					a = xs[0]
				}
				o.lastX = int(a.Mode())
				return a
			case "Mind Spring":
				if seed/2%2 == 0 {
					return xs[len(xs)-2] // max-1: exactly {U} floats
				}
				return xs[0]
			}
			return rnd.Pick(c)
		}
		if c.D.Kind != DPriority {
			return rnd.Pick(c)
		}
		stackLen := len(g.Stack)
		var cast, springs, land, act, pass []Canon
		for _, a := range c.D.Options {
			ob := o.statObj(c, a)
			switch a.Kind() {
			case ActCast:
				if ob != nil && ob.Face().Name == "Stature, Size Shifter" {
					cast = append(cast, a)
				} else if ob != nil && ob.Face().Name == "Mind Spring" {
					springs = append(springs, a)
				}
			case ActPlayLand:
				land = append(land, a)
			case ActActivate:
				if ob != nil && ob.Face().Name == "Stature, Size Shifter" {
					act = append(act, a)
				}
			case ActPass:
				pass = append(pass, a)
			}
		}
		// the power-up frames the counters read
		if p := c.D.Player; p < 2 {
			pool := g.Players[p].Pool
			total := 0
			for _, n := range pool {
				total += int(n)
			}
			for _, ob := range g.Objs {
				if ob.Face() == nil || ob.Face().Name != "Stature, Size Shifter" || ob.Zone != state.ZBattlefield || ob.Controller != state.PlayerID(p) {
					continue
				}
				if len(act) > 0 && o.entry[ob.ID] == o.turn && pool[1] == 1 && total == 1 {
					o.counts["stature/offered on the reduced cost alone"]++
				}
				if len(act) == 0 && o.used[ob.ID] && total >= 2 && pool[1] >= 2 {
					if key := [2]int{int(ob.ID), o.turn}; !o.held[key] {
						o.held[key] = true
						o.counts["stature/second activation withheld"]++
					}
				}
			}
		}
		buf = buf[:0]
		switch {
		case len(act) > 0 && seed%3 == 0 && stackLen > 0 && len(pass) > 0:
			buf = append(buf, pass...) // resolve Mind Spring first; the float stays
		case len(act) > 0 && seed%3 == 0 && stackLen == 0 && len(land) > 0:
			buf = append(buf, land...) // a land after the float: an untapped source at the X ask
		case len(act) > 0:
			buf = append(buf, act...)
		case len(springs) > 0 && o.hasStature(g, state.PlayerID(c.D.Player)):
			buf = append(buf, springs...)
		case len(cast) > 0:
			buf = append(buf, cast...)
		case len(land) > 0:
			buf = append(buf, land...)
		case len(pass) > 0:
			buf = append(buf, pass...)
		default:
			return rnd.Pick(c)
		}
		return pick(c)
	})
}

// hasStature: p controls an unpowered Stature.
func (o *kvsObs) hasStature(g *state.Game, p state.PlayerID) bool {
	for _, ob := range g.Objs {
		if ob.Face() != nil && ob.Face().Name == "Stature, Size Shifter" && ob.Zone == state.ZBattlefield && ob.Controller == p && !o.used[ob.ID] {
			return true
		}
	}
	return false
}

func TestLowerColourVergeStatureLockstep(t *testing.T) {
	t.Parallel()

	reg, set := kvsEnv(t)
	counts := map[string]int{}
	r := kvsRun{mi: 3, seeds: 256, maxTurns: 30, policy: kvsStaturePolicy, autopay: func(seed uint64) bool { return seed%4 == 0 }}
	_, games := kvsGames(t, reg, set, r, counts, false)
	kvsReport(t, "M3", games, counts)
	kvsFloor(t, counts, "stature/reduced X=0", "stature/reduced X>=1", "stature/full X=0", "stature/full X>=1",
		"stature/offered on the reduced cost alone", "stature/X ask with an untapped source", "stature/second activation withheld")
}
