package oracle

import (
	"regexp"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The twobrid lockstep (CR 107.4e, 601.2b, 202.3f): constructed
// matchups over the generated twobrid pool. Gorge prices no twobrid cast but
// a floating pool's (the cast planner and the potential planner both refuse
// "cost:twobrid"), so the {X} spells leave mana floating: a policy casts one
// with the smallest X while it holds a twobrid card.
//
// The counters come from gorge's decisions and log, where the branch is taken.
var ktwobridDecks = [2][]cardset.Entry{
	{
		{Name: "Temur Tawnyback", Count: 3}, {Name: "Dragonclaw Strike", Count: 3},
		{Name: "Monastery Messenger", Count: 2}, {Name: "Blaze", Count: 4},
		{Name: "Mind Spring", Count: 3}, {Name: "Stream of Life", Count: 3},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Goblin Electromancer", Count: 2}, {Name: "Opt", Count: 2},
		{Name: "Island", Count: 5}, {Name: "Mountain", Count: 5}, {Name: "Forest", Count: 5},
	},
	{
		{Name: "Gurmag Nightwatch", Count: 3}, {Name: "Rakshasa's Bargain", Count: 2},
		{Name: "Kin-Tree Severance", Count: 3}, {Name: "Reputable Merchant", Count: 3},
		{Name: "Defibrillating Current", Count: 2}, {Name: "Karn, Gilded Guardian", Count: 1},
		{Name: "Eliminate", Count: 3}, {Name: "Despark", Count: 2},
		{Name: "Exsanguinate", Count: 4}, {Name: "Stream of Life", Count: 3},
		{Name: "Plains", Count: 5}, {Name: "Swamp", Count: 5}, {Name: "Forest", Count: 4},
	},
}

var tbPip = regexp.MustCompile(`\b2[WUBRG]\b`)

// tbCard reports whether the face's printed cost has a twobrid pip.
func tbCard(f *cards.Face) bool { return f != nil && tbPip.MatchString(f.ManaCost) }

// tbPayable reports whether some face assignment of the printed cost is paid
// by the mana (W U B R G C slots; every source makes one colour).
func tbPayable(cost string, m [6]int) bool {
	var need [5]int
	gen, tb := 0, []int{}
	for _, t := range strings.Fields(cost) {
		switch {
		case len(t) == 2 && t[0] == '2' && strings.IndexByte("WUBRG", t[1]) >= 0:
			tb = append(tb, strings.IndexByte("WUBRG", t[1]))
		case len(t) == 1 && strings.IndexByte("WUBRG", t[0]) >= 0:
			need[strings.IndexByte("WUBRG", t[0])]++
		default:
			n := 0
			for _, ch := range t {
				n = n*10 + int(ch-'0')
			}
			gen += n
		}
	}
	total := 0
	for _, v := range m {
		total += v
	}
	for mask := 0; mask < 1<<len(tb); mask++ {
		n, g := need, gen
		for i, c := range tb {
			if mask>>i&1 != 0 {
				g += 2
			} else {
				n[c]++
			}
		}
		ok, sum := true, g
		for c := 0; c < 5; c++ {
			sum += n[c]
			if n[c] > m[c] {
				ok = false
			}
		}
		if ok && sum <= total {
			return true
		}
	}
	return false
}

// ktwobridTally counts the twobrid routes.
type ktwobridTally struct {
	autopay bool
	ref     *GorgeSide

	colChosen, genChosen, mixed int
	genWithheld, colWithheld    int
	unpriced                    int
	resolved, entered, karn     int
	mvLE3, mvGE4, handDemand    int
	autopayOffers, floatOffers  int
	pipAsks                     int
	curCol, curGen              bool
	perCard                     map[string]int
}

func (o *ktwobridTally) start(cfg Config) {
	o.autopay = cfg.GorgeAutopaySurface
	o.ref = nil
	o.curCol, o.curGen = false, false
	if o.perCard == nil {
		o.perCard = map[string]int{}
	}
}

// observe counts one posed decision and the answer the policy gave.
func (o *ktwobridTally) observe(c *PickCtx, a Canon) {
	if c.G == nil || c.G.fr == nil || c.G.fr.d == nil {
		return
	}
	o.ref = c.G
	g := c.G.E.G
	d := c.G.fr.d
	pay, _ := chooseCode("pay")
	isPip := len(c.D.Options) > 0
	for _, op := range c.D.Options {
		if op.Kind() != ActChoose+CanonKind(pay) {
			isPip = false
		}
	}
	if !isPip && o.curCol && o.curGen {
		o.mixed++
	}
	if !isPip {
		o.curCol, o.curGen = false, false
	}
	if isPip {
		o.pipAsks++
		has6, hasCol := false, false
		for _, op := range c.D.Options {
			if op.Mode() == 6 {
				has6 = true
			} else {
				hasCol = true
			}
		}
		if has6 && !hasCol {
			o.colWithheld++
		}
		if hasCol && !has6 {
			o.genWithheld++
		}
		if a.Mode() == 6 {
			o.genChosen++
			o.curGen = true
		} else {
			o.colChosen++
			o.curCol = true
		}
		return
	}
	actor := state.PlayerID(d.Player)
	switch d.Kind {
	case decision.KPriority:
		var mana [6]int
		for i, v := range g.Players[actor].Pool {
			mana[i] = int(v)
		}
		pool := mana
		for _, id := range g.Zone(state.ZBattlefield, actor) {
			ob := g.Obj(id)
			if ob == nil || ob.Tapped || ob.Face() == nil || !ob.Face().IsBasic() {
				continue
			}
			mana[state.ManaIndex(basicColour(ob.Face().Name))]++
		}
		castable := map[state.ObjID]bool{}
		for i := range d.Options {
			if d.Options[i].Kind == "cast" {
				castable[d.Options[i].Obj] = true
			}
		}
		sorc := g.Active == actor && g.Step.IsMain() && len(g.Stack) == 0
		holdsTb := false
		for _, id := range g.Zone(state.ZHand, actor) {
			ob := g.Obj(id)
			if ob == nil || !tbCard(ob.Face()) {
				continue
			}
			holdsTb = true
			if castable[id] {
				o.floatOffers++
				if o.autopay {
					o.autopayOffers++
				}
				continue
			}
			f := ob.Face()
			// a creature spell needs only its timing: a frame where the
			// sources could pay it, the floating pool cannot, and gorge
			// lists no cast is the unpriced one
			if !o.autopay && f.IsCreature() && sorc && !tbPayable(f.ManaCost, pool) && tbPayable(f.ManaCost, mana) {
				o.unpriced++
			}
		}
		if holdsTb && a.Kind() == ActCast {
			if ob := canonObj(c, a.Src()); ob != nil && ob.Face() != nil && strings.Contains(ob.Face().ManaCost, "X") {
				o.handDemand++
			}
		}
	case decision.KTarget:
		if len(g.Stack) == 0 {
			return
		}
		top := g.Obj(g.Stack[len(g.Stack)-1])
		if top == nil || top.Face() == nil {
			return
		}
		inOpts := func(id state.ObjID) bool {
			for i := range d.Options {
				if d.Options[i].Obj == id {
					return true
				}
			}
			return false
		}
		switch top.Face().Name {
		case "Eliminate":
			for p := range g.Players {
				for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
					if ob := g.Obj(id); ob != nil && tbCard(ob.Face()) && (ob.Face().IsCreature() || ob.Face().IsPlaneswalker()) && !inOpts(id) {
						o.mvLE3++
						return
					}
				}
			}
		case "Despark", "Kin-Tree Severance":
			for i := range d.Options {
				if ob := g.Obj(d.Options[i].Obj); ob != nil && ob.Zone == state.ZBattlefield && tbCard(ob.Face()) {
					o.mvGE4++
					return
				}
			}
		}
	}
}

