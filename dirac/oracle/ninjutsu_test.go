package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The ninjutsu lockstep (CR 702.49): constructed
// matchups over the generated ninja pool. gorge expands K:Ninjutsu into a
// hand ability (engine/cards/kw_ninjutsu.go) the kernel lowers as
// kop.AfNinjutsu (dirac/kw_ninjutsu.go). The policy keeps its lands
// untapped through the first main phase so the window has mana, attacks
// with everything and activates every ninjutsu it is offered. The counters
// come from gorge's event log and option lists:
//
//  1. a ninjutsu activation per carrier (AbilityPush of a Keyword$ Ninjutsu
//     ability), 2. the ninja entered tapped ("entered tapped") and then
//     attacking the returned creature's defender ("entered attacking"),
//  3. a planeswalker defender carried over, 4. a ninja that connected,
//  5. a return pick with several candidates, 6. with one (still asked),
//  7. Moon-Circuit Hacker's discard (it did not enter this turn: X = 1),
//  8. its no-discard arm (it entered this turn, X = 0), 9. withheld frames:
//     a carrier, the mana and an unblocked attacker outside the window
//     (the declare-attackers step), where neither side may offer it.
var ninjaDecks = [2][]cardset.Entry{
	{
		{Name: "Ninja of the Deep Hours", Count: 4}, {Name: "Moon-Circuit Hacker", Count: 4},
		{Name: "Kaito, Bane of Nightmares", Count: 3}, {Name: "Wind Drake", Count: 6},
		{Name: "Walking Corpse", Count: 6}, {Name: "Island", Count: 9}, {Name: "Swamp", Count: 8},
	},
	{
		{Name: "Savannah Lions", Count: 6}, {Name: "Hill Giant", Count: 6}, {Name: "Shock", Count: 6},
		{Name: "Swords to Plowshares", Count: 4}, {Name: "Plains", Count: 9}, {Name: "Mountain", Count: 9},
	},
}

var ninjaCarrierNames = []string{"Ninja of the Deep Hours", "Moon-Circuit Hacker", "Kaito, Bane of Nightmares"}

// ninjaAbility reports whether ability idx of o's face is the K:Ninjutsu
// expansion.
func ninjaAbility(o *state.Object, idx int) bool {
	if o == nil || o.Face() == nil || idx < 0 || idx >= len(o.Face().Abilities) {
		return false
	}
	sa := o.Face().Abilities[idx]
	return sa != nil && sa.Params["Keyword"] == "Ninjutsu"
}

// ninjaCarrier reports whether o has a K:Ninjutsu ability.
func ninjaCarrier(o *state.Object) bool {
	if o == nil || o.Face() == nil {
		return false
	}
	for i := range o.Face().Abilities {
		if ninjaAbility(o, i) {
			return true
		}
	}
	return false
}

type ninjaTally struct {
	evAt      int
	pushed    map[state.ObjID]bool // ninjutsu activations whose card has not entered yet
	tapped    map[state.ObjID]bool
	entered   map[state.ObjID]bool // ninjas that entered attacking this combat
	enterTurn map[state.ObjID]bool // Hackers that entered the battlefield this turn
	hackerAct map[state.PlayerID]int
	carriers  map[string]int
	entries   int
	badEntry  int
	pwEntry   int
	connect   int
	atkTrig   int
	retMany   int
	retOne    int
	discard   int // 7
	noDiscard int // 8
	wOut      int // 9
	badOffer  int
	hackerOld map[state.PlayerID]bool // a Hacker trigger for p whose source did not enter this turn is on the stack
}

func (p *ninjaTally) reset() {
	p.evAt = 0
	p.pushed = map[state.ObjID]bool{}
	p.tapped = map[state.ObjID]bool{}
	p.entered = map[state.ObjID]bool{}
	p.enterTurn = map[state.ObjID]bool{}
	p.hackerOld = map[state.PlayerID]bool{}
}

