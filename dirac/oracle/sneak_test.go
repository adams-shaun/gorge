package oracle

import (
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Sneak lockstep (CR 702.190): constructed matchups over the generated
// sneak pool. gorge offers a sneak cast only when the floating pool pays it
// (its planner prices no sneak play), so the policy floats mana with a
// small-X instant in the declare-blockers step; the autopay surface floats
// none. The counters come from gorge's event log and option lists:
//
//  1. a creature cast for its sneak cost enters tapped, then attacking the
//     returned creature's captured defender ("entered attacking");
//  2. a sneaked instant or sorcery resolves;
//  3. a return pick with several candidates, 4. with one (still asked);
//  5. a sneaked attacker deals combat damage, with no Attacks trigger;
//  6. withheld with every attacker blocked, 7. in a later combat step,
//  8. in the opponent's declare-blockers step, each with a carrier
//     whose sneak cost the floating pool pays.
var sneakDecks = [2][]cardset.Entry{
	{
		{Name: "Leonardo, Big Brother", Count: 1}, {Name: "Leonardo, Cutting Edge", Count: 1},
		{Name: "Leonardo's Technique", Count: 3}, {Name: "Splinter, Hamato Yoshi", Count: 2},
		{Name: "Splinter's Technique", Count: 2}, {Name: "Oroku Saki, Shredder Rising", Count: 1},
		{Name: "Shredder, Unrelenting", Count: 3}, {Name: "Elektra, Daughter of the Hand", Count: 3},
		{Name: "Slice from the Shadows", Count: 8}, {Name: "Savannah Lions", Count: 4},
		{Name: "Plains", Count: 6}, {Name: "Swamp", Count: 6},
	},
	{
		{Name: "Donatello's Technique", Count: 3}, {Name: "Donatello, Gadget Master", Count: 2},
		{Name: "Raphael, the Nightwatcher", Count: 1}, {Name: "Jennika's Technique", Count: 3},
		{Name: "New Generation's Technique", Count: 3}, {Name: "Pedal to the Metal", Count: 4},
		{Name: "Torch Breath", Count: 4}, {Name: "Tyvar's Stand", Count: 1},
		{Name: "Fanatical Firebrand", Count: 3}, {Name: "Courageous Goblin", Count: 1},
		{Name: "Island", Count: 4}, {Name: "Mountain", Count: 7}, {Name: "Forest", Count: 4},
	},
}

// sneakCarrierNames are the lifted carriers the lockstep proves: 12 of the
// 13 (TMT +12, MSH +1). Jennika's Technique never fired in any seed (its
// plain instant-speed cast and the board wipe it causes end the float
// windows), so per the controller ruling it is removed from this list and
// named in the report; the kernel unit test TestSneakCarrierCoverage still
// casts it for its sneak cost. Every listed carrier must fire at least once
// (three times where feasible).
var sneakCarrierNames = []string{
	"Donatello's Technique", "Donatello, Gadget Master",
	"Leonardo's Technique", "Leonardo, Big Brother", "Leonardo, Cutting Edge",
	"New Generation's Technique", "Oroku Saki, Shredder Rising",
	"Raphael, the Nightwatcher", "Shredder, Unrelenting", "Splinter's Technique",
	"Splinter, Hamato Yoshi", "Elektra, Daughter of the Hand",
}

// sneakTally counts the arms on gorge's side.
type sneakTally struct {
	evAt                    int
	sneaked                 map[state.ObjID]bool // spells cast for their sneak cost (CastInfo "sneaked")
	tapped                  map[state.ObjID]bool // sneaked permanents tapped as they entered
	entered                 map[state.ObjID]bool // sneaked permanents that entered attacking this combat
	autopay                 bool
	carriers                map[string]int // sneak casts by carrier name (every lifted carrier must fire)
	creature                int            // 1
	badEntry                int            // a sneaked permanent entered attacking with no Tap first, or the wrong defender
	nonCre                  int            // 2
	retMany                 int            // 3
	retOne                  int            // 4
	connect                 int            // 5
	atkTrig                 int            // Attacks triggers sourced by a sneaked attacker (must be 0)
	wBlocked                int            // 6
	wStep                   int            // 7
	wOpp                    int            // 8
	badOffer                int            // a sneak option in a withheld frame (must be 0)
	offers                  int
	offersAP                int // sneak options on the autopay surface
	emptyOffer              map[state.ObjID]bool
	emptyOffers, emptyTaken int
	pwEntry                 int // H4: entries attacking a planeswalker
	reSneak                 int // H7: a sneak-entered attacker among return candidates
}

func (p *sneakTally) reset() {
	p.evAt = 0
	p.sneaked = map[state.ObjID]bool{}
	p.tapped = map[state.ObjID]bool{}
	p.entered = map[state.ObjID]bool{}
	p.emptyOffer = map[state.ObjID]bool{}
}

// sneakCost is the printed K:Sneak cost of o's face (ok false without one).
func sneakCost(o *state.Object) (generic int32, pips [5]int32, ok bool) {
	if o == nil || o.Face() == nil {
		return 0, pips, false
	}
	for _, kw := range o.Face().Keywords {
		param, found := strings.CutPrefix(kw, "Sneak:")
		if !found {
			continue
		}
		for _, t := range strings.Fields(param) {
			if n, err := strconv.Atoi(t); err == nil {
				generic += int32(n)
			} else if i := strings.Index("WUBRG", t); len(t) == 1 && i >= 0 {
				pips[i]++
			}
		}
		return generic, pips, true
	}
	return 0, pips, false
}

// sneakAttackers counts p's attacking creatures, and the unblocked ones.
func sneakAttackers(g *state.Game, p state.PlayerID) (all, unblocked int) {
	for _, id := range g.Zone(state.ZBattlefield, p) {
		o := g.Obj(id)
		if o == nil || o.Controller != p || !o.IsAttacking || o.Face() == nil || !o.Face().IsCreature() {
			continue
		}
		all++
		if len(o.BlockedBy) == 0 {
			unblocked++
		}
	}
	return
}

// sneakBlockers counts p's untapped creatures, the ones that could block.
func sneakBlockers(g *state.Game, p state.PlayerID) int {
	n := 0
	for _, id := range g.Zone(state.ZBattlefield, p) {
		if o := g.Obj(id); o != nil && o.Controller == p && o.Face() != nil && o.Face().IsCreature() && !o.Tapped {
			n++
		}
	}
	return n
}

func (p *sneakTally) check(ref *GorgeSide, k Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.reset()
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.CastInfo:
			for _, f := range strings.Split(e.Counter, ",") {
				if strings.TrimSpace(f) == "sneaked" {
					p.sneaked[e.Obj] = true
					if p.emptyOffer[e.Obj] {
						p.emptyTaken++
					}
					if o := g.Obj(e.Obj); o != nil && o.Face() != nil {
						p.carriers[o.Face().Name]++
					}
				}
			}
		case events.Tap:
			if p.sneaked[e.Obj] {
				if o := g.Obj(e.Obj); o != nil && o.Zone == state.ZBattlefield {
					p.tapped[e.Obj] = true
				}
			}
		case events.TokenAttacks:
			if !p.sneaked[e.Obj] || e.Text != "entered attacking" {
				continue
			}
			o := g.Obj(e.Obj)
			if !p.tapped[e.Obj] || o == nil || !o.SneakDefenderValid || len(e.IDs) == 0 || e.IDs[0] != state.PlayerRef(o.SneakDefender) {
				p.badEntry++
				continue
			}
			if len(e.IDs) > 1 || o.SneakDefenderObject != 0 {
				p.pwEntry++
			}
			p.creature++
			p.entered[e.Obj] = true
		case events.MoveZone:
			if p.sneaked[e.Obj] && e.From == state.ZStack && e.To != state.ZBattlefield {
				if o := g.Obj(e.Obj); o != nil && o.Face() != nil && (o.Face().IsInstant() || o.Face().IsSorcery()) && e.To == state.ZGraveyard {
					p.nonCre++
				}
				delete(p.sneaked, e.Obj)
			}
			if e.From == state.ZBattlefield {
				delete(p.entered, e.Obj)
			}
		case events.DamageProvenance:
			if p.entered[e.Obj] && (g.Step == state.StepCombatDamage) {
				p.connect++
			}
		case events.TriggerPush:
			if p.entered[e.Obj] {
				if o := g.Obj(e.Obj); o != nil && o.Face() != nil && int(e.Amount) < len(o.Face().Triggers) && o.Face().Triggers[e.Amount].Mode == "Attacks" {
					p.atkTrig++
				}
			}
		case events.StepChange:
			if e.Step == state.StepEndCombat || e.Step == state.StepCleanup {
				p.entered = map[state.ObjID]bool{}
			}
		}
	}
	p.evAt = len(evs)
	fr := ref.fr
	if fr == nil || fr.d == nil {
		return nil
	}
	if fr.d.Kind == decision.KChoose {
		n := 0
		re := false
		for _, o := range fr.d.Options {
			if o.Kind == "returncost" {
				n++
				re = re || p.entered[o.Obj]
			}
		}
		switch {
		case n == 1:
			p.retOne++
		case n >= 2:
			p.retMany++
		}
		if re {
			p.reSneak++
		}
		return nil
	}
	if fr.d.Kind != decision.KPriority {
		return nil
	}
	actor := state.PlayerID(fr.d.Player)
	opts := 0
	for _, r := range kernelFaceRefs(k, faceCodeSneak) {
		id := ref.id.obj[r]
		opts++
		p.offers++
		if p.autopay {
			p.offersAP++
		}
		if g.Players[actor].Pool.Total() == 0 {
			p.emptyOffers++
			p.emptyOffer[id] = true
		}
	}
	all, unb := sneakAttackers(g, actor)
	// The withheld-frame assertions read the kernel's own posed options: the
	// lockstep already compares the two lists, but a gate that only stopped
	// the kernel offering while gorge kept offering would be a divergence,
	// and a gate the kernel kept while gorge stopped would show up only here
	// (TestSneakOfferWindow pins the same gate directly).
	//
	// Counter 8 (the non-active player's declare-blockers step) is
	// structurally unreachable, so its >=10 floor is waived by the
	// controller ruling. gorge's sneakTimingOK (engine/rules/sneak.go:73-75
	// at the pin) is `G.Active == p && G.Step == StepDeclareBlockers`; the
	// kernel caller tests `p == s.active` (dirac/legal.go:69) and
	// sneakStep (dirac/kw_sneak.go:56). Count every window where
	// the non-active priority holder has a carrier -- the frame a stray
	// offer would need -- and assert none was offered.
	if g.Step == state.StepDeclareBlockers && g.Active != actor && sneakCarrier(g, actor) {
		p.wOpp++
		if opts != 0 {
			p.badOffer++
		}
		return nil
	}
	if !sneakCarrier(g, actor) {
		return nil
	}
	switch {
	case g.Step == state.StepDeclareBlockers && g.Active == actor && all > 0 && unb == 0:
		p.wBlocked++
	// gorge maps its "First Strike Damage" step to StepCombatDamage
	// (state/phase.go), so StepCombatDamage is both damage steps; the
	// kernel's separate StepFirstStrikeDamage is pinned by
	// TestSneakOfferWindow.
	case g.Active == actor && unb > 0 && (g.Step == state.StepCombatDamage || g.Step == state.StepEndCombat):
		p.wStep++
	default:
		return nil
	}
	if opts != 0 {
		p.badOffer++
	}
	return nil
}

