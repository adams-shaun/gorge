package oracle

import (
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: Kroxa, Titan of Death's Hunger's RepeatEach player body and
// its gate. The enter/attack trigger makes each opponent discard a card, then
// each opponent who did not discard a nonland card lose 3 life: the gate SVar
// (Remembered$Valid Card.nonLand+RememberedPlayerOwn) reads 1 exactly when
// that opponent discarded a nonland card, so a land discard costs the life
// and a nonland discard does not. The pool carries cheap discard fillers and
// outlets, the BBB/RRR floaters that pay Kroxa's escape cost (an escaped
// Kroxa survives its enter sacrifice and reaches the attack trigger),
// vanilla attackers and lands.
var krcDecks = [2][]cardset.Entry{
	{ // A: Kroxa, the floaters, discard outlets and vanilla attackers
		{Name: "Kroxa, Titan of Death's Hunger", Count: 3},
		{Name: "Coal Stoker", Count: 4}, {Name: "Dark Ritual", Count: 4},
		{Name: "Tormenting Voice", Count: 4}, {Name: "Thrill of Possibility", Count: 2},
		{Name: "Walking Corpse", Count: 3}, {Name: "Warpath Ghoul", Count: 3},
		{Name: "Barony Vampire", Count: 2}, {Name: "Sign in Blood", Count: 1},
		{Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 6},
	},
	{ // B: land-heavy, so its hand holds lands to discard, and cheap bodies
		{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 4},
		{Name: "Rotting Mastodon", Count: 2}, {Name: "Kami of Old Stone", Count: 3},
		{Name: "Walking Corpse", Count: 3}, {Name: "Barony Vampire", Count: 3},
		{Name: "Catacomb Crocodile", Count: 3}, {Name: "Wandering Tombshell", Count: 2},
		{Name: "Thrill of Possibility", Count: 1}, {Name: "Tormenting Voice", Count: 3},
	},
}

// krcCounts are what gorge's event log showed.
type krcCounts struct {
	etb    int // Kroxa enter triggers that made the opponent discard
	attack int // Kroxa attack triggers that made the opponent discard
	lost   int // iterations where the opponent discarded and lost 3 life (the land arm)
	kept   int // iterations where the opponent discarded and lost nothing (the nonland arm)
}

// kroxaName is the pool's gate carrier.
const kroxaName = "Kroxa, Titan of Death's Hunger"

// krcFloaters are the pool's cards whose entry floats the mana that pays
// Kroxa's escape cost (both of gorge's escape gates price the floating pool:
// its offer walk's castable and the kernel's FeatPoolPays). Coal Stoker
// floats R:3; Dark Ritual's {B} pip is colour-forced, so its B:3 floats
// cannot be eaten by the red ones and the pool ends at B:3,R:3. gorge's
// planned payment route spends a live pool on the next cast's generic pips,
// so Coal Stoker (its generic 3) must go first, into an empty pool.
var krcFloaters = map[string]bool{"Coal Stoker": true, "Dark Ritual": true}

// krcOutlets are the pool's spells that move cards into a graveyard.
var krcOutlets = map[string]bool{"Tormenting Voice": true, "Thrill of Possibility": true}

// krcProbe wraps a seeded random policy biased to take Kroxa's escape cast
// when it is offered, to float mana with the pool's floaters while Kroxa
// waits in the graveyard and to move Kroxa and its five exile mates there,
// so both trigger paths play: the enter trigger from a hand cast (Kroxa then
// dies to its enter sacrifice) and the attack trigger from an escaped Kroxa.
type krcProbe struct {
	base Policy
	r    *rand.Rand
}

func (p *krcProbe) pick(c *PickCtx) Canon {
	a := p.base.Pick(c)
	if c.G == nil {
		return a
	}
	e := c.G.E
	// heavyReady: Kroxa sits in this player's graveyard with the five other
	// cards its escape exiles. Its escape is a sorcery-speed cast the walk
	// offers in a main phase with an empty stack, priced from the untapped
	// census (gorge's planned payment route spends a live floating pool on
	// the next cast's generic pips, so the floats are held unspent), so the
	// probe holds until the escape is offered and keeps the lands untapped.
	grave := e.G.Zone(state.ZGraveyard, state.PlayerID(c.D.Player))
	graveKroxa := map[state.ObjID]bool{}
	heavyReady := false
	for _, id := range grave {
		if ob := e.G.Obj(id); ob != nil && ob.Face() != nil && ob.Face().Name == kroxaName {
			graveKroxa[id] = true
			heavyReady = len(grave) >= 6
		}
	}
	inHand := false
	for _, id := range e.G.Zone(state.ZHand, state.PlayerID(c.D.Player)) {
		if ob := e.G.Obj(id); ob != nil && ob.Face() != nil && ob.Face().Name == kroxaName {
			inHand = true
		}
	}
	var esc, atk, floaters, outlets, disc, passes []Canon
	for _, x := range c.D.Options {
		o, ok := c.G.GorgeOption(x)
		if !ok {
			continue
		}
		name := ""
		if ob := e.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
			name = ob.Face().Name
		}
		switch {
		case o.Kind == "cast" && name == kroxaName && graveKroxa[o.Obj]:
			// the escape cast: gorge's walk offers it with Mode escape; the
			// planned payment route that prices it reports the payment
			// action's empty cast mode, so both shapes are the escape. A
			// Kroxa cast option whose object is not in the grave is a plain
			// hand cast and stays for the base policy.
			esc = append(esc, x)
		case o.Kind == "attacker" && name == kroxaName:
			// the attack trigger: the base policy often declares no
			// attackers, so the probe always sends Kroxa when it can
			atk = append(atk, x)
		case o.Kind == "cast" && krcFloaters[name]:
			floaters = append(floaters, x)
		case o.Kind == "cast" && krcOutlets[name]:
			outlets = append(outlets, x)
		case o.Kind == "discard" && name == kroxaName:
			disc = append(disc, x)
		case o.Kind == "pass":
			passes = append(passes, x)
		}
	}
	// a floater the base policy would cast stays in hand: the floats only
	// matter while Kroxa waits in the graveyard, and gorge's planned payment
	// route spends a live floating pool on the next cast's generic pips, so
	// the escape is priced from the untapped census instead
	for _, x := range floaters {
		if a == x && len(passes) > 0 {
			return passes[0]
		}
	}
	switch {
	case len(esc) > 0:
		return esc[p.r.IntN(len(esc))]
	case len(atk) > 0:
		return atk[p.r.IntN(len(atk))]
	case heavyReady:
		// The floats expire with the step (CR 500.4), so both are built and
		// spent inside one main phase, in cost order: Coal Stoker first, its
		// generic 3 paid from lands while the pool is empty (gorge's planned
		// payment route spends a live pool on generic), then its trigger's
		// R:3, then Dark Ritual -- its {B} pip cannot be paid with the red
		// floats, so the pool ends at B:3,R:3 and the escape is payable.
		// Anything else holds with a pass.
		pool := e.G.Players[c.D.Player].Pool
		coal, ritual := -1, -1
		for i, x := range floaters {
			o, ok := c.G.GorgeOption(x)
			if !ok {
				continue
			}
			ob := e.G.Obj(o.Obj)
			if ob == nil || ob.Face() == nil {
				continue
			}
			switch ob.Face().Name {
			case "Coal Stoker":
				coal = i
			case "Dark Ritual":
				ritual = i
			}
		}
		switch {
		case pool.Total() == 0 && coal >= 0:
			return floaters[coal]
		case pool[state.MR] >= 2 && ritual >= 0:
			return floaters[ritual]
		case len(passes) > 0:
			return passes[0] // hold the floats until the escape is payable
		}
	case (inHand || heavyReady) && e.G.Players[c.D.Player].Pool.Total() == 0 && len(disc) > 0:
		return disc[p.r.IntN(len(disc))] // move Kroxa into the graveyard
	case (inHand || heavyReady) && e.G.Players[c.D.Player].Pool.Total() == 0 && len(outlets) > 0:
		return outlets[p.r.IntN(len(outlets))] // fill the graveyard
	}
	return a
}

