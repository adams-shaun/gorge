package oracle

import (
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The kc10d lockstep (D): a mana ability gated by IsPresent$
// (gorge pay.ManaActivationGateHolds), in two shapes: the Verge pair (a plain
// ungated {T} ability beside a gated one of another colour) and a card whose
// only mana ability is gated (Mox Jasper, Flywheel Racer, Tablet of
// Compleation). Matchup 2 reaches the kernel's mana window (payWindow,
// windowTap): a Shock at a Cackling Prowler (Ward {2}) makes the caster pay
// from its sources, and the ward-window counters read the gate on and off at
// that payment. A generic ward cost cannot show which colour a Verge's window
// activation produced; the four out-of-order Verges are proved in vs_test.go.
// Matchup 0 holds the six Verges whose gated colour is above the first
// ability's, matchup 1 the single gated cards. Run as its own shard
// (-run '^TestGatedManaAbilityLockstep$').
var gatedmanaabilityDecks = [][2][]cardset.Entry{{
	{
		{Name: "Blazemire Verge", Count: 2}, {Name: "Floodfarm Verge", Count: 2},
		{Name: "Gloomlake Verge", Count: 2}, {Name: "Thornspire Verge", Count: 2},
		{Name: "Sunbillow Verge", Count: 2}, {Name: "Willowrush Verge", Count: 2},
		{Name: "Swamp", Count: 3}, {Name: "Mountain", Count: 3}, {Name: "Plains", Count: 2},
		{Name: "Forest", Count: 2}, {Name: "Island", Count: 2},
		{Name: "Hill Giant", Count: 3}, {Name: "Walking Corpse", Count: 3}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Wind Drake", Count: 3}, {Name: "Savannah Lions", Count: 2}, {Name: "Gray Ogre", Count: 2},
	},
	{
		{Name: "Blazemire Verge", Count: 2}, {Name: "Sunbillow Verge", Count: 2}, {Name: "Willowrush Verge", Count: 2},
		{Name: "Swamp", Count: 4}, {Name: "Mountain", Count: 4}, {Name: "Plains", Count: 3},
		{Name: "Forest", Count: 3}, {Name: "Island", Count: 3},
		{Name: "Hill Giant", Count: 3}, {Name: "Walking Corpse", Count: 3}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Wind Drake", Count: 3}, {Name: "Savannah Lions", Count: 3}, {Name: "Gray Ogre", Count: 2},
	},
}, {
	{
		{Name: "Mox Jasper", Count: 4}, {Name: "Flywheel Racer", Count: 4}, {Name: "Tablet of Compleation", Count: 6},
		{Name: "Dragon Hatchling", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Mountain", Count: 9}, {Name: "Forest", Count: 5}, {Name: "Island", Count: 4},
	},
	{
		{Name: "Mox Jasper", Count: 4}, {Name: "Flywheel Racer", Count: 4}, {Name: "Tablet of Compleation", Count: 6},
		{Name: "Dragon Hatchling", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Mountain", Count: 9}, {Name: "Forest", Count: 5}, {Name: "Island", Count: 4},
	},
}, {
	// matchup 2 (the ward window): Cackling Prowler (Ward {2}) beside Shock, so
	// the Shock caster pays the ward cost from its sources through the
	// kernel's payWindow/windowTap, with single gated sources and two Verges in
	// play: Blazemire's gate opens on a Swamp or Mountain, Floodfarm's (Plains
	// or Island, neither in the deck) never does
	{
		{Name: "Mox Jasper", Count: 4}, {Name: "Tablet of Compleation", Count: 4}, {Name: "Dragon Hatchling", Count: 3},
		{Name: "Blazemire Verge", Count: 3}, {Name: "Floodfarm Verge", Count: 3}, {Name: "Cackling Prowler", Count: 6}, {Name: "Shock", Count: 7},
		{Name: "Swamp", Count: 2}, {Name: "Mountain", Count: 2}, {Name: "Forest", Count: 6},
	},
	{
		{Name: "Mox Jasper", Count: 4}, {Name: "Tablet of Compleation", Count: 4}, {Name: "Dragon Hatchling", Count: 3},
		{Name: "Blazemire Verge", Count: 3}, {Name: "Floodfarm Verge", Count: 3}, {Name: "Cackling Prowler", Count: 6}, {Name: "Shock", Count: 7},
		{Name: "Swamp", Count: 2}, {Name: "Mountain", Count: 2}, {Name: "Forest", Count: 6},
	},
}}

// gatedmanaabilityGate maps a Verge to the basic land names its gate counts (IsPresent$
// <A>.YouCtrl,<B>.YouCtrl, a Plains/Swamp... of the same controller).
var gatedmanaabilityGate = map[string][2]string{
	"Blazemire Verge": {"Swamp", "Mountain"}, "Floodfarm Verge": {"Plains", "Island"},
	"Gloomlake Verge": {"Island", "Swamp"}, "Thornspire Verge": {"Mountain", "Forest"},
	"Sunbillow Verge": {"Mountain", "Plains"}, "Willowrush Verge": {"Forest", "Island"},
}

// gatedmanaabilityGateOn reads the gate of carrier ob from gorge's battlefield (the
// activating player is the controller). ok is false for a card whose gate
// the test does not read (Flywheel Racer's "is a creature").
func gatedmanaabilityGateOn(g *state.Game, ob *state.Object, name string) (on, ok bool) {
	if gt, isVerge := gatedmanaabilityGate[name]; isVerge {
		for _, o := range g.Objs {
			if o.Face() != nil && o.Zone == state.ZBattlefield && o.Controller == ob.Controller &&
				(o.Face().Name == gt[0] || o.Face().Name == gt[1]) {
				return true, true
			}
		}
		return false, true
	}
	switch name {
	case "Mox Jasper":
		for _, o := range g.Objs {
			if o.Face() != nil && o.Zone == state.ZBattlefield && o.Controller == ob.Controller && o.Face().Name == "Dragon Hatchling" {
				return true, true
			}
		}
		return false, true
	case "Tablet of Compleation":
		return ob.Counter("OIL") >= 2, true
	}
	return false, false
}

// gatedmanaabilityGatedMask is the mana a carrier's gate opens: a Verge's gated colour,
// any colour for Mox Jasper and Flywheel Racer, {C} for Tablet of
// Compleation. ManaAmount is [6]uint32 in WUBRGC order (canon.go's manaSyms).
func gatedmanaabilityGatedMask(name string) int {
	switch name {
	case "Blazemire Verge", "Sunbillow Verge":
		return 1 << 3 // R
	case "Floodfarm Verge":
		return 1 << 1 // U
	case "Gloomlake Verge":
		return 1 << 2 // B
	case "Thornspire Verge", "Willowrush Verge":
		return 1 << 4 // G
	case "Mox Jasper", "Flywheel Racer":
		return 0x1f // any colour
	case "Tablet of Compleation":
		return 1 << 5 // C
	}
	return 0
}

// gatedmanaabilityPlanGated reports whether the chosen cast's payment plan activates
// carrier and produces a colour the gate opens (gorge's
// PaymentActivation.Produces): the gated colour itself, not merely the
// source being tapped.
func gatedmanaabilityPlanGated(c *PickCtx, obj, carrier state.ObjID, mask int) bool {
	if c.G == nil || mask == 0 {
		return false
	}
	hit := func(plans []decision.PaymentPlan) bool {
		for i := range plans {
			for _, act := range plans[i].Activations {
				if act.Source != carrier {
					continue
				}
				for b := 0; b < 6; b++ {
					if mask&(1<<b) != 0 && act.Produces[b] > 0 {
						return true
					}
				}
			}
		}
		return false
	}
	e := c.G.E
	for _, pa := range e.EnsurePaymentActions() {
		if pa.Cast.Object == obj && hit(pa.Plans) {
			return true
		}
	}
	if d := c.G.Pending(); d != nil {
		for _, pp := range e.PotentialPaymentPlans(d.Player) {
			if pp.Action.Obj == obj && pp.Plan != nil && hit([]decision.PaymentPlan{*pp.Plan}) {
				return true
			}
		}
	}
	return false
}

// gatedmanaabilityPolicy plays lands, then casts or activates. At every cast it notes
// each untapped gated carrier with its gate state and the payment surface
// (seed parity: odd = the payment window, even = the planner), and at the
// next decision whether the payment tapped it. The lockstep compares the
// kernel's tapped state, so a wrong gate diverges.
func gatedmanaabilityPolicy(seed uint64, counts map[string]int) Policy {
	rnd := RandomPolicy(seed)
	surf := "window"
	if seed%2 == 0 {
		surf = "planner"
	}
	type note struct {
		key string
		ob  state.ObjID
		on  bool
	}
	var pend []note
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			g := c.G.E.G
			for _, n := range pend {
				ob := g.Obj(n.ob)
				tapped := ob != nil && ob.Zone == state.ZBattlefield && ob.Tapped
				if tapped {
					counts[n.key+"/tapped"]++
				}
				if !n.on && !tapped {
					counts[n.key+"/withheld"]++
				}
			}
			pend = pend[:0]
		}
		if c.G != nil {
			// a ward payment: pay, so the kernel's mana window (payWindow,
			// windowTap) takes the payer's sources, and note each carrier with
			// its gate state; the next decision shows whether the window tapped it
			if d := c.G.Pending(); d != nil && d.ResumeKind == "unless_pay" && strings.HasSuffix(d.Prompt, " for ward?") {
				for _, a := range c.D.Options {
					if a.Kind() != ActMode || a.Mode() != 0 {
						continue
					}
					g := c.G.E.G
					for _, ob := range g.Objs {
						if ob.Face() == nil || ob.Zone != state.ZBattlefield || ob.Tapped || ob.Controller != d.Player {
							continue
						}
						name := ob.Face().Name
						on, ok := gatedmanaabilityGateOn(g, &ob, name)
						if !ok {
							continue
						}
						st := "off"
						if on {
							st = "on"
						}
						shape := "verge"
						if _, v := gatedmanaabilityGate[name]; !v {
							shape = "single:" + name
						}
						key := "ward/" + shape + "/" + st
						counts[key+"/offered"]++
						pend = append(pend, note{key: key, ob: ob.ID, on: on})
					}
					counts["ward/paid"]++
					return a
				}
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActPlayLand {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 && seed%3 == 0 && c.G != nil && gatedmanaabilityTabletBelow2(c.G.E.G) {
			// every third game prefers activations: Tablet of Compleation's
			// oil counters open its mana ability
			for _, a := range c.D.Options {
				if ob := c.G.E.G.Obj(state.ObjID(a.Src())); a.Kind() == ActActivate && ob != nil && ob.Face() != nil && ob.Face().Name == "Tablet of Compleation" {
					buf = append(buf, a)
				}
			}
		}
		if len(buf) == 0 && c.G != nil && gatedmanaabilityTabletReady(c.G.E.G) {
			// an untapped Tablet holding two oil counters: cast, so a payment
			// meets its open mana ability
			for _, a := range c.D.Options {
				if a.Kind() == ActCast {
					buf = append(buf, a)
				}
			}
			if len(buf) == 0 {
				// nothing to cast yet: anything but tapping a Tablet
				for _, a := range c.D.Options {
					if ob := c.G.E.G.Obj(state.ObjID(a.Src())); a.Kind() != ActActivate || ob == nil || ob.Face() == nil || ob.Face().Name != "Tablet of Compleation" {
						buf = append(buf, a)
					}
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
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		pick := rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		if c.G != nil && pick.Kind() == ActCast {
			obj := state.ObjID(pick.Src())
			for _, ob := range c.G.E.G.Objs {
				if ob.Face() == nil || ob.Zone != state.ZBattlefield || ob.Tapped {
					continue
				}
				name := ob.Face().Name
				on, ok := gatedmanaabilityGateOn(c.G.E.G, &ob, name)
				if !ok {
					if name == "Flywheel Racer" {
						pend = append(pend, note{key: "Flywheel/" + surf, ob: ob.ID, on: true})
						counts["Flywheel/"+surf+"/offered"]++
						if gatedmanaabilityPlanGated(c, obj, ob.ID, gatedmanaabilityGatedMask(name)) {
							counts["Flywheel/"+surf+"/gated"]++
						}
					}
					continue
				}
				st := "off"
				if on {
					st = "on"
				}
				shape := "verge"
				if _, v := gatedmanaabilityGate[name]; !v {
					shape = "single:" + name
				}
				key := shape + "/" + st + "/" + surf
				counts[key+"/offered"]++
				if on && gatedmanaabilityPlanGated(c, obj, ob.ID, gatedmanaabilityGatedMask(name)) {
					counts[key+"/gated"]++
				}
				pend = append(pend, note{key: key, ob: ob.ID, on: on})
			}
		}
		return pick
	})
}

// gatedmanaabilityGame plays one seeded game of matchup mi, counting into
// counts.
func gatedmanaabilityGame(t *testing.T, set cardset.Set, build func(int) (*Matchup, error), tbl *kop.Table, mi int, seed uint64, counts map[string]int) {
	t.Helper()
	m, err := build(mi)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{GorgeAutopaySurface: seed%2 == 0}
	k, err := SetSide(set, tbl, cfg)(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	res := Lockstep(m, seed, k, gatedmanaabilityPolicy(seed, counts), RunConfig{Config: cfg, MaxDecisions: 1500, MaxTurns: 16})
	CloseSide(k)
	if strings.HasPrefix(res.Stall, KernelStallPrefix) {
		t.Errorf("matchup %d seed %d: kernel capacity stall %q", mi, seed, res.Stall)
	}
	if d := res.Div; d != nil {
		lo := max(d.At-12, 0)
		t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
		t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
	}
}

// gatedmanaabilityWardMatchup is the matchup whose games reach the ward mana window.
const gatedmanaabilityWardMatchup = 2

// gatedmanaabilityWardKeys are the ward-window counters whose floor is 10: every
// carrier met with its gate on and off at a ward payment, a gate-on single
// source tapped by the window, a gate-off one withheld, and a Verge tapped.
var gatedmanaabilityWardKeys = []string{
	"ward/single:Mox Jasper/on/offered", "ward/single:Mox Jasper/off/offered",
	"ward/single:Tablet of Compleation/on/offered", "ward/single:Tablet of Compleation/off/offered",
	"ward/verge/on/offered", "ward/verge/off/offered",
	"ward/single:Mox Jasper/on/tapped", "ward/single:Tablet of Compleation/on/tapped",
	"ward/single:Mox Jasper/off/withheld", "ward/single:Tablet of Compleation/off/withheld",
	"ward/verge/on/tapped",
}

func gatedmanaabilityWardFloors(counts map[string]int) bool {
	for _, k := range gatedmanaabilityWardKeys {
		if counts[k] < 10 {
			return false
		}
	}
	return true
}

func gatedmanaabilityWardCounts(counts map[string]int) map[string]int {
	out := map[string]int{}
	for _, k := range gatedmanaabilityWardKeys {
		out[k] = counts[k]
	}
	return out
}

func gatedmanaabilityEnv(t *testing.T) (cardset.Set, func(mi int) (*Matchup, error)) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("enterstapped")
	if err != nil {
		t.Fatal(err)
	}
	build := func(mi int) (*Matchup, error) {
		md := gatedmanaabilityDecks[mi]
		var decks [2][]*cards.Card
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
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		return NewMatchup(reg, [2]string{"kc10d-a", "kc10d-b"}, decks, 1)
	}
	return set, build
}

func TestGatedManaAbilityLockstep(t *testing.T) {
	t.Parallel()

	set, build := gatedmanaabilityEnv(t)
	counts := map[string]int{}
	games := 0
	// pre is the counts of the matchups up to the ward matchup: what the
	// ward matchup's extension read when the matchups played in order
	pre := map[string]int{}
	merge := func(part map[string]int, mi int) {
		for k, v := range part {
			counts[k] += v
			if mi <= gatedmanaabilityWardMatchup {
				pre[k] += v
			}
		}
	}
	// every deck builds (build's lookups fail on the test's goroutine)
	for mi := range gatedmanaabilityDecks {
		if _, err := build(mi); err != nil {
			t.Fatal(err)
		}
	}
	// seeds 1..512 of every matchup as parallel chunks, each on a table of its own
	matchupChunks(t, len(gatedmanaabilityDecks), 1, 512, 128, func(t *testing.T, c seedChunk, mu *sync.Mutex) {
		md := gatedmanaabilityDecks[c.mi]
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		part := map[string]int{}
		for seed := c.lo; seed <= c.hi; seed++ {
			gatedmanaabilityGame(t, set, build, tbl, c.mi, seed, part)
		}
		mu.Lock()
		defer mu.Unlock()
		games += int(c.hi - c.lo + 1)
		merge(part, c.mi)
	})
	// the ward matchup runs on, seed by seed, until every ward floor is met
	// (deterministic: the stop reads only the counts so far)
	wmd := gatedmanaabilityDecks[gatedmanaabilityWardMatchup]
	wtbl, _, _, err := set.Matchup(wmd[0], wmd[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(513); seed <= 4096 && !gatedmanaabilityWardFloors(pre); seed++ {
		part := map[string]int{}
		gatedmanaabilityGame(t, set, build, wtbl, gatedmanaabilityWardMatchup, seed, part)
		games++
		merge(part, gatedmanaabilityWardMatchup)
	}
	t.Logf("%d games, carrier counts %v", games, counts)
	for _, sh := range []string{"verge"} {
		for _, st := range []string{"on", "off"} {
			for _, sf := range []string{"planner", "window"} {
				if k := sh + "/" + st + "/" + sf; counts[k+"/offered"] < 10 {
					t.Errorf("%s offered: %d, want at least 10", k, counts[k+"/offered"])
				}
			}
		}
		for _, sf := range []string{"planner", "window"} {
			if k := sh + "/on/" + sf; counts[k+"/tapped"] < 10 {
				t.Errorf("%s tapped by a payment: %d, want at least 10", k, counts[k+"/tapped"])
			}
		}
	}
	for _, sh := range []string{"single:Mox Jasper", "single:Tablet of Compleation"} {
		for _, st := range []string{"on", "off"} {
			for _, sf := range []string{"planner", "window"} {
				if k := sh + "/" + st + "/" + sf; counts[k+"/offered"] < 10 {
					t.Errorf("%s offered: %d, want at least 10", k, counts[k+"/offered"])
				}
			}
		}
	}
	for _, sf := range []string{"planner", "window"} {
		if counts["Flywheel/"+sf+"/offered"] < 10 || counts["Flywheel/"+sf+"/tapped"] < 10 {
			t.Errorf("Flywheel Racer %s: offered %d tapped %d, want at least 10 each", sf, counts["Flywheel/"+sf+"/offered"], counts["Flywheel/"+sf+"/tapped"])
		}
	}
	// The gated colour itself is produced (gorge's PaymentActivation.Produces),
	// not merely the carrier tapped, at a planner payment.
	for _, k := range []string{"verge/on/planner", "single:Mox Jasper/on/planner", "single:Tablet of Compleation/on/planner"} {
		if counts[k+"/gated"] < 10 {
			t.Errorf("%s gated colour produced: %d, want at least 10", k, counts[k+"/gated"])
		}
	}
	if counts["Flywheel/planner/gated"] < 10 {
		t.Errorf("Flywheel Racer planner gated colour produced: %d, want at least 10", counts["Flywheel/planner/gated"])
	}
	// The ward window (matchup 2): the kernel's windowTap meets each carrier
	// with its gate on and off. A gate-on single source is tapped by a window
	// payment; a gate-off one never is (a Verge's ungated ability keeps it
	// tappable either way, so its tapped count is not a gate signal here).
	if !gatedmanaabilityWardFloors(counts) {
		t.Errorf("ward window floors not met: %v", gatedmanaabilityWardCounts(counts))
	}
	for _, sh := range []string{"single:Mox Jasper", "single:Tablet of Compleation"} {
		if n := counts["ward/"+sh+"/off/tapped"]; n != 0 {
			t.Errorf("ward/%s/off tapped by the window: %d, want 0", sh, n)
		}
	}
	// A gate-off single gated source is withheld (untapped) and never tapped
	// by a payment on any surface.
	for _, sh := range []string{"single:Mox Jasper", "single:Tablet of Compleation"} {
		for _, sf := range []string{"planner", "window"} {
			if n := counts[sh+"/off/"+sf+"/tapped"]; n != 0 {
				t.Errorf("%s/off/%s tapped by a payment: %d, want 0", sh, sf, n)
			}
		}
		if counts[sh+"/off/planner/withheld"] < 10 {
			t.Errorf("%s gate-off planner withheld: %d, want at least 10", sh, counts[sh+"/off/planner/withheld"])
		}
	}
}

// TestGatedManaAbilityMutation (e): the gate treated as always on must diverge.
func TestGatedManaAbilityMutation(t *testing.T) {
	set, build := gatedmanaabilityEnv(t)
	dirac.SetDebugEntersTappedMutate(5)
	defer dirac.SetDebugEntersTappedMutate(0)
	diverged := false
	for mi := range gatedmanaabilityDecks {
		md := gatedmanaabilityDecks[mi]
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= 128 && !diverged; seed++ {
			m, err := build(mi)
			if err != nil {
				t.Fatal(err)
			}
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, gatedmanaabilityPolicy(seed, map[string]int{}), RunConfig{Config: cfg, MaxDecisions: 1500, MaxTurns: 16})
			CloseSide(k)
			if res.Div != nil {
				t.Logf("gate always on: matchup %d seed %d diverged at decision %d", mi, seed, res.Div.At)
				diverged = true
			}
		}
	}
	if !diverged {
		t.Errorf("gate always on: mutation diverged in no game")
	}
}

// gatedmanaabilityTabletBelow2: a Tablet of Compleation on the battlefield holds fewer
// than two oil counters.
func gatedmanaabilityTabletBelow2(g *state.Game) bool {
	for _, o := range g.Objs {
		if o.Face() != nil && o.Zone == state.ZBattlefield && o.Face().Name == "Tablet of Compleation" && o.Counter("OIL") < 2 {
			return true
		}
	}
	return false
}

// gatedmanaabilityTabletReady: an untapped Tablet of Compleation holds two or more oil.
func gatedmanaabilityTabletReady(g *state.Game) bool {
	for _, o := range g.Objs {
		if o.Face() != nil && o.Zone == state.ZBattlefield && o.Face().Name == "Tablet of Compleation" && !o.Tapped && o.Counter("OIL") >= 2 {
			return true
		}
	}
	return false
}