func (p *ninjaTally) check(ref *GorgeSide, k Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.reset()
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.AbilityPush:
			if o := g.Obj(e.Obj); ninjaAbility(o, int(e.Amount)) {
				p.pushed[e.Obj] = true
				p.carriers[o.Face().Name]++
			}
		case events.Tap:
			if p.pushed[e.Obj] && e.Text == "entered tapped" {
				p.tapped[e.Obj] = true
			}
		case events.TokenAttacks:
			if !p.pushed[e.Obj] || e.Text != "entered attacking" {
				continue
			}
			if !p.tapped[e.Obj] || len(e.IDs) == 0 {
				p.badEntry++
				continue
			}
			if len(e.IDs) > 1 {
				p.pwEntry++
			}
			p.entries++
			p.entered[e.Obj] = true
			delete(p.pushed, e.Obj)
		case events.MoveZone:
			if e.To == state.ZBattlefield {
				if o := g.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Moon-Circuit Hacker" {
					p.enterTurn[e.Obj] = true
				}
			}
			if e.From == state.ZBattlefield {
				delete(p.entered, e.Obj)
			}
			if e.From == state.ZHand && e.To == state.ZGraveyard && g.Step == state.StepCombatDamage {
				// the pool's only combat-step discard is the Hacker's
				if o := g.Obj(e.Obj); o != nil && p.hackerOld[o.Owner] {
					p.discard++
					p.hackerOld[o.Owner] = false
				}
			}
		case events.TriggerPush:
			o := g.Obj(e.Obj)
			if o == nil || o.Face() == nil || int(e.Amount) >= len(o.Face().Triggers) {
				continue
			}
			mode := o.Face().Triggers[e.Amount].Mode
			if p.entered[e.Obj] && mode == "Attacks" {
				p.atkTrig++
			}
			if o.Face().Name == "Moon-Circuit Hacker" && mode == "DamageDone" {
				if p.enterTurn[e.Obj] {
					p.noDiscard++
				} else {
					p.hackerOld[o.Controller] = true
				}
			}
		case events.DamageProvenance:
			if p.entered[e.Obj] && g.Step == state.StepCombatDamage {
				p.connect++
			}
		case events.StepChange:
			if e.Step == state.StepEndCombat || e.Step == state.StepCleanup {
				p.entered = map[state.ObjID]bool{}
			}
		case events.TurnChange:
			p.enterTurn = map[state.ObjID]bool{}
		}
	}
	p.evAt = len(evs)
	fr := ref.fr
	if fr == nil || fr.d == nil {
		return nil
	}
	if fr.d.Kind == decision.KChoose {
		n := 0
		for _, o := range fr.d.Options {
			if o.Kind == "returncost" {
				n++
			}
		}
		switch {
		case n == 1:
			p.retOne++
		case n >= 2:
			p.retMany++
		}
		return nil
	}
	if fr.d.Kind != decision.KPriority {
		return nil
	}
	actor := state.PlayerID(fr.d.Player)
	if g.Step != state.StepDeclareAttackers || g.Active != actor || !ninjaHolds(g, actor) {
		return nil
	}
	_, unb := sneakAttackers(g, actor)
	if unb == 0 {
		return nil
	}
	p.wOut++
	for _, o := range fr.d.Options {
		if o.Kind == "activate" && ninjaAbility(g.Obj(o.Obj), o.Ability) {
			p.badOffer++
		}
	}
	if ninjaKernelOffers(k) != 0 {
		p.badOffer++
	}
	return nil
}

// ninjaHolds reports whether p holds a ninjutsu carrier and has two untapped
// lands (every carrier's ninjutsu but Kaito's costs at most two).
func ninjaHolds(g *state.Game, p state.PlayerID) bool {
	held := false
	for _, id := range g.Zone(state.ZHand, p) {
		held = held || ninjaCarrier(g.Obj(id))
	}
	if !held {
		return false
	}
	lands := 0
	for _, id := range g.Zone(state.ZBattlefield, p) {
		if o := g.Obj(id); o != nil && o.Controller == p && !o.Tapped && o.Face() != nil && o.Face().IsLand() {
			lands++
		}
	}
	return lands >= 2
}

