package oracle

import (
	"math/rand/v2"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The copy-ability amount lockstep (CR 707.10): Gogo, Master of Mimicry copies
// an ability of its controller X times, the copies' new targets chosen one
// copy at a time, and its own ability can't be copied; Mister Fantastic copies
// a triggered ability twice. Echo, Perceptive Prodigy is the second copier, so
// a Gogo ability is also the target of a copy that is not made.

var copyabAmtDecks = [2][]cardset.Entry{
	{ // A: U/R
		{Name: "Gogo, Master of Mimicry", Count: 4}, {Name: "Echo, Perceptive Prodigy", Count: 2}, {Name: "Blaze", Count: 5},
		{Name: "Prodigal Sorcerer", Count: 4}, {Name: "Man-o'-War", Count: 2}, {Name: "Rod of Ruin", Count: 2},
		{Name: "Prodigal Pyromancer", Count: 1}, {Name: "Fire Imp", Count: 1}, {Name: "Shock", Count: 1}, {Name: "Lightning Bolt", Count: 1},
		{Name: "Island", Count: 9}, {Name: "Mountain", Count: 8},
	},
	{ // B: four colours for Mister Fantastic
		{Name: "Gogo, Master of Mimicry", Count: 3}, {Name: "Mister Fantastic", Count: 2}, {Name: "Echo, Perceptive Prodigy", Count: 1},
		{Name: "Blaze", Count: 5}, {Name: "Prodigal Sorcerer", Count: 4}, {Name: "Man-o'-War", Count: 1}, {Name: "Flametongue Kavu", Count: 1},
		{Name: "Rod of Ruin", Count: 2}, {Name: "Prodigal Pyromancer", Count: 1},
		{Name: "Island", Count: 7}, {Name: "Mountain", Count: 5}, {Name: "Plains", Count: 4}, {Name: "Forest", Count: 4},
	},
}

type copyabAmtStats struct {
	copies          int // ability copies gorge made
	multi           int // resolutions that made two or more copies
	multiRetargeted int // ... of which a copy was given a new target
	cantCopy        int // Echo or Gogo resolutions aimed at a Gogo ability that made no copy
	gogoRes         int // Gogo ability resolutions
	asks            int
	echoAimed       int // Echo activations aimed at a Gogo ability
}

// copyabAmtProbe is copyabProbe's counterpart for the amount: it reads the
// copies off gorge's event log, attributing each to the ability resolving when
// it was minted.
type copyabAmtProbe struct {
	st        *copyabAmtStats
	base      Policy
	r         *rand.Rand
	seen      int
	cur       state.ObjID
	copies    map[state.ObjID]int
	retarg    map[state.ObjID]int
	aimed     map[state.ObjID]bool // resolutions aimed at a Gogo ability
	xcode     uint8
	echoReady bool
	pickedAim map[state.ObjID]bool // Echo abilities given a Gogo ability as their target
}

func copyabAmtSrcName(g *state.Game, o *state.Object) string {
	if o == nil {
		return ""
	}
	if o.Ability == nil && o.Face() != nil {
		return o.Face().Name // a card: the spell or permanent itself
	}
	if src := g.Obj(o.Source); src != nil && src.Face() != nil {
		return src.Face().Name
	}
	return ""
}

func (p *copyabAmtProbe) scan(g *state.Game, evs []events.Event) {
	for ; p.seen < len(evs); p.seen++ {
		ev := evs[p.seen]
		switch ev.Kind {
		case events.Resolve:
			o := g.Obj(ev.Obj)
			if o == nil || o.Ability == nil {
				p.cur = 0
				continue
			}
			p.cur = ev.Obj
			if p.pickedAim[ev.Obj] {
				p.aimed[ev.Obj] = true
			}
			name := copyabAmtSrcName(g, o)
			if name == "Gogo, Master of Mimicry" {
				p.st.gogoRes++
			}
			if (name == "Gogo, Master of Mimicry" || name == "Echo, Perceptive Prodigy") && len(o.Targets) > 0 {
				if t := g.Obj(o.Targets[0].Obj); t != nil && copyabAmtSrcName(g, t) == "Gogo, Master of Mimicry" {
					p.aimed[ev.Obj] = true
				}
			}
		case events.StackCopy:
			if o := g.Obj(ev.Obj); o != nil && o.Ability != nil && p.cur != 0 {
				p.copies[p.cur]++
				p.st.copies++
			}
		}
	}
}

func (p *copyabAmtProbe) finish() {
	for id, n := range p.copies {
		if n >= 2 {
			p.st.multi++
			if p.retarg[id] > 0 {
				p.st.multiRetargeted++
			}
		}
	}
	for id := range p.aimed {
		if p.copies[id] == 0 {
			p.st.cantCopy++
		}
	}
}

var copyabAmtCopiers = map[string]bool{"Gogo, Master of Mimicry": true, "Echo, Perceptive Prodigy": true, "Mister Fantastic": true}

func (p *copyabAmtProbe) pick(c *PickCtx) Canon {
	a := p.base.Pick(c)
	g := c.G
	if g == nil {
		return a
	}
	gm := g.E.G
	p.scan(gm, g.E.L.Events)
	// the options by what they play: a Gogo or other copier activation, an
	// activation of a pinger the copiers copy, a cast of a creature that
	// carries one of the abilities
	var gogo, echo, pingers, casts, blaze []Canon
	for _, x := range c.D.Options {
		o, ok := g.GorgeOption(x)
		if !ok {
			continue
		}
		so := gm.Obj(o.Obj)
		if so == nil || so.Face() == nil {
			continue
		}
		name := so.Face().Name
		switch {
		case x.Kind() == ActActivate && name == "Gogo, Master of Mimicry":
			gogo = append(gogo, x)
		case x.Kind() == ActActivate && copyabAmtCopiers[name]:
			echo = append(echo, x)
		case x.Kind() == ActActivate:
			pingers = append(pingers, x)
		case x.Kind() == ActCast && name == "Blaze":
			blaze = append(blaze, x)
		case x.Kind() == ActCast && (copyabAmtCopiers[name] || name == "Prodigal Sorcerer" || name == "Man-o'-War" || name == "Flametongue Kavu"):
			casts = append(casts, x)
		}
	}
	// Gogo's {X}{X} is offered from the floating pool alone (gorge prices
	// no X activation from lands), so a Blaze cast with a small X leaves
	// the mana for it.
	ready := false
	echoReady := false
	lands := 0 // untapped lands: Blaze floats them all
	for _, id := range gm.Zone(state.ZBattlefield, state.PlayerID(c.D.Player)) {
		if o := gm.Obj(id); o != nil && o.Face() != nil && o.Face().IsLand() && !o.Tapped {
			lands++
		}
	}
	for _, id := range gm.Zone(state.ZBattlefield, state.PlayerID(c.D.Player)) {
		if o := gm.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Gogo, Master of Mimicry" && !o.Tapped && !o.SummonSick {
			ready = true
		}
		if o := gm.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Echo, Perceptive Prodigy" && !o.Tapped && !o.SummonSick {
			echoReady = true
		}
	}
	p.echoReady = echoReady
	if c.D.Kind == DPriority {
		mine := false // one of this seat's abilities is on the stack
		for _, id := range gm.Stack {
			if o := gm.Obj(id); o != nil && o.Ability != nil && int(o.Controller) == int(c.D.Player) {
				mine = true
			}
		}
		onGogo := false // a Gogo ability is on the stack
		for _, id := range gm.Stack {
			if o := gm.Obj(id); o != nil && o.Ability != nil && copyabAmtSrcName(gm, o) == "Gogo, Master of Mimicry" {
				onGogo = true
			}
		}
		switch {
		case mine && len(gogo) > 0 && p.r.IntN(10) != 0:
			a = gogo[p.r.IntN(len(gogo))]
		case mine && len(echo) > 0 && (onGogo || p.r.IntN(3) != 0):
			a = echo[p.r.IntN(len(echo))]
		case !mine && len(pingers) > 0 && gm.Players[c.D.Player].Pool.Total() >= 2 && p.r.IntN(10) != 0:
			a = pingers[p.r.IntN(len(pingers))]
		case ready && len(pingers) > 0 && len(blaze) > 0 && (lands >= 5 || !echoReady && lands >= 3) && p.r.IntN(10) != 0:
			a = blaze[p.r.IntN(len(blaze))]
		case len(casts) > 0 && p.r.IntN(10) < 7:
			a = casts[p.r.IntN(len(casts))]
		}
	}
	if c.D.Kind == DTarget && len(gm.Stack) > 0 {
		// Echo's own activation is choosing its target (it waits on top of the
		// stack, targetless): a Gogo ability, when one is offered, is the copy
		// that is not made
		top := gm.Obj(gm.Stack[len(gm.Stack)-1])
		if pd := g.Pending(); top != nil && top.Ability != nil && len(top.Targets) == 0 && copyabAmtSrcName(gm, top) == "Echo, Perceptive Prodigy" &&
			(pd == nil || pd.ResumeKind != "copy_targets") {
			for _, x := range c.D.Options {
				if o, ok := g.GorgeOption(x); ok && o.Kind == "ability" && copyabAmtSrcName(gm, gm.Obj(o.Obj)) == "Gogo, Master of Mimicry" {
					a = x
					p.st.echoAimed++
					p.pickedAim[gm.Stack[len(gm.Stack)-1]] = true
				}
			}
		}
	}
	if c.D.Kind == DChoose && len(c.D.Options) > 0 && c.D.Options[0].Kind() == ActChoose+CanonKind(p.xcode) {
		if pd := g.Pending(); pd != nil {
			switch copyabAmtSrcName(gm, gm.Obj(pd.Source)) {
			case "Blaze":
				a = c.D.Options[0] // the smallest X leaves the most mana floating
			case "Gogo, Master of Mimicry":
				// the smallest X leaves mana floating for Echo's {1}
				if p.echoReady {
					a = c.D.Options[0]
				} else if p.r.IntN(2) == 0 {
					a = c.D.Options[len(c.D.Options)-1]
				}
			}
		}
	}
	if d := g.Pending(); d != nil && d.ResumeKind == "copy_targets" {
		p.st.asks++
		if o, ok := g.GorgeOption(a); ok && len(gm.Stack) > 0 {
			if top := gm.Obj(gm.Stack[len(gm.Stack)-1]); top != nil && top.Ability != nil && o.Index >= len(top.Targets) {
				p.retarg[p.cur]++
			}
		}
	}
	return a
}