// krcScan reads gorge's event log: a Kroxa trigger resolution is an ETB one
// when two or more Kroxa triggers were pushed adjacently (the enter
// sacrifice beside the discard trigger), else an attack one.
func krcScan(g *GorgeSide) krcCounts {
	var c krcCounts
	if g == nil {
		return c
	}
	name := func(id state.ObjID) string {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil {
			return o.Face().Name
		}
		return ""
	}
	type stk struct {
		src state.ObjID
		ab  bool
		// etb is per push, not per source: one Kroxa object can resolve an
		// enter trigger and later an attack one, so a source-keyed mark
		// would swallow the attack trigger of the escaped copy.
		etb bool
	}
	var stack []stk
	adj := 0
	evs := g.E.L.Events
	for i, ev := range evs {
		switch ev.Kind {
		case events.TurnChange:
			stack, adj = stack[:0], 0
		case events.Priority, events.Resolve:
			adj = 0
			if ev.Kind == events.Priority {
				continue
			}
			fallthrough
		case events.TriggerPush, events.AbilityPush:
			if ev.Kind != events.TriggerPush && ev.Kind != events.AbilityPush {
				var top stk
				if ev.Text == "" {
					if len(stack) == 0 {
						continue
					}
					top = stack[len(stack)-1]
					stack = stack[:len(stack)-1]
				} else {
					for len(stack) > 0 && stack[len(stack)-1].src != ev.Obj {
						stack = stack[:len(stack)-1]
					}
					if len(stack) == 0 {
						continue
					}
					top = stack[len(stack)-1]
					stack = stack[:len(stack)-1]
				}
				n := name(top.src)
				j := i + 1
				for j < len(evs) && evs[j].Kind != events.Priority {
					j++
				}
				var discard, lost3 bool
				for _, w := range evs[i+1 : j] {
					switch {
					case w.Kind == events.MoveZone && w.From == state.ZHand && w.To == state.ZGraveyard:
						discard = true
					case w.Kind == events.LifeChange && w.Amount == -3:
						lost3 = true
					}
				}
				// A resolution counts when the opponent discarded (either arm)
				// or lost the 3 life outright -- an empty hand discards
				// nothing and still lands in the land arm.
				if n == "Kroxa, Titan of Death's Hunger" && top.ab && (discard || lost3) {
					if top.etb {
						c.etb++
					} else {
						c.attack++
					}
					if lost3 {
						c.lost++
					} else {
						c.kept++
					}
				}
				continue
			}
			adj++
			cur := stk{src: ev.Obj, ab: true}
			if name(ev.Obj) == kroxaName && adj >= 2 {
				// a pair: the enter sacrifice trigger beside the discard
				// trigger. Mark both pushes (either order) so the discard
				// trigger's resolution reads as the enter one; an attack
				// push is alone (the sacrifice resolved with the entry).
				cur.etb = true
				for j := range stack {
					if stack[j].ab && name(stack[j].src) == kroxaName {
						stack[j].etb = true
					}
				}
			}
			stack = append(stack, cur)
		case events.PutOnStack:
			adj = 0
			stack = append(stack, stk{src: ev.Obj, ab: false})
		}
	}
	return c
}

