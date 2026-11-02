package oracle

import (
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Endure lockstep (CR 701.63; gorge effEndure): the nine
// TDM carriers against a Shock deck. Matchup 0 is W/B (Anafenza's death
// trigger, Descendant's paid attack trigger, the enter triggers, Sinkhole's
// sub-ability); matchup 1 is G/B with Warden of the Grove (Endure X, X its
// counters, X = 0 a no-op) and Hardened Scales on the counters arm. Shock
// and Disfigure are cast at a creature whenever they can, so a carrier
// often dies with its trigger on the stack (the Spirit-only branch).
var kendureB = []cardset.Entry{
	{Name: "Grizzly Bears", Count: 6}, {Name: "Shock", Count: 6}, {Name: "Llanowar Elves", Count: 4},
	{Name: "Sandskitter Outrider", Count: 2}, {Name: "Inspirited Vanguard", Count: 2},
	{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
}

var kendureDecks = [][2][]cardset.Entry{{
	{
		{Name: "Fortress Kin-Guard", Count: 4}, {Name: "Descendant of Storms", Count: 4},
		{Name: "Anafenza, Unyielding Lineage", Count: 3}, {Name: "Kin-Tree Nurturer", Count: 4},
		{Name: "Sinkhole Surveyor", Count: 3}, {Name: "Disfigure", Count: 4}, {Name: "Ornithopter", Count: 2},
		{Name: "Plains", Count: 10}, {Name: "Swamp", Count: 6},
	},
	kendureB,
}, {
	{
		{Name: "Warden of the Grove", Count: 4}, {Name: "Dusyut Earthcarver", Count: 2},
		{Name: "Kin-Tree Nurturer", Count: 4}, {Name: "Inspirited Vanguard", Count: 4},
		{Name: "Hardened Scales", Count: 3}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Sandskitter Outrider", Count: 3}, {Name: "Forest", Count: 10}, {Name: "Swamp", Count: 6},
	},
	kendureB,
}}

// kendurePolicy answers the endure ask by seed parity (even: counters),
// casts Shock or Disfigure whenever it can, aims a target at a creature
// when one is offered, and otherwise plays like pkPolicy.
func kendurePolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	name := func(c *PickCtx, r Ref) string {
		if c.G == nil || int(r) >= int(RefPlayer) {
			return ""
		}
		if o := c.G.E.G.Obj(c.G.id.obj[r]); o != nil && o.Face() != nil {
			return o.Face().Name
		}
		return ""
	}
	sub := func(c *PickCtx) Canon {
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		switch c.D.Kind {
		case DChoose:
			want := "endure_spirit"
			if seed%2 == 0 {
				want = "endure_counters"
			}
			for _, a := range c.D.Options {
				if code, _ := chooseCode(want); a.Kind() == ActChoose+CanonKind(code) {
					return a
				}
			}
		case DPriority:
			for _, a := range c.D.Options {
				if a.Kind() == ActCast {
					if n := name(c, a.Src()); n == "Shock" || n == "Disfigure" {
						return a
					}
				}
			}
			buf = buf[:0]
			for _, a := range c.D.Options {
				if k := a.Kind(); k == ActCast || k == ActPlayLand {
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 && seed%4 != 0 {
				return sub(c)
			}
		case DTarget:
			// a creature whose Endure trigger is on the stack first: it
			// leaves before the trigger resolves (the Spirit-only branch)
			if c.G != nil {
				g := c.G.E.G
				for _, a := range c.D.Options {
					if a.Kind() != ActTarget || int(a.Tgt()) >= int(RefPlayer) {
						continue
					}
					id := c.G.id.obj[a.Tgt()]
					for _, si := range g.Stack {
						if it := g.Obj(si); it != nil && it.Source == id && it.Ability != nil && it.Ability.API == "Endure" {
							return a
						}
					}
				}
			}
			buf = buf[:0]
			for _, a := range c.D.Options {
				if a.Kind() == ActTarget && int(a.Tgt()) < int(RefPlayer) {
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 {
				return sub(c)
			}
		}
		return rnd.Pick(c)
	})
}

// kendureTally counts the endure branches from gorge's side: E-C counters
// chosen, E-S the Spirit chosen (its power N), E-G the recipient gone (a
// Spirit minted with no ask), E-H Hardened Scales on the counters arm, E-0
// a Warden trigger resolving with X = 0 (logged only).
type kendureTally struct {
	ec, es, eg, eh, e0, games int
	bad                       []string

	evAt int
	// the posed ask awaiting its outcome
	ask             bool
	obj             state.ObjID
	n               int32 // -1: a non-literal Num$ (Warden's X)
	before, scales  int32
	objsAt, wardenS int
	fx              int // the game's peak battlefield Spirits (one fx record each)
}

func (p *kendureTally) reset() {
	p.evAt, p.ask, p.wardenS, p.fx = 0, false, -1, 0
}

func (p *kendureTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		if e.Kind != events.TokenCreate || e.Text != "w_x_x_spirit" {
			continue
		}
		if !p.ask {
			p.eg++
			continue
		}
		p.ask = false
		p.es++
		for i := len(g.Objs) - 1; i >= p.objsAt; i-- {
			ob := &g.Objs[i]
			if ob.IsToken && ob.Zone == state.ZBattlefield && ob.Face() != nil && ob.Face().Name == "Spirit Token" {
				id := state.ObjID(i + 1)
				if pw, tg := ref.E.Power(id), ref.E.Toughness(id); pw != tg || (p.n >= 0 && pw != p.n) {
					p.bad = append(p.bad, "Spirit "+strconv.Itoa(int(pw))+"/"+strconv.Itoa(int(tg))+" for endure "+strconv.Itoa(int(p.n)))
				}
				break
			}
		}
	}
	p.evAt = len(evs)
	sp := 0
	for i := range g.Objs {
		if ob := &g.Objs[i]; ob.IsToken && ob.Zone == state.ZBattlefield && ob.Face() != nil && ob.Face().Name == "Spirit Token" {
			sp++
		}
	}
	p.fx = max(p.fx, sp)
	if p.ask {
		// no Spirit since the ask: the counters arm
		p.ask = false
		if o := g.Obj(p.obj); o != nil && o.Zone == state.ZBattlefield {
			got := o.Counter("P1P1") - p.before
			p.ec++
			if p.scales > 0 {
				p.eh++
			}
			if p.n >= 0 && got != p.n+p.scales {
				p.bad = append(p.bad, "counters +"+strconv.Itoa(int(got))+" for endure "+strconv.Itoa(int(p.n))+" with "+strconv.Itoa(int(p.scales))+" Scales")
			}
		}
	}
	// Warden's trigger on top of the stack with no counters on Warden: X = 0
	if n := len(g.Stack); n > 0 {
		if it := g.Obj(g.Stack[n-1]); it != nil && it.Ability != nil && it.Ability.API == "Endure" && int(g.Stack[n-1]) != p.wardenS {
			p.wardenS = int(g.Stack[n-1])
			if src := g.Obj(it.Source); src != nil && src.Face() != nil && src.Face().Name == "Warden of the Grove" && src.Zone == state.ZBattlefield && src.Counter("ALL") == 0 {
				p.e0++
			}
		}
	}
	d := ref.Pending()
	if d == nil || d.ResumeKind != "endure" || len(d.Options) == 0 {
		return nil
	}
	p.ask = true
	p.obj = d.Options[0].Obj
	p.n = -1
	if d.ResumeSA != nil {
		if v, err := strconv.Atoi(d.ResumeSA.Params["Num"]); err == nil {
			p.n = int32(v)
		}
	}
	p.before, p.scales = 0, 0
	if o := g.Obj(p.obj); o != nil {
		p.before = o.Counter("P1P1")
	}
	for i := range g.Objs {
		ob := &g.Objs[i]
		if ob.Zone == state.ZBattlefield && ob.Controller == d.Player && ob.Face() != nil && ob.Face().Name == "Hardened Scales" {
			p.scales++
		}
	}
	p.objsAt = len(g.Objs)
	return nil
}

func TestEndureLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("endure")
	if err != nil {
		t.Fatal(err)
	}
	const seeds = 400
	var tal [2]kendureTally
	div, stalls := 0, 0
	for mi, md := range kendureDecks {
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
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		tl := &tal[mi]
		for seed := uint64(1); seed <= seeds; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"en-a", "en-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			tl.reset()
			res := Lockstep(m, seed, k, kendurePolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30, Check: tl.check})
			CloseSide(k)
			tl.games++
			if res.Stall == KernelStallPrefix+"tables" {
				// the kernel's effect-record pool (maxFx) is full: every
				// Spirit holds its N/N record, a capacity end
				stalls++
				t.Logf("matchup %d seed %d: capacity stall: %s (peak Spirits %d)", mi, seed, res.Stall, tl.fx)
			} else if res.Stall != "" && res.Stall != "turns" {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				div++
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	var sum kendureTally
	for i := range tal {
		sum.ec += tal[i].ec
		sum.es += tal[i].es
		sum.eg += tal[i].eg
		sum.eh += tal[i].eh
		sum.e0 += tal[i].e0
		sum.games += tal[i].games
		for _, b := range tal[i].bad {
			t.Errorf("matchup %d: %s", i, b)
		}
	}
	t.Logf("%d games, %d divergences, %d capacity stalls: E-C %d, E-S %d, E-G %d, E-H %d, E-0 %d", sum.games, div, stalls, sum.ec, sum.es, sum.eg, sum.eh, sum.e0)
	if sum.ec < 10 || sum.es < 10 || sum.eg < 10 || sum.eh < 10 {
		t.Errorf("endure branches: E-C %d, E-S %d, E-G %d, E-H %d; want at least 10 each", sum.ec, sum.es, sum.eg, sum.eh)
	}
}