// copyabAmtSetup builds the matchup's corpus decks and table.
func copyabAmtSetup(t *testing.T) (*cards.Registry, cardset.Set, *kop.Table, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("copyab")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range copyabAmtDecks {
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
	tbl, _, _, err := set.Matchup(copyabAmtDecks[0], copyabAmtDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, tbl, decks
}

// copyabAmtRun plays seeds [from, to] and returns whether a game diverged, the
// kernel stall count and the probe's tallies.
func copyabAmtRun(t *testing.T, from, to uint64, report bool) (diverged bool, stalls int, st copyabAmtStats) {
	t.Helper()
	reg, set, tbl, decks := copyabAmtSetup(t)
	for seed := from; seed <= to; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"copyab-amt-a", "copyab-amt-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		xcode, _ := chooseCode("x")
		probe := &copyabAmtProbe{st: &st, xcode: xcode, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x61746f6d)),
			copies: map[state.ObjID]int{}, retarg: map[state.ObjID]int{}, aimed: map[state.ObjID]bool{}, pickedAim: map[state.ObjID]bool{}}
		res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		probe.finish()
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
		}
		if d := res.Div; d != nil {
			diverged = true
			if report {
				lo := max(d.At-12, 0)
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			return
		}
	}
	return
}

func TestCopyabAmountLockstep(t *testing.T) {
	t.Parallel()
	div, stalls, st := copyabAmtRun(t, 1, 1024, true)
	t.Logf("1024 games, copyab amount %+v, %d divergences, %d stalls", st, b2i(div), stalls)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	for _, c := range []struct {
		n, min int
		s      string
	}{
		{st.gogoRes, 20, "Gogo ability resolutions"},
		{st.multi, 10, "resolutions that made 2+ ability copies"},
		{st.multiRetargeted, 5, "such resolutions with a copy given a new target"},
		{st.cantCopy, 2, "Echo or Gogo resolutions aimed at a Gogo ability that made no copy"},
	} {
		if c.n < c.min {
			t.Errorf("%s: %d, want at least %d", c.s, c.n, c.min)
		}
	}
}

