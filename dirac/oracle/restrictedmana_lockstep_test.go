package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Spend-restricted mana from a source whose cost is more than {T} (CR 106.6,
// 605.1a, 601.2g-h): a mana fee, a sacrifice of the source or another permanent
// and a life payment. gorge's window probe prices these as units for exactly the
// payments their restriction admits, a life payment as a last-resort unit spent
// after every other (pay.PaymentPlanShapeTierOf). The pods play the
// restrictedmana fixture; each deck carries the supported carriers beside the
// spells and abilities their restrictions admit and refuse.

// restrictedManaClass is how a carrier's cost is classed.
type restrictedManaClass int

const (
	rmFee  restrictedManaClass = iota // {N}, {T}
	rmSac                             // {T}, sacrifice
	rmLife                            // {T}, pay life
	rmClasses
)

var restrictedManaClassOf = map[string]restrictedManaClass{
	"Castle Garenbrig": rmFee, "Cormela, Glamour Thief": rmFee, "Great Hall of the Citadel": rmFee,
	"Hydro-Channeler": rmFee, "Interplanar Beacon": rmFee, "Orb of Dragonkind": rmFee,
	"Jeweled Lotus": rmSac, "Master of Dark Rites": rmSac,
	"Gnarlroot Trapper": rmLife, "Untaidake, the Cloud Keeper": rmLife,
	"Thieving Varmint": rmLife,
}

// restrictedManaDecks are the pods' decks: a life-payment pair, a mana-fee deck, a sacrifice deck.
func restrictedManaDecks() [4][]cardset.Entry {
	return [4][]cardset.Entry{
		{
			{Name: "Gnarlroot Trapper", Count: 5}, {Name: "Elvish Archdruid", Count: 4}, {Name: "Elvish Champion", Count: 4},
			{Name: "Llanowar Elves", Count: 3}, {Name: "Craw Wurm", Count: 4}, {Name: "Giant Growth", Count: 3},
			{Name: "Grizzly Bears", Count: 4}, {Name: "Forest", Count: 10}, {Name: "Swamp", Count: 3},
		},
		{
			{Name: "Untaidake, the Cloud Keeper", Count: 5}, {Name: "Akroma, Angel of Wrath", Count: 3}, {Name: "Kamahl, Pit Fighter", Count: 3},
			{Name: "Isamaru, Hound of Konda", Count: 4}, {Name: "Lightning Bolt", Count: 3}, {Name: "Divination", Count: 3},
			{Name: "Shock", Count: 2}, {Name: "Hill Giant", Count: 4}, {Name: "Mountain", Count: 5}, {Name: "Plains", Count: 8},
		},
		{
			{Name: "Grizzly Bears", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
			// the Varmint's "spells you don't own" is never satisfiable in the
			// pod (no seat casts a card another seat owns): it is carried for
			// the engines' agreement on the lowered unit, counted by name if
			// it ever does pay.
			{Name: "Hill Giant", Count: 5}, {Name: "Prodigal Sorcerer", Count: 3}, {Name: "Thieving Varmint", Count: 2}, {Name: "Jeweled Lotus", Count: 2}, {Name: "Mountain", Count: 9}, {Name: "Swamp", Count: 7},
		},
		{
			// the sacrifice deck: the Master's cast-sacrifice-recast loop (its
			// own restriction admits the Vampire Nighthawks beside it), kept
			// dense so the seeds reach it; Isamaru is the legendary spell the
			// Untaidake's legendary-only {C}{C} relaxes onto
			{Name: "Untaidake, the Cloud Keeper", Count: 3}, {Name: "Isamaru, Hound of Konda", Count: 4}, {Name: "Walking Ballista", Count: 3},
			{Name: "Pacifism", Count: 3}, {Name: "Master of Dark Rites", Count: 4}, {Name: "Vampire Nighthawk", Count: 4},
			{Name: "Savannah Lions", Count: 3}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 4}, {Name: "Plains", Count: 6},
		},
	}
}

// restrictedManaCounts counts the restricted mana a carrier made that a spell
// or an ability the same decision pushed spent: byClass per cost class,
// byName per carrier (a class's count is not one carrier's).
type restrictedManaCounts struct {
	byClass [rmClasses]int
	byName  map[string]int
}

// restrictedManaScan walks gorge's log: a restricted positive ManaAdd names its
// source (the " @id" suffix) and the next push of the decision spends it.
func restrictedManaScan(g *GorgeSide, n *restrictedManaCounts) {
	if g == nil {
		return
	}
	if n.byName == nil {
		n.byName = map[string]int{}
	}
	var pending []state.ObjID
	credit := func(src state.ObjID) {
		if ob := g.E.G.Obj(src); ob != nil && ob.Face() != nil {
			name := ob.Face().Name
			if c, ok := restrictedManaClassOf[name]; ok {
				n.byClass[c]++
				n.byName[name]++
			}
		}
	}
	for i := range g.E.L.Events {
		ev := &g.E.L.Events[i]
		switch ev.Kind {
		case events.ManaAdd:
			if ev.Amount <= 0 {
				continue
			}
			if _, src, _, _, ok := events.ManaRestrictionFromText(restrSourceCutPersistent(ev.Text)); ok && src != 0 {
				pending = append(pending, src)
			}
		case events.ManaClear, events.TurnChange:
			pending = pending[:0]
		case events.PutOnStack, events.AbilityPush:
			for _, src := range pending {
				credit(src)
			}
			pending = pending[:0]
		}
	}
}

type restrictedManaPod struct {
	reg   *cards.Registry
	set   cardset.Set
	decks [][]cardset.Entry
}

// newRestrictedManaPod builds a pod of the first n decks.
func newRestrictedManaPod(t *testing.T, n int) restrictedManaPod {
	t.Helper()
	all := restrictedManaDecks()
	// The corpus skip comes before the fixture table read: a fresh clone has no
	// .cards (and so no tables), and must SKIP rather than fail on RawByName.
	reg := synthReg(t)
	set, err := cardset.RawByName("restrictedmana")
	if err != nil {
		t.Fatal(err)
	}
	p := restrictedManaPod{reg: reg, set: set}
	for i := 0; i < n; i++ {
		c := 0
		for _, e := range all[i] {
			c += e.Count
		}
		if c != 40 {
			t.Fatalf("deck %d: %d cards", i, c)
		}
		p.decks = append(p.decks, all[i])
	}
	return p
}

// play plays one seed of the pod (four seats with MoreDecks, two without).
func (p restrictedManaPod) play(t *testing.T, seed uint64, maxTurns int32) (Result, restrictedManaCounts) {
	return p.playWith(t, seed, maxTurns, nil)
}

func (p restrictedManaPod) playWith(t *testing.T, seed uint64, maxTurns int32, fin func(*GorgeSide)) (Result, restrictedManaCounts) {
	t.Helper()
	expand := func(d []cardset.Entry) []string {
		var out []string
		for _, e := range d {
			for j := 0; j < e.Count; j++ {
				out = append(out, e.Name)
			}
		}
		return out
	}
	r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{expand(p.decks[0]), expand(p.decks[1])},
		Seed: seed, PassElision: true, Surface: "full"}
	if len(p.decks) == 4 {
		r.MoreDecks = []string{"x2", "x3"}
		r.MoreCards = [][]string{expand(p.decks[2]), expand(p.decks[3])}
	}
	m, err := r.Matchup(p.reg)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions, rc.MaxTurns = 8000, maxTurns
	c := restrictedManaCounts{byName: map[string]int{}}
	rc.Finish = func(g *GorgeSide) {
		restrictedManaScan(g, &c)
		if fin != nil {
			fin(g)
		}
	}
	tbl, _, err := p.set.MatchupN(p.decks)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(p.set, tbl, rc.Config)(m, seed)
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	defer CloseSide(k)
	lead, err := NewGorgeSide(m, seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	return LockstepLed(m, seed, lead, k, castFirstPolicy(seed), rc), c
}

func restrictedManaFail(t *testing.T, seed uint64, res Result) {
	t.Helper()
	lo := max(res.Div.At-12, 0)
	t.Fatalf("seed %d: divergence at %d: %s %s %v stall=%q over=%v\nref %q\nker %q\n%s last %s\nlast actions %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Stall, res.Over,
		res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail, res.Div.LastSourced, actionStrings(res.Actions[lo:res.Div.At]))
}

var restrictedManaSeeds = []uint64{1, 2, 3, 4, 5, 6}

func restrictedManaLockstep(t *testing.T, seats int) {
	p := newRestrictedManaPod(t, seats)
	sum := restrictedManaCounts{byName: map[string]int{}}
	for _, seed := range restrictedManaSeeds {
		res, c := p.play(t, seed, 40)
		if res.Div != nil {
			restrictedManaFail(t, seed, res)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		for i := range sum.byClass {
			sum.byClass[i] += c.byClass[i]
		}
		for name, n := range c.byName {
			sum.byName[name] += n
		}
		t.Logf("seed %d: %d decisions, over %v %s; spent fee/sac/life %v by %v", seed, res.Decisions, res.Over, res.Stall, c.byClass, c.byName)
	}
	if seats == 4 && (sum.byClass[rmLife] == 0 || sum.byClass[rmSac] == 0) {
		t.Fatalf("restricted mana spent (fee, sacrifice, life) = %v by %v: want a life one and a sacrifice one", sum.byClass, sum.byName)
	}
	if seats == 2 && sum.byClass[rmLife] == 0 {
		t.Fatalf("restricted mana spent (fee, sacrifice, life) = %v by %v: want a fee or a life one", sum.byClass, sum.byName)
	}
}

// TestRestrictedManaSourcesFourSeatLockstep plays four-seat pods of the
// restrictedmana fixture: no divergence and no stall, and a restricted unit
// with a mana fee, a sacrifice and a life payment was spent.
func TestRestrictedManaSourcesFourSeatLockstep(t *testing.T) { restrictedManaLockstep(t, 4) }

// TestRestrictedManaSourcesTwoSeatLockstep is the same over the first two decks.
func TestRestrictedManaSourcesTwoSeatLockstep(t *testing.T) { restrictedManaLockstep(t, 2) }

// TestRestrictedManaLastResortMutation: spending a PayLife restricted unit
// before the other units (DebugRestrictedLifeFirst) changes which sources a
// payment taps, so it must diverge from gorge's last-resort tier.
func TestRestrictedManaLastResortMutation(t *testing.T) {
	p := newRestrictedManaPod(t, 4)
	dirac.DebugRestrictedLifeFirst = true
	defer func() { dirac.DebugRestrictedLifeFirst = false }()
	for _, seed := range restrictedManaSeeds {
		if res, _ := p.play(t, seed, 40); res.Div != nil {
			t.Logf("mutation diverged at %d: %s %s", res.Div.At, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Fatal("spending a PayLife restricted unit first diverged on no seed")
}