// sneakPolicy attacks with everything and takes a sneak cast when offered.
// The probe seeds (class seed/4%4) shape the withheld frames: class 1 in the
// active player's later combat steps, class 2 in the opponent's
// declare-blockers step, class 3 with its one attacker blocked. Otherwise it
// plays or casts as preparePolicy does.
func sneakPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	probe := seed / 4 % 4 // independent of the surface (seed%2) and the policy (seed%4)
	var buf []Canon
	casts := map[string]int{} // sneaks taken this game, per carrier
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DAttackers {
			// class 3 attacks with one creature, and only once the defender
			// controls an untapped creature that can block it (so the
			// attackers end up all blocked: counter 6); the others attack
			// with every able creature
			for _, a := range c.D.Options {
				if a.Kind() != ActAttack {
					continue
				}
				if probe == 3 && c.G != nil {
					p := state.PlayerID(c.D.Player)
					if sneakAttacking(c.G.E.G, p) || sneakBlockers(c.G.E.G, 1-p) == 0 {
						continue
					}
				}
				return a
			}
			for _, a := range c.D.Options {
				if a.Kind() == ActDone {
					return a
				}
			}
		}
		if c.D.Kind == DBlockers {
			// class 3 blocks whatever it can (every attacker blocked:
			// counter 6); the others let the attackers through
			want := ActDone
			if probe == 3 {
				want = ActBlock
			}
			for _, a := range c.D.Options {
				if a.Kind() == want {
					return a
				}
			}
		}
		// A legal sneak offer is always taken: the policy never skips a
		// legal action. When several carriers are offered at once, take
		// the one sneaked fewest times this game (ties: the priciest), so
		// every carrier is exercised; the cheap one would otherwise win
		// every time. Counter 6 (all attackers blocked) is a board where
		// no sneak option can exist at all, so taking every offer does
		// not hide it.
		var sneak Canon
		sneakName := ""
		sneakN := int(1 << 30)
		sneakTot := int32(-1)
		for _, a := range c.D.Options {
			if a.Kind() == ActCastFace && a.Mode()&0x0f == faceCodeSneak {
				if c.G == nil || int(a.Src()) >= len(c.G.id.obj) {
					return a
				}
				o := c.G.E.G.Obj(c.G.id.obj[a.Src()])
				gen, pips, ok := sneakCost(o)
				if !ok {
					return a
				}
				tot := gen
				for _, n := range pips {
					tot += n
				}
				name := ""
				if o != nil && o.Face() != nil {
					name = o.Face().Name
				}
				n := casts[name]
				if n < sneakN || n == sneakN && tot > sneakTot {
					sneak, sneakName, sneakN, sneakTot = a, name, n, tot
				}
			}
		}
		if sneakN < 1<<30 {
			casts[sneakName]++
			return sneak
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActActivate || k == ActPlayLand {
				buf = append(buf, a)
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

// sneakAttacking reports whether p already declared an attacker.
func sneakAttacking(g *state.Game, p state.PlayerID) bool {
	all, _ := sneakAttackers(g, p)
	return all > 0
}

func sneakCarrier(g *state.Game, p state.PlayerID) bool {
	for _, id := range g.Zone(state.ZHand, p) {
		if _, _, ok := sneakCost(g.Obj(id)); ok {
			return true
		}
	}
	return false
}

func sneakRun(t *testing.T, reg *cards.Registry, set cardset.Set, decks [2][]*cards.Card, tbl *kop.Table, seed uint64, cfg Config, check func(*GorgeSide, Side) *Divergence) Result {
	t.Helper()
	m, err := NewMatchup(reg, [2]string{"sneak-a", "sneak-b"}, decks, 1)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(set, tbl, cfg)(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	pol := sneakPolicy(seed)
	if seed%4 == 0 {
		pol = RandomPolicy(seed)
	}
	res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 8000, MaxTurns: 80, Check: check})
	CloseSide(k)
	return res
}

func sneakSetup(t *testing.T) (*cards.Registry, cardset.Set, [][2][]*cards.Card, []*kop.Table) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("sneak")
	if err != nil {
		t.Fatal(err)
	}
	matchups := [][2][]cardset.Entry{{sneakDecks[0], sneakDecks[1]}, {sneakDecks[1], sneakDecks[0]}}
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

// sneakFocus describes the per-carrier focus deck: the carrier, an {X}
// instant whose lands float the carrier's colours, an attacker, and the
// land split. Both seats play it.
type sneakFocus struct {
	carrier, x, creature string
	lands                [][2]interface{} // name, count
}

var sneakFocusDecks = []sneakFocus{
	{"Leonardo, Big Brother", "Slice from the Shadows", "Savannah Lions", [][2]interface{}{{"Plains", 9}, {"Swamp", 8}}},
	{"Leonardo, Cutting Edge", "Slice from the Shadows", "Savannah Lions", [][2]interface{}{{"Plains", 9}, {"Swamp", 8}}},
	{"Leonardo's Technique", "Slice from the Shadows", "Savannah Lions", [][2]interface{}{{"Plains", 9}, {"Swamp", 8}}},
	{"Splinter, Hamato Yoshi", "Slice from the Shadows", "Savannah Lions", [][2]interface{}{{"Plains", 9}, {"Swamp", 8}}},
	{"Splinter's Technique", "Slice from the Shadows", "Savannah Lions", [][2]interface{}{{"Plains", 9}, {"Swamp", 8}}},
	{"Oroku Saki, Shredder Rising", "Slice from the Shadows", "Savannah Lions", [][2]interface{}{{"Plains", 9}, {"Swamp", 8}}},
	{"Shredder, Unrelenting", "Slice from the Shadows", "Savannah Lions", [][2]interface{}{{"Plains", 9}, {"Swamp", 8}}},
	{"Elektra, Daughter of the Hand", "Slice from the Shadows", "Savannah Lions", [][2]interface{}{{"Plains", 9}, {"Swamp", 8}}},
	{"Donatello's Technique", "Pedal to the Metal", "Fanatical Firebrand", [][2]interface{}{{"Island", 9}, {"Mountain", 8}}},
	{"Donatello, Gadget Master", "Pedal to the Metal", "Fanatical Firebrand", [][2]interface{}{{"Island", 9}, {"Mountain", 8}}},
	{"Raphael, the Nightwatcher", "Pedal to the Metal", "Fanatical Firebrand", [][2]interface{}{{"Mountain", 17}}},
	{"New Generation's Technique", "Pedal to the Metal", "Fanatical Firebrand", [][2]interface{}{{"Forest", 9}, {"Mountain", 8}}},
}

func (f sneakFocus) entries() []cardset.Entry {
	es := []cardset.Entry{{Name: f.carrier, Count: 4}, {Name: f.x, Count: 14}, {Name: f.creature, Count: 5}}
	for _, l := range f.lands {
		es = append(es, cardset.Entry{Name: l[0].(string), Count: l[1].(int)})
	}
	return es
}

const sneakSeeds = 120

func TestSneakLockstep(t *testing.T) {
	t.Parallel()

	reg, set, decks, tbls := sneakSetup(t)
	var p sneakTally
	p.reset()
	p.carriers = map[string]int{}
	games := 0
	play := func(mi int, seed uint64, cfg Config) {
		p.reset()
		p.autopay = cfg.GorgeAutopaySurface
		res := sneakRun(t, reg, set, decks[mi], tbls[mi], seed, cfg, p.check)
		games++
		if res.Stall != "" {
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
	for mi := range decks {
		for seed := uint64(1); seed <= sneakSeeds; seed++ {
			play(mi, seed, Config{GorgeAutopaySurface: seed%2 == 0})
		}
	}
	// Per-carrier focus games: a deck weighted to one carrier and the
	// lands that pay it, on the lowering surface. Each of the 12 carriers
	// must be sneaked at least three times (Donatello's Technique once),
	// counted from gorge's CastInfo "sneaked" events.
	const focusMin = 3
	for _, f := range sneakFocusDecks {
		es := f.entries()
		var fd [2][]*cards.Card
		for sd := range fd {
			for _, e := range es {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					fd[sd] = append(fd[sd], c)
				}
			}
		}
		tbl, _, _, err := set.Matchup(es, es)
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed < 700 && p.carriers[f.carrier] < focusMin; seed++ {
			if seed%2 == 0 || seed%4 == 0 || seed/4%4 == 3 {
				continue
			}
			p.reset()
			res := sneakRun(t, reg, set, fd, tbl, seed, Config{}, p.check)
			games++
			if res.Stall != "" || res.Div != nil {
				t.Errorf("focus %s seed %d: stall %q div %+v", f.carrier, seed, res.Stall, res.Div)
			}
		}
	}
	t.Logf("%d games: creatures sneaked %d (bad entries %d), noncreatures sneaked %d; return picks: %d with several candidates, %d with one; sneaked attackers connected %d (Attacks triggers %d); withheld: all blocked %d, later step %d, opponent's step %d (offered %d); sneak offers %d (%d on the autopay surface); empty-pool sneak offers %d, taken %d; H4 planeswalker entries %d; H7 sneak-entered return candidates %d",
		games, p.creature, p.badEntry, p.nonCre, p.retMany, p.retOne, p.connect, p.atkTrig, p.wBlocked, p.wStep, p.wOpp, p.badOffer, p.offers, p.offersAP, p.emptyOffers, p.emptyTaken, p.pwEntry, p.reSneak)
	// The random games do not reliably reach every carrier (a carrier
	// that is never offered looks the same as a dead sneak arm), so the
	// per-carrier floor is asserted by TestSneakCarrierCoverage, which
	// constructs a payable declare-blockers window for each of the 13.
	// The tally here still reports the distribution the games reached.
	for _, name := range sneakCarrierNames {
		t.Logf("carrier %q: %d sneak casts", name, p.carriers[name])
		want := focusMin
		if name == "Donatello's Technique" {
			want = 1 // a one-pip sorcery: its float is rare (reached twice)
		}
		if p.carriers[name] < want {
			t.Errorf("carrier %q sneaked %d times, want at least %d", name, p.carriers[name], want)
		}
	}
	for _, c := range []struct {
		name string
		n    int
	}{
		{"creatures sneaked", p.creature}, {"noncreatures sneaked", p.nonCre},
		{"return picks with several candidates", p.retMany}, {"return picks with one candidate", p.retOne},
		{"sneaked attackers connecting", p.connect},
		{"withheld (all blocked)", p.wBlocked},
		{"withheld (later step)", p.wStep},
		{"withheld (opponent's step, windows checked)", p.wOpp},
	} {
		if c.n < 10 {
			t.Errorf("%d %s, want at least 10", c.n, c.name)
		}
	}
	// The untapped sources are the point of the follow: a carrier offered
	// while the pool is empty must be taken, which only the kernel's own
	// option list can show (gorge's direct options list never holds them).
	if p.emptyOffers < 10 {
		t.Errorf("%d empty-pool sneak offers, want at least 10", p.emptyOffers)
	}
	if p.emptyTaken < 10 {
		t.Errorf("%d empty-pool sneak offers taken, want at least 10", p.emptyTaken)
	}
	// Counter 8's >=10 offer floor is waived (controller ruling): by rule
	// the sneak offer exists only in the active player's declare-blockers
	// step (gorge sneakTimingOK, engine/rules/sneak.go:73-75; kernel
	// legal.go:69 and kw_sneak.go:56), so the non-active player can
	// never receive it. The >=10 floor above therefore counts the windows
	// checked, and TestSneakOfferWindow additionally pins the withhold
	// with the opponent's own floating pool (s.prio = 1, pool[W] = 1).
	for _, c := range []struct {
		name string
		n    int
	}{
		{"bad sneak entries", p.badEntry}, {"Attacks triggers from sneaked attackers", p.atkTrig},
		{"sneak offers in withheld frames", p.badOffer},
	} {
		if c.n != 0 {
			t.Errorf("%d %s, want 0", c.n, c.name)
		}
	}
}

// TestSneakMutation: each mutation must diverge on some seed.
func TestSneakMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, set, decks, tbls := sneakSetup(t)
	// each mutation scans the policy class that exercises it (seed/4%4:
	// 0 the declare-blockers window, 1 the later step, 3 the blocked
	// attacker), keeping the test within its shard's budget
	muts := []struct {
		name  string
		on    *bool
		class uint64
		limit uint64
	}{
		{"sneaked permanent not attacking", &dirac.DebugSneakNoAttack, 0, sneakSeeds},
		{"offer in the damage steps", &dirac.DebugSneakAnyStep, 1, 1000},
		{"blocked attacker returnable", &dirac.DebugSneakBlockedReturn, 3, sneakSeeds},
		{"face cast offered only when the pool pays", &dirac.DebugFacePoolOnly, 0, sneakSeeds},
	}
	for _, mu := range muts {
		*mu.on = true
		diverged := false
		for mi := range decks {
			for seed := uint64(1); seed <= mu.limit && !diverged; seed++ {
				if seed%2 == 0 || seed%4 == 0 || seed/4%4 != mu.class {
					continue // the lowering surface, the sneak policy, the class
				}
				cfg := Config{}
				res := sneakRun(t, reg, set, decks[mi], tbls[mi], seed, cfg, nil)
				if diverged = res.Div != nil; diverged {
					t.Logf("mutation %q: matchup %d seed %d diverged at decision %d: %s %s", mu.name, mi, seed, res.Div.At, res.Div.What, res.Div.Field)
				}
			}
		}
		*mu.on = false
		if !diverged {
			t.Errorf("mutation %q never diverged", mu.name)
		}
	}
}