// TestCopyabAmountMutation: a copy that makes one copy whatever Amount$ says,
// and one that copies an ability that can't be copied, must each make the
// lockstep diverge.
func TestCopyabAmountMutation(t *testing.T) {
	for _, m := range []struct {
		name string
		flag *bool
	}{
		{"DebugCopyAbOneCopy", &dirac.DebugCopyAbOneCopy},
		{"DebugCopyAbIgnoreCantCopy", &dirac.DebugCopyAbIgnoreCantCopy},
	} {
		t.Run(m.name, func(t *testing.T) {
			*m.flag = true
			defer func() { *m.flag = false }()
			if diverged, _, _ := copyabAmtRun(t, 1, 1024, false); !diverged {
				t.Fatalf("%s: no divergence in 1024 seeds", m.name)
			}
		})
	}
}

// TestGogoReproCopiesAbilityXTimes replays the recorded game and requires it to
// reach what its name says: a Gogo ability resolving with its X of two or more,
// and making that many copies of the targeted ability. The lockstep entry in
// TestStdSetReprosLockstep alone would accept a script that never got there.
func TestGogoReproCopiesAbilityXTimes(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	r, err := ReadRepro(filepath.Join("testdata", "repro", "gogo_copies_ability_x_times.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	var st copyabAmtStats
	probe := &copyabAmtProbe{st: &st, copies: map[state.ObjID]int{}, retarg: map[state.ObjID]int{}, aimed: map[state.ObjID]bool{}, pickedAim: map[state.ObjID]bool{}}
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + 200
	rc.Check = func(ref *GorgeSide, _ Side) *Divergence {
		probe.scan(ref.E.G, ref.E.L.Events)
		return nil
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	set := reproSet(t, "custom", r)
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	res := Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	probe.finish()
	if st.gogoRes < 1 || st.multi < 1 {
		t.Errorf("Gogo resolutions %d, resolutions with 2+ copies %d: want at least one of each", st.gogoRes, st.multi)
	}
}