// krcRun plays seeds 1..seeds and returns the summed counts, the first
// divergence ("" if none) and the kernel stalls.
func krcRun(t *testing.T, seeds uint64) (tot krcCounts, games int, div string, stalls map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("repeatcheck")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range krcDecks {
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
	tbl, _, _, err := set.Matchup(krcDecks[0], krcDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	stalls = map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"krc-a", "krc-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		probe := &krcProbe{base: castFirstPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x6b726378))}
		if seed%8 == 0 {
			probe.base = RandomPolicy(seed)
		}
		wrapped := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
			}
			return probe.pick(c)
		})
		res := Lockstep(m, seed, k, wrapped, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls[res.Stall]++
		}
		c := krcScan(gs)
		tot.etb += c.etb
		tot.attack += c.attack
		tot.lost += c.lost
		tot.kept += c.kept
		if d := res.Div; d != nil {
			lo := max(d.At-8, 0)
			t.Logf("seed %d last actions %v\ndiff %v %s", seed, actionStrings(res.Actions[lo:d.At]), d.Diff, d.Detail)
			return tot, games, d.What + " " + d.Field + " seed " + strconv.FormatUint(seed, 10), stalls
		}
	}
	return tot, games, "", stalls
}

func TestRepeatcheckLockstep(t *testing.T) {
	t.Parallel()
	tot, games, div, stalls := krcRun(t, 400)
	t.Logf("%d games; Kroxa enter triggers %d, attack triggers %d; land arms (3 life lost) %d, nonland arms (no loss) %d; stalls %v",
		games, tot.etb, tot.attack, tot.lost, tot.kept, stalls)
	if div != "" {
		t.Fatalf("diverged: %s", div)
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
	// coverage floor, measured over the 400 seeds (356/109/262/203) with
	// about three times headroom
	if tot.etb < 100 {
		t.Errorf("Kroxa enter triggers %d, want at least 100", tot.etb)
	}
	if tot.attack < 20 {
		t.Errorf("Kroxa attack triggers %d, want at least 20", tot.attack)
	}
	if tot.lost < 100 {
		t.Errorf("land arms %d, want at least 100", tot.lost)
	}
	if tot.kept < 50 {
		t.Errorf("nonland arms %d, want at least 50", tot.kept)
	}
}

// TestRepeatcheckMutation: with ExRemEachOwnNonLand counting land cards too,
// a land discard stops costing the life and the lockstep must diverge within
// 128 seeds.
func TestRepeatcheckMutation(t *testing.T) {
	dirac.DebugRemEachOwnAnyCard = true
	defer func() { dirac.DebugRemEachOwnAnyCard = false }()
	_, games, div, _ := krcRun(t, 128)
	if div == "" {
		t.Fatalf("no divergence in %d games with DebugRemEachOwnAnyCard", games)
	}
	t.Logf("mutation diverged after %d games: %s", games, div)
}