func basicColour(name string) byte {
	switch name {
	case "Plains":
		return 'W'
	case "Island":
		return 'U'
	case "Swamp":
		return 'B'
	case "Mountain":
		return 'R'
	case "Forest":
		return 'G'
	}
	return 'C'
}

// scan reads the finished game's event log.
func (o *ktwobridTally) scan() {
	if o.ref == nil {
		return
	}
	g := o.ref.E.G
	for i := range o.ref.E.L.Events {
		e := &o.ref.E.L.Events[i]
		ob := g.Obj(e.Obj)
		if ob == nil || !tbCard(ob.Face()) {
			continue
		}
		switch e.Kind {
		case events.PutOnStack:
			o.perCard[ob.Face().Name]++
			if ob.Face().Name == "Karn, Gilded Guardian" {
				o.karn++
			}
		case events.Resolve:
			o.resolved++
		case events.MoveZone:
			if e.To == state.ZBattlefield {
				o.entered++
			}
		}
	}
}

// ktwobridPolicy casts a twobrid card when offered; else, holding one with
// nothing floating, an {X} spell with the smallest X; else the seeded random
// policy. It only orders legal options.
func ktwobridPolicy(seed uint64, tally *ktwobridTally) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		a := ktwobridPick(c, rnd)
		if tally != nil {
			tally.observe(c, a)
		}
		return a
	})
}