// ninjaKernelOffers counts the kernel's posed ninjutsu activations.
func ninjaKernelOffers(k Side) int {
	r, ok := k.(*realSide)
	if !ok {
		return 0
	}
	n := 0
	for _, o := range r.opts {
		if o.Kind() != ActActivate || int(o.Src()) >= int(r.kp.NObj) {
			continue
		}
		c := r.kp.Objs[o.Src()].Card
		if int(c) >= len(r.tbl.Cards) {
			continue
		}
		for _, a := range r.tbl.Cards[c].Abilities {
			if a.AFlags&kop.AfNinjutsu != 0 && r.kp.Objs[o.Src()].Zone == kop.ZoneHand {
				n++
				break
			}
		}
	}
	return n
}

// ninjaPolicy attacks with every able creature (a random defender among the
// offered pairs), blocks per its class (seed/4%3: 0 never, 1 always, 2 at
// random), activates every ninjutsu it is offered (the carrier activated
// fewest times first), plays only a land in its first main phase so the
// combat window has mana, and otherwise plays at random.
func ninjaPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	class := seed / 4 % 3
	used := map[string]int{}
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		switch c.D.Kind {
		case DAttackers:
			buf = buf[:0]
			for _, a := range c.D.Options {
				if a.Kind() == ActAttack {
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 {
				d := *c.D
				d.Options = buf
				return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
			for _, a := range c.D.Options {
				if a.Kind() == ActDone {
					return a
				}
			}
		case DBlockers:
			want := ActDone
			if class == 1 {
				want = ActBlock
			}
			if class != 2 {
				for _, a := range c.D.Options {
					if a.Kind() == want {
						return a
					}
				}
			}
		case DPriority:
			var best Canon
			bestN := 1 << 30
			for _, a := range c.D.Options {
				if a.Kind() != ActActivate || c.G == nil || int(a.Src()) >= len(c.G.id.obj) {
					continue
				}
				o := c.G.E.G.Obj(c.G.id.obj[a.Src()])
				if o == nil || o.Zone != state.ZHand || !ninjaAbility(o, int(a.Mode())) {
					continue
				}
				if n := used[o.Face().Name]; n < bestN {
					best, bestN = a, n
				}
			}
			if bestN < 1<<30 {
				o := c.G.E.G.Obj(c.G.id.obj[best.Src()])
				used[o.Face().Name]++
				return best
			}
			if c.G != nil {
				g := c.G.E.G
				p := state.PlayerID(c.D.Player)
				if g.Active == p && (g.Step == state.StepMain1 || g.Step == state.StepBeginCombat || g.Step == state.StepDeclareAttackers) {
					for _, a := range c.D.Options {
						if a.Kind() == ActPlayLand {
							return a
						}
					}
					for _, a := range c.D.Options {
						if a.Kind() == ActPass {
							return a
						}
					}
				}
			}
			buf = buf[:0]
			for _, a := range c.D.Options {
				if k := a.Kind(); k == ActCast || k == ActActivate || k == ActPlayLand {
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 {
				d := *c.D
				d.Options = buf
				return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
		}
		return rnd.Pick(c)
	})
}

func ninjaRun(t *testing.T, reg *cards.Registry, set cardset.Set, decks [2][]*cards.Card, tbl *kop.Table, seed uint64, cfg Config, check func(*GorgeSide, Side) *Divergence) Result {
	t.Helper()
	m, err := NewMatchup(reg, [2]string{"ninja-a", "ninja-b"}, decks, 1)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(set, tbl, cfg)(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	pol := ninjaPolicy(seed)
	if seed%4 == 0 {
		pol = RandomPolicy(seed)
	}
	res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 8000, MaxTurns: 80, Check: check})
	CloseSide(k)
	return res
}

func ninjaSetup(t *testing.T) (*cards.Registry, cardset.Set, [][2][]*cards.Card, []*kop.Table) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("ninja")
	if err != nil {
		t.Fatal(err)
	}
	matchups := [][2][]cardset.Entry{{ninjaDecks[0], ninjaDecks[1]}, {ninjaDecks[1], ninjaDecks[0]}, {ninjaDecks[0], ninjaDecks[0]}}
	decks := make([][2][]*cards.Card, len(matchups))
	tbls := make([]*kop.Table, len(matchups))
	for mi, md := range matchups {
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[mi][s] = append(decks[mi][s], c)
				}
			}
			if len(decks[mi][s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[mi][s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		tbls[mi] = tbl
	}
	return reg, set, decks, tbls
}

const ninjaSeeds = 60

func TestNinjutsuLockstep(t *testing.T) {
	t.Parallel()
	reg, set, decks, tbls := ninjaSetup(t)
	var p ninjaTally
	p.reset()
	p.carriers = map[string]int{}
	games, embStalls := 0, 0
	for mi := range decks {
		for seed := uint64(1); seed <= ninjaSeeds; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			p.reset()
			res := ninjaRun(t, reg, set, decks[mi], tbls[mi], seed, cfg, p.check)
			games++
			switch {
			case res.Stall == "kernel emblem":
				// Kaito's +1 emblem past the kernel's maxEmb (4 per player,
				// gen.go): the documented capacity draw, not a divergence
				embStalls++
			case res.Stall != "":
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games: ninjutsu entries %d (bad %d, planeswalker defenders %d), connected %d (Attacks triggers %d); return picks: %d several, %d one; Hacker: discard %d, no discard (entered this turn) %d; withheld frames %d (offered %d); emblem capacity stalls %d",
		games, p.entries, p.badEntry, p.pwEntry, p.connect, p.atkTrig, p.retMany, p.retOne, p.discard, p.noDiscard, p.wOut, p.badOffer, embStalls)
	for _, name := range ninjaCarrierNames {
		t.Logf("carrier %q: %d activations", name, p.carriers[name])
		if p.carriers[name] < 5 {
			t.Errorf("carrier %q activated %d times, want at least 5", name, p.carriers[name])
		}
	}
	for _, c := range []struct {
		name   string
		n, min int
	}{
		{"ninjutsu entries", p.entries, 20}, {"connected ninjas", p.connect, 10},
		{"planeswalker defenders", p.pwEntry, 1},
		{"return picks with several candidates", p.retMany, 10}, {"return picks with one candidate", p.retOne, 10},
		{"Hacker discards", p.discard, 3}, {"Hacker no-discard arms", p.noDiscard, 3},
		{"withheld frames", p.wOut, 10},
	} {
		if c.n < c.min {
			t.Errorf("%d %s, want at least %d", c.n, c.name, c.min)
		}
	}
	for _, c := range []struct {
		name string
		n    int
	}{{"bad ninjutsu entries", p.badEntry}, {"Attacks triggers from ninjas", p.atkTrig}, {"ninjutsu offers in withheld frames", p.badOffer}} {
		if c.n != 0 {
			t.Errorf("%d %s, want 0", c.n, c.name)
		}
	}
}

// TestNinjutsuMutation: each mutation must diverge on some seed.
func TestNinjutsuMutation(t *testing.T) {
	if isolate(t) {
		return // sets dirac.Debug* globals
	}
	reg, set, decks, tbls := ninjaSetup(t)
	for _, mu := range []struct {
		name string
		on   *bool
	}{
		{"ninja not attacking", &dirac.DebugNinjutsuNoAttack},
		{"offered outside the window", &dirac.DebugNinjutsuAnyStep},
	} {
		*mu.on = true
		diverged := false
		for seed := uint64(1); seed <= ninjaSeeds && !diverged; seed++ {
			if seed%4 == 0 {
				continue
			}
			res := ninjaRun(t, reg, set, decks[0], tbls[0], seed, Config{}, nil)
			if diverged = res.Div != nil; diverged {
				t.Logf("mutation %q: seed %d diverged at decision %d: %s %s", mu.name, seed, res.Div.At, res.Div.What, res.Div.Field)
			}
		}
		*mu.on = false
		if !diverged {
			t.Errorf("mutation %q never diverged", mu.name)
		}
	}
}