func ktwobridPick(c *PickCtx, rnd Policy) Canon {
	if c.G == nil {
		return rnd.Pick(c)
	}
	g := c.G.E.G
	p := state.PlayerID(c.D.Player)
	for _, a := range c.D.Options {
		if a.Kind() != ActCast {
			continue
		}
		if o := canonObj(c, a.Src()); o != nil && tbCard(o.Face()) {
			return a
		}
	}
	holds := false
	for _, id := range g.Zone(state.ZHand, p) {
		if o := g.Obj(id); o != nil && tbCard(o.Face()) {
			holds = true
		}
	}
	if holds && g.Players[p].Pool.Total() == 0 {
		xc, _ := chooseCode("x")
		var small Canon
		haveX := false
		for _, a := range c.D.Options {
			switch {
			case a.Kind() == ActChoose+CanonKind(xc):
				if !haveX || a.Mode() < small.Mode() {
					small, haveX = a, true
				}
			case a.Kind() == ActCast:
				if o := canonObj(c, a.Src()); o != nil && o.Face() != nil && strings.Contains(o.Face().ManaCost, "X") {
					return a
				}
			}
		}
		if haveX {
			return small
		}
	}
	return rnd.Pick(c)
}

func ktwobridSetup(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card, *kop.Table) {
	t.Helper()
	cardsettest.Require(t, "twobrid")
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("twobrid")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range ktwobridDecks {
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
	tbl, _, _, err := set.Matchup(ktwobridDecks[0], ktwobridDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, decks, tbl
}

func ktwobridRun(t *testing.T, reg *cards.Registry, set cardset.Set, decks [2][]*cards.Card, tbl *kop.Table, seed uint64, cfg Config, tally *ktwobridTally) Result {
	t.Helper()
	m, err := NewMatchup(reg, [2]string{"twobrid-a", "twobrid-b"}, decks, 1)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(set, tbl, cfg)(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	var pol Policy = ktwobridPolicy(seed, tally)
	if seed%4 == 1 {
		pol = RandomPolicy(seed)
	}
	if tally != nil {
		tally.start(cfg)
	}
	res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 5000, MaxTurns: 80})
	if tally != nil {
		tally.scan()
	}
	CloseSide(k)
	return res
}

func ktwobridCfg(seed uint64) Config {
	if seed%2 == 0 {
		return Config{GorgeAutopaySurface: true}
	}
	return Config{}
}

// TestTwobridLockstep: the kernel matches gorge over the twobrid pool, and
// every branch is reached.
func TestTwobridLockstep(t *testing.T) {
	t.Parallel()

	reg, set, decks, tbl := ktwobridSetup(t)
	var tally ktwobridTally
	const seeds = 300
	for seed := uint64(1); seed <= seeds; seed++ {
		res := ktwobridRun(t, reg, set, decks, tbl, seed, ktwobridCfg(seed), &tally)
		if res.Stall != "" {
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d seeds: pip asks %d; colour face chosen %d, generic face chosen %d, mixed casts %d; generic withheld %d, colour withheld %d; H1 withheld while unpriced %d; offers with floating mana %d (%d autopay); resolved %d, entered %d; MV cmcLE3 %d, cmcGE4 %d; hand demand %d; Karn casts %d; per card %v",
		seeds, tally.pipAsks, tally.colChosen, tally.genChosen, tally.mixed, tally.genWithheld, tally.colWithheld,
		tally.unpriced, tally.floatOffers, tally.autopayOffers, tally.resolved, tally.entered,
		tally.mvLE3, tally.mvGE4, tally.handDemand, tally.karn, tally.perCard)
	for _, c := range []struct {
		name string
		got  int
	}{
		{"colour face chosen", tally.colChosen},
		{"generic face chosen", tally.genChosen},
		{"mixed cast", tally.mixed},
		{"generic face withheld", tally.genWithheld},
		{"colour face withheld", tally.colWithheld},
		{"withheld while unpriced", tally.unpriced},
		{"twobrid spells resolved", tally.resolved},
		{"twobrid permanents entered", tally.entered},
		{"MV cmcLE3 (Eliminate leaves a twobrid permanent out)", tally.mvLE3},
		{"MV cmcGE4 (Despark or Kin-Tree offers a twobrid permanent)", tally.mvGE4},
		{"hand demand", tally.handDemand},
	} {
		if c.got < 10 {
			t.Errorf("%s: %d, want at least 10", c.name, c.got)
		}
	}
	if tally.autopayOffers != 0 {
		t.Errorf("twobrid cast offers on autopay-surface seeds: %d, want 0", tally.autopayOffers)
	}
}

// ktwobridMutate returns tbl with every twobrid cost rewritten by f (a table
// edit: the generated table is copied first).
func ktwobridMutate(tbl *kop.Table, f func(c *kop.ManaCost)) *kop.Table {
	t2 := *tbl
	t2.Cards = append([]kop.CardDef(nil), tbl.Cards...)
	for i := range t2.Cards {
		if t2.Cards[i].Cost.Tb != 0 {
			f(&t2.Cards[i].Cost)
		}
	}
	return &t2
}

// TestTwobridMutation: each table mutation must diverge on some seed.
func TestTwobridMutation(t *testing.T) {
	t.Parallel()

	reg, set, decks, tbl := ktwobridSetup(t)
	muts := []struct {
		name string
		f    func(c *kop.ManaCost)
	}{
		{"every twobrid pip as its colour", func(c *kop.ManaCost) {
			for i, n := 0, c.TbN(); i < n; i++ {
				c.Pips[c.TbCol(i)]++
			}
			c.Tb, c.Hyb[0] = 0, 0
		}},
		{"every twobrid pip as plain {2}", func(c *kop.ManaCost) {
			c.Generic += 2 * uint8(c.TbN())
			c.Tb, c.Hyb[0] = 0, 0
		}},
		{"the pips in reversed order", func(c *kop.ManaCost) {
			var r uint16
			for i, n := 0, c.TbN(); i < n; i++ {
				r |= uint16(c.TbCol(n-1-i)+1) << (3 * uint(i))
			}
			c.Tb = r
		}},
	}
	for _, mu := range muts {
		mt := ktwobridMutate(tbl, mu.f)
		diverged := false
		for seed := uint64(1); seed <= 300 && !diverged; seed++ {
			res := ktwobridRun(t, reg, set, decks, mt, seed, ktwobridCfg(seed), nil)
			if diverged = res.Div != nil; diverged {
				t.Logf("mutation %q: seed %d diverged at decision %d: %s %s", mu.name, seed, res.Div.At, res.Div.What, res.Div.Field)
			}
		}
		if !diverged {
			t.Errorf("mutation %q never diverged", mu.name)
		}
	}
}
